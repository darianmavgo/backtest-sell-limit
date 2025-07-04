#!/usr/bin/env python3
"""
Simple backtest runner for SPXL strategy.
Loads SPXL data from SQLite and runs the strategy.
"""

import sqlite3
import backtrader as bt
import pandas as pd
import logging
import os
from strategies.SPXLStrategy import SPXLStrategy

# Configuration  
DB_FILE = "spxl_backtest.db"
START_DATE = "2024-06-01"
END_DATE = "2025-06-26"
INITIAL_CASH = 1_000_000.0

def setup_logging():
    """Setup simple console logging."""
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(levelname)s - %(message)s',
        handlers=[logging.StreamHandler()]
    )

def get_spxl_symbol():
    """Get SPXL symbol from database."""
    conn = sqlite3.connect(DB_FILE)
    query = "SELECT symbol FROM spxl_tickers LIMIT 1"
    df = pd.read_sql_query(query, conn)
    conn.close()
    
    if not df.empty:
        return df['symbol'].iloc[0]
    return None

def get_stock_data(symbol, start_date, end_date):
    """Get historical stock data for a symbol."""
    import datetime
    
    # Convert date strings to Unix timestamps for database query
    start_ts = int(datetime.datetime.strptime(start_date, "%Y-%m-%d").timestamp())
    end_ts = int(datetime.datetime.strptime(end_date, "%Y-%m-%d").timestamp())
    
    conn = sqlite3.connect(DB_FILE)
    query = """
        SELECT date, open, high, low, close, volume
        FROM stock_historical_data 
        WHERE symbol = ? AND date BETWEEN ? AND ?
        ORDER BY date
    """
    df = pd.read_sql_query(
        query, 
        conn, 
        params=[symbol, start_ts, end_ts]
    )
    conn.close()
    
    if df.empty:
        return None
    
    # Convert Unix timestamps back to datetime
    df['date'] = pd.to_datetime(df['date'], unit='s')
    df.set_index('date', inplace=True)
    return df

def run_backtest():
    """Run the backtest."""
    setup_logging()
    
    print(f"Starting SPXL backtest: {START_DATE} to {END_DATE}")
    
    # Initialize Cerebro
    cerebro = bt.Cerebro()
    cerebro.addstrategy(SPXLStrategy)
    cerebro.broker.setcash(INITIAL_CASH)
    
    # Load data for SPXL
    symbol = get_spxl_symbol()
    if symbol:
        df = get_stock_data(symbol, START_DATE, END_DATE)
        
        if df is not None and len(df) > 0:
            # Create Backtrader data feed
            data = bt.feeds.PandasData(
                dataname=df,
                fromdate=df.index[0],
                todate=df.index[-1]
            )
            cerebro.adddata(data, name=symbol)
            print(f"Loaded data for {symbol}")
        else:
            print(f"No data for {symbol}")
            return # Exit if no data for SPXL
    else:
        print("SPXL symbol not found in database.")
        return # Exit if SPXL symbol not found
    
    # Run backtest
    print("Running backtest...")
    initial_value = cerebro.broker.getvalue()
    results = cerebro.run()
    final_value = cerebro.broker.getvalue()
    
    # Results
    total_return = (final_value - initial_value) / initial_value * 100
    print(f"\nBacktest Results:")
    print(f"Initial Value: ${initial_value:,.2f}")
    print(f"Final Value: ${final_value:,.2f}")
    print(f"Total Return: {total_return:.2f}%")
    if results:
        print(f"Strategy: {results[0].__class__.__name__}")
    else:
        print("No strategy results")

if __name__ == "__main__":
    run_backtest()
