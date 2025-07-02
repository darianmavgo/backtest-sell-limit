import sqlite3
import backtrader as bt
import pandas as pd
from datetime import datetime, timedelta, date
import warnings
import logging
import sys
import os
import atexit
from database_log_handler import DatabaseLogHandler
from db_manager import SQLiteConnectionManager
from strategies import BuySP500Up20

# Global settings
DB_FILE = os.path.join(os.path.dirname(os.path.abspath(__file__)), "backtest_sell_limits.db")
TICKER_TABLE = "ticker_list"
START_DATE = "2024-06-01"
END_DATE = "2025-06-26"
INITIAL_CASH = 1_000_000.0 # A large amount to ensure any trade can be made
# self.__class__.__name__ gives you the class name as a string.


# Global database connection
db = SQLiteConnectionManager(DB_FILE)

def get_sp500_tickers():
    """Fetches the list of S&P 500 tickers from the SQLite database."""
    try:
        query = db.load_sql_query("get_sp500_tickers.sql").format(ticker_table=TICKER_TABLE)
        df = pd.read_sql_query(query, db.get_connection())
        return df['symbol'].tolist()
    except sqlite3.OperationalError as e:
        if "database is locked" in str(e).lower() or "unable to open database file" in str(e).lower():
            if db.kill_locks():
                # Retry after clearing locks
                try:
                    df = pd.read_sql_query(query, db.get_connection())
                    return df['symbol'].tolist()
                except Exception as retry_error:
                    print(f"Error fetching tickers after lock cleanup: {retry_error}")
                    return []
        print(f"Database error fetching tickers: {e}")
        return []
    except Exception as e:
        print(f"Error fetching tickers: {e}")
        return []

def get_historical_data(symbol, start_date, end_date):
    """Get historical data for a symbol from SQLite database"""
    try:
        query = db.load_sql_query("get_historical_data.sql")
        df = pd.read_sql_query(
            query, 
            db.get_connection(), 
            params=[symbol, start_date, end_date],
            parse_dates={'date': '%Y-%m-%d %H:%M:%S'}
        )
        df.set_index('date', inplace=True)
        return df
    except Exception as e:
        logging.error(f"Error fetching historical data for {symbol}: {e}")
        return None

def process_trade_queue(trade_queue):
    """Process queued trades and insert them into strategy_history table"""
    try:
        # Ensure the table exists
        db.execute_sql_file("create_strategy_history_table.sql")
        
        # Process each trade
        for trade_data in trade_queue:
            db.execute_sql_file("insert_strategy_history.sql", (
                trade_data['strategy_name'],
                trade_data['symbol'],
                trade_data['trade_type'],
                trade_data['trade_status'],
                trade_data['quantity'],
                trade_data['price'],
                trade_data['value'],
                trade_data['pnl'],
                trade_data['pnl_percent'],
                trade_data['commission'],
                trade_data['trade_date']
            ))
        
        db.commit()
        logging.info(f"Successfully processed {len(trade_queue)} trades to strategy_history")
        
    except Exception as e:
        logging.error(f"Error processing trade queue: {e}")
        db.rollback()

def save_backtest_results(strategy_name, daily_values, initial_value, final_value, total_return):
    """Save backtest results to SQLite database"""
    try:
        logging.info(f"Saving backtest results for strategy: {strategy_name}")
        logging.info(f"Initial value: ${initial_value:,.2f}")
        logging.info(f"Final value: ${final_value:,.2f}")
        logging.info(f"Total return: {total_return:.2f}%")
        logging.info(f"Number of daily values: {len(daily_values)}")
        
        # Create tables if they don't exist
        db.execute_sql_file("create_backtest_strategies_table.sql")
        db.execute_sql_file("create_backtest_daily_values_table.sql")
        
        # NOTE: backtest_order_history table is no longer created or used.
        # All order history is now in the 'logs' table.

        # Save strategy performance
        db.execute_sql_file("insert_backtest_strategy.sql", 
                          (strategy_name, START_DATE, END_DATE, initial_value, final_value, total_return))
        logging.info("Saved strategy performance")

        # Save daily values
        for date, value in daily_values:
            db.execute_sql_file("insert_backtest_daily_value.sql", 
                              (strategy_name, date.strftime('%Y-%m-%d'), value))
        logging.info("Saved daily values")

        # NOTE: Order history is no longer saved here. It's logged directly.

        db.commit()
        logging.info("Database changes committed")
    except Exception as e:
        logging.error(f"Error saving backtest results: {e}")
        db.rollback()

