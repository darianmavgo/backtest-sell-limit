#!/usr/bin/env python3
"""
SPY Buy and Hold Comparison

This script downloads SPY data for the same period as the cluster strategy
and calculates the buy-and-hold returns for comparison.
"""

import yfinance as yf
import pandas as pd
import sqlite3
from datetime import datetime

def download_spy_data():
    """Download SPY data and save to database"""
    print("Downloading SPY data from Yahoo Finance...")
    
    # Download SPY data for the same period as the cluster strategy
    spy = yf.Ticker("SPY")
    
    # Get data from 2020-07-20 (first trade date) to 2025-05-13 (last trade date)
    spy_data = spy.history(start="2020-07-20", end="2025-05-14")
    
    if spy_data.empty:
        print("Failed to download SPY data")
        return pd.DataFrame()
    
    # Reset index to make Date a column
    spy_data = spy_data.reset_index()
    spy_data['Symbol'] = 'SPY'
    
    print(f"Downloaded {len(spy_data)} days of SPY data")
    print(f"Date range: {spy_data['Date'].min().date()} to {spy_data['Date'].max().date()}")
    
    return spy_data

def calculate_spy_returns(spy_data, start_date="2020-07-20", end_date="2025-05-13"):
    """Calculate SPY buy-and-hold returns"""
    
    # Convert dates and handle timezone
    spy_data['Date'] = pd.to_datetime(spy_data['Date']).dt.tz_localize(None)  # Remove timezone
    start_dt = pd.to_datetime(start_date)
    end_dt = pd.to_datetime(end_date)
    
    # Get the closest dates to our start and end
    start_price_row = spy_data[spy_data['Date'] >= start_dt].iloc[0] if len(spy_data[spy_data['Date'] >= start_dt]) > 0 else spy_data.iloc[0]
    end_price_row = spy_data[spy_data['Date'] <= end_dt].iloc[-1] if len(spy_data[spy_data['Date'] <= end_dt]) > 0 else spy_data.iloc[-1]
    
    start_price = start_price_row['Close']
    end_price = end_price_row['Close']
    
    actual_start_date = start_price_row['Date'].date()
    actual_end_date = end_price_row['Date'].date()
    
    # Calculate returns
    total_return = ((end_price / start_price) - 1) * 100
    years = (actual_end_date - actual_start_date).days / 365.25
    annualized_return = ((end_price / start_price) ** (1/years) - 1) * 100
    
    # Calculate portfolio value for $100,000 investment
    starting_value = 100000
    final_value = starting_value * (end_price / start_price)
    
    print("\n" + "="*80)
    print("SPY BUY AND HOLD ANALYSIS")
    print("="*80)
    print(f"Start Date: {actual_start_date}")
    print(f"End Date: {actual_end_date}")
    print(f"Period: {years:.2f} years")
    print(f"Starting Price: ${start_price:.2f}")
    print(f"Ending Price: ${end_price:.2f}")
    print(f"Starting Portfolio: ${starting_value:,.2f}")
    print(f"Final Portfolio: ${final_value:,.2f}")
    print(f"Total Return: {total_return:.2f}%")
    print(f"Annualized Return: {annualized_return:.2f}%")
    print("="*80)
    
    return {
        'strategy': 'SPY Buy & Hold',
        'start_date': actual_start_date,
        'end_date': actual_end_date,
        'years': years,
        'start_price': start_price,
        'end_price': end_price,
        'starting_value': starting_value,
        'final_value': final_value,
        'total_return_pct': total_return,
        'annualized_return_pct': annualized_return
    }

def calculate_yearly_spy_returns(spy_data):
    """Calculate year-by-year SPY returns"""
    spy_data['Date'] = pd.to_datetime(spy_data['Date']).dt.tz_localize(None)  # Remove timezone
    spy_data['Year'] = spy_data['Date'].dt.year
    
    yearly_returns = []
    years = sorted(spy_data['Year'].unique())
    
    portfolio_value = 100000  # Starting value
    
    for year in years:
        year_data = spy_data[spy_data['Year'] == year].copy()
        
        if len(year_data) == 0:
            continue
            
        # Get first and last trading day of the year
        start_price = year_data.iloc[0]['Close']
        end_price = year_data.iloc[-1]['Close']
        
        # Calculate annual return
        annual_return = ((end_price / start_price) - 1) * 100
        
        # Update portfolio value
        portfolio_value = portfolio_value * (end_price / start_price)
        
        yearly_returns.append({
            'year': year,
            'start_price': start_price,
            'end_price': end_price,
            'annual_return_pct': annual_return,
            'portfolio_value': portfolio_value,
            'trading_days': len(year_data)
        })
    
    return yearly_returns

