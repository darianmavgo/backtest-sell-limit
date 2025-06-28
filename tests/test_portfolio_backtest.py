import os
import pytest
import sqlite3
from portfolio_backtest import clear_backtest_history, db
from db_manager import SQLiteConnectionManager

@pytest.fixture
def test_db():
    """Create a temporary test database with some sample data"""
    # Use an in-memory database for testing
    test_db = SQLiteConnectionManager(":memory:")
    
    # Create the necessary tables
    test_db.execute("""
        CREATE TABLE IF NOT EXISTS backtest_strategies (
            strategy_name TEXT PRIMARY KEY,
            start_date TEXT,
            end_date TEXT,
            initial_value REAL,
            final_value REAL,
            total_return REAL
        )
    """)
    
    test_db.execute("""
        CREATE TABLE IF NOT EXISTS backtest_daily_values (
            strategy_name TEXT,
            date TEXT,
            value REAL,
            PRIMARY KEY (strategy_name, date)
        )
    """)
    
    test_db.execute("""
        CREATE TABLE IF NOT EXISTS logs (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp TEXT,
            level TEXT,
            message TEXT
        )
    """)
    
    # Insert some sample data
    test_db.execute(
        "INSERT INTO backtest_strategies VALUES (?, ?, ?, ?, ?, ?)",
        ("test_strategy", "2024-01-01", "2024-12-31", 1000000.0, 1100000.0, 10.0)
    )
    
    test_db.execute(
        "INSERT INTO backtest_daily_values VALUES (?, ?, ?)",
        ("test_strategy", "2024-01-01", 1000000.0)
    )
    
    test_db.execute(
        "INSERT INTO logs VALUES (?, ?, ?, ?)",
        (1, "2024-01-01 00:00:00", "INFO", "Test log message")
    )
    
    test_db.commit()
    return test_db

def test_clear_backtest_history(test_db, monkeypatch):
    """Test that clear_backtest_history properly clears all relevant tables"""
    # Mock the load_sql_query method to return our test SQL
    def mock_load_sql_query(self, filename):
        if filename == "clear_backtest_tables.sql":
            return """
                DELETE FROM backtest_strategies;
                DELETE FROM backtest_daily_values;
                DELETE FROM logs;
            """
        return ""
    
    # Patch both the global db connection and the load_sql_query method
    monkeypatch.setattr("portfolio_backtest.db", test_db)
    monkeypatch.setattr(SQLiteConnectionManager, "load_sql_query", mock_load_sql_query)
    
    # Verify we have data before clearing
    assert test_db.execute("SELECT COUNT(*) FROM backtest_strategies").fetchone()[0] > 0
    assert test_db.execute("SELECT COUNT(*) FROM backtest_daily_values").fetchone()[0] > 0
    assert test_db.execute("SELECT COUNT(*) FROM logs").fetchone()[0] > 0
    
    # Clear the history
    clear_backtest_history("test_strategy")
    
    # Verify all tables are empty
    assert test_db.execute("SELECT COUNT(*) FROM backtest_strategies").fetchone()[0] == 0
    assert test_db.execute("SELECT COUNT(*) FROM backtest_daily_values").fetchone()[0] == 0
    assert test_db.execute("SELECT COUNT(*) FROM logs").fetchone()[0] == 0 