def clear_backtest_history(strategy_name):
    """Clears all previous backtest data for a given strategy and the logs."""
    try:
        logging.info(f"Clearing backtest history for strategy: {strategy_name}")
        # Execute each DROP statement from the SQL file
        drop_statements = db.load_sql_query("clear_backtest_tables.sql").split(';')
        for statement in drop_statements:
            if statement.strip():
                db.execute(statement)
        db.commit()
        logging.info("Successfully cleared backtest history and logs.")
    except Exception as e:
        # Use print because logger might not be configured yet or is being cleared
        print(f"Error clearing backtest history: {e}")
        db.rollback()

def get_portfolio_data():
    """Get current portfolio data from SQLite database"""
    try:
        query = db.load_sql_query("get_portfolio_data.sql")
        df = pd.read_sql_query(query, db.get_connection())
        return df
    except Exception as e:
        print(f"Error fetching portfolio data: {e}")
        return pd.DataFrame()

def calculate_portfolio_value():
    """Calculate current portfolio value"""
    df = get_portfolio_data()
    if df.empty:
        return 0
    
    total_value = (df['shares'] * df['current_price']).sum()
    return total_value

def setup_logging():
    """Configure logging to use console output only (disable database logging for now)."""
    logger = logging.getLogger()
    logger.setLevel(logging.INFO)
    # Clear any existing handlers
    logger.handlers = []
    
    # Temporarily disable database handler due to concurrent access issues
    # db_handler = DatabaseLogHandler(db)
    # logger.addHandler(db_handler)
    
    # Add console handler
    console_handler = logging.StreamHandler(sys.stdout)
    console_handler.setLevel(logging.INFO)
    console_formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
    console_handler.setFormatter(console_formatter)
    logger.addHandler(console_handler)
    
    return logger

def run_backtest():
    strategy_name ="BuySP500Up20"
    # Clear old data before setting up logging
    clear_backtest_history(strategy_name)

    # Setup logging to the database
    logger = setup_logging()

    logger.info(f"Date range: {START_DATE} to {END_DATE}")

    # Set up Backtrader
    cerebro = bt.Cerebro()
    
    # Add our strategy
    cerebro.addstrategy(BuySP500Up20)
    
    # Get portfolio data
    portfolio_df = get_portfolio_data()
    
    # Add data feeds for each stock in portfolio
    for symbol in get_sp500_tickers():  # Changed to use sp500_tickers instead of portfolio
        try:
            # Get historical data from SQLite
            query = db.load_sql_query("get_backtest_historical_data.sql")
            df = pd.read_sql_query(
                query,
                db.get_connection(),
                params=[symbol, START_DATE, END_DATE],
                parse_dates={'date': '%Y-%m-%d %H:%M:%S'}
            )
            
            if len(df) == 0:
                logger.warning(f"No data found for {symbol} between {START_DATE} and {END_DATE}")
                continue
                
            df.set_index('date', inplace=True)
            
            # Create data feed
            data = bt.feeds.PandasData(
                dataname=df,
                fromdate=df.index[0],
                todate=df.index[-1],
                open='open',
                high='high',
                low='low',
                close='close',
                volume='volume',
                openinterest=-1
            )
            cerebro.adddata(data, name=symbol)
            logger.info(f"Added data feed for {symbol} with {len(df)} data points")
            
        except Exception as e:
            logger.error(f"Error adding data feed for {symbol}: {e}")
            continue
    
    # Set our desired cash start
    cerebro.broker.setcash(INITIAL_CASH)
    
    # Run the backtest
    logger.info("Running Cerebro...")
    results = cerebro.run()
    logger.info("Cerebro run complete.")

    # Print final portfolio value from the strategy's perspective
    final_value = results[0].broker.getvalue()
    logger.info(f"Final Portfolio Value (from strategy): ${final_value:,.2f}")

if __name__ == '__main__':
    run_backtest() 