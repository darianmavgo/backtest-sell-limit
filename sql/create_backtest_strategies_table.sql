CREATE TABLE IF NOT EXISTS backtest_strategies (
    strategy_name TEXT PRIMARY KEY,
    start_date TEXT,
    end_date TEXT,
    initial_value REAL,
    final_value REAL,
    total_return REAL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
); 