def save_spy_comparison_to_db(spy_data, spy_summary, yearly_returns):
    """Save SPY data and comparison to database"""
    try:
        conn = sqlite3.connect('spxl_backtest.db')
        cursor = conn.cursor()
        
        # Create SPY historical data table
        cursor.execute("DROP TABLE IF EXISTS spy_historical_data")
        cursor.execute("""
            CREATE TABLE spy_historical_data (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                date TEXT NOT NULL,
                open REAL NOT NULL,
                high REAL NOT NULL,
                low REAL NOT NULL,
                close REAL NOT NULL,
                adj_close REAL NOT NULL,
                volume INTEGER NOT NULL,
                created_at TEXT NOT NULL
            )
        """)
        
        # Insert SPY data
        current_time = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        spy_insert_data = []
        
        for _, row in spy_data.iterrows():
            spy_insert_data.append((
                row['Date'].strftime('%Y-%m-%d'),
                row['Open'],
                row['High'], 
                row['Low'],
                row['Close'],
                row['Close'],  # Assuming Close = Adj Close for simplicity
                int(row['Volume']),
                current_time
            ))
        
        cursor.executemany("""
            INSERT INTO spy_historical_data (date, open, high, low, close, adj_close, volume, created_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)
        """, spy_insert_data)
        
        # Create strategy comparison table
        cursor.execute("DROP TABLE IF EXISTS strategy_comparison")
        cursor.execute("""
            CREATE TABLE strategy_comparison (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                strategy_name TEXT NOT NULL,
                start_date TEXT NOT NULL,
                end_date TEXT NOT NULL,
                years REAL NOT NULL,
                starting_value REAL NOT NULL,
                final_value REAL NOT NULL,
                total_return_pct REAL NOT NULL,
                annualized_return_pct REAL NOT NULL,
                created_at TEXT NOT NULL
            )
        """)
        
        # Insert SPY summary
        cursor.execute("""
            INSERT INTO strategy_comparison (
                strategy_name, start_date, end_date, years, starting_value, 
                final_value, total_return_pct, annualized_return_pct, created_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        """, (
            spy_summary['strategy'],
            spy_summary['start_date'].strftime('%Y-%m-%d'),
            spy_summary['end_date'].strftime('%Y-%m-%d'),
            spy_summary['years'],
            spy_summary['starting_value'],
            spy_summary['final_value'],
            spy_summary['total_return_pct'],
            spy_summary['annualized_return_pct'],
            current_time
        ))
        
        # Get cluster strategy data for comparison
        cluster_query = """
        SELECT 
            'Cluster Strategy' as strategy_name,
            MIN(entry_date) as start_date,
            MAX(exit_date) as end_date,
            (julianday(MAX(exit_date)) - julianday(MIN(entry_date))) / 365.25 as years,
            100000 as starting_value,
            MAX(portfolio_value) as final_value,
            ((MAX(portfolio_value) / 100000 - 1) * 100) as total_return_pct,
            (POW(MAX(portfolio_value) / 100000, 1.0 / ((julianday(MAX(exit_date)) - julianday(MIN(entry_date))) / 365.25)) - 1) * 100 as annualized_return_pct
        FROM cluster_strategy_trades
        """
        
        cluster_data = cursor.execute(cluster_query).fetchone()
        if cluster_data:
            cursor.execute("""
                INSERT INTO strategy_comparison (
                    strategy_name, start_date, end_date, years, starting_value, 
                    final_value, total_return_pct, annualized_return_pct, created_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
            """, (*cluster_data, current_time))
        
        # Create yearly comparison table
        cursor.execute("DROP TABLE IF EXISTS yearly_spy_returns")
        cursor.execute("""
            CREATE TABLE yearly_spy_returns (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                year INTEGER NOT NULL,
                start_price REAL NOT NULL,
                end_price REAL NOT NULL,
                annual_return_pct REAL NOT NULL,
                portfolio_value REAL NOT NULL,
                trading_days INTEGER NOT NULL,
                created_at TEXT NOT NULL
            )
        """)
        
        # Insert yearly SPY returns
        for year_data in yearly_returns:
            cursor.execute("""
                INSERT INTO yearly_spy_returns (
                    year, start_price, end_price, annual_return_pct, 
                    portfolio_value, trading_days, created_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?)
            """, (
                year_data['year'],
                year_data['start_price'],
                year_data['end_price'],
                year_data['annual_return_pct'],
                year_data['portfolio_value'],
                year_data['trading_days'],
                current_time
            ))
        
        conn.commit()
        conn.close()
        
        print(f"\n✅ Saved SPY data and comparison to database")
        print(f"   - {len(spy_data)} days of SPY historical data")
        print(f"   - {len(yearly_returns)} years of annual returns")
        print(f"   - Strategy comparison table created")
        
    except Exception as e:
        print(f"❌ Error saving to database: {e}")

def create_comparison_view():
    """Create a view to easily compare strategies"""
    try:
        conn = sqlite3.connect('spxl_backtest.db')
        cursor = conn.cursor()
        
        cursor.execute("DROP VIEW IF EXISTS strategy_performance_comparison")
        cursor.execute("""
            CREATE VIEW strategy_performance_comparison AS
            SELECT 
                strategy_name,
                start_date,
                end_date,
                ROUND(years, 2) as years,
                ROUND(starting_value, 2) as starting_value,
                ROUND(final_value, 2) as final_value,
                ROUND(total_return_pct, 2) as total_return_pct,
                ROUND(annualized_return_pct, 2) as annualized_return_pct,
                ROUND(final_value - starting_value, 2) as absolute_profit
            FROM strategy_comparison
            ORDER BY annualized_return_pct DESC
        """)
        
        conn.commit()
        conn.close()
        
        print("✅ Created strategy comparison view")
        
    except Exception as e:
        print(f"❌ Error creating view: {e}")

def main():
    """Main function"""
    print("SPY Buy and Hold Comparison Analysis")
    print("-" * 60)
    
    # Download SPY data
    spy_data = download_spy_data()
    if spy_data.empty:
        print("Cannot proceed without SPY data")
        return
    
    # Calculate overall returns
    spy_summary = calculate_spy_returns(spy_data)
    
    # Calculate yearly returns
    yearly_returns = calculate_yearly_spy_returns(spy_data)
    
    print("\nYEAR-BY-YEAR SPY RETURNS:")
    print("-" * 60)
    for year_data in yearly_returns:
        print(f"{year_data['year']}: {year_data['annual_return_pct']:+6.2f}% | Portfolio: ${year_data['portfolio_value']:,.2f}")
    
    # Save to database
    save_spy_comparison_to_db(spy_data, spy_summary, yearly_returns)
    
    # Create comparison view
    create_comparison_view()

if __name__ == "__main__":
    main()