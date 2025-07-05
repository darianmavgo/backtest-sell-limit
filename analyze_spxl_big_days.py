#!/usr/bin/env python3
"""
SPXL Big Days Analyzer

This script analyzes SPXL historical data to find all days where SPXL went up 7% or more
in one day, calculated as (day's high - day's open) / day's open * 100.

Uses data from spxl_backtest.db -> stock_historical_data table
"""

import sqlite3
import pandas as pd
from datetime import datetime
import sys

def analyze_spxl_big_days(db_path="spxl_backtest.db", min_gain_percent=7.0):
    """
    Analyze SPXL historical data for big up days
    
    Args:
        db_path (str): Path to the SQLite database
        min_gain_percent (float): Minimum gain percentage threshold (default: 7.0%)
    
    Returns:
        pandas.DataFrame: Table of big up days
    """
    
    try:
        # Connect to database
        print(f"Connecting to database: {db_path}")
        conn = sqlite3.connect(db_path)
        
        # Query SPXL historical data
        query = """
        SELECT 
            symbol,
            date,
            open,
            high,
            low,
            close,
            adj_close,
            volume
        FROM stock_historical_data 
        WHERE symbol = 'SPXL'
        ORDER BY date ASC
        """
        
        print("Querying SPXL historical data...")
        df = pd.read_sql_query(query, conn)
        conn.close()
        
        if df.empty:
            print("ERROR: No SPXL data found in database!")
            return pd.DataFrame()
        
        print(f"Loaded {len(df)} SPXL trading days")
        
        # Convert Unix timestamp to readable date
        df['date'] = pd.to_datetime(df['date'], unit='s')
        df['date_str'] = df['date'].dt.strftime('%Y-%m-%d')
        
        # Calculate intraday gain: (high - open) / open * 100
        df['intraday_gain_pct'] = ((df['high'] - df['open']) / df['open']) * 100
        
        # Calculate daily return: (close - open) / open * 100  
        df['daily_return_pct'] = ((df['close'] - df['open']) / df['open']) * 100
        
        # Filter for big up days (>= min_gain_percent)
        big_days = df[df['intraday_gain_pct'] >= min_gain_percent].copy()
        
        # Sort by intraday gain descending
        big_days = big_days.sort_values('intraday_gain_pct', ascending=False)
        
        # Reset index
        big_days = big_days.reset_index(drop=True)
        
        print(f"\nFound {len(big_days)} days where SPXL gained {min_gain_percent}%+ intraday")
        
        return big_days
        
    except sqlite3.Error as e:
        print(f"Database error: {e}")
        return pd.DataFrame()
    except Exception as e:
        print(f"Error: {e}")
        return pd.DataFrame()

def display_results(big_days_df, min_gain_percent=7.0):
    """
    Display the results in a nicely formatted table
    
    Args:
        big_days_df (pandas.DataFrame): DataFrame with big up days
        min_gain_percent (float): Minimum gain percentage used
    """
    
    if big_days_df.empty:
        print("No big up days found!")
        return
    
    print("\n" + "=" * 100)
    print(f"SPXL BIG UP DAYS ANALYSIS - Days with {min_gain_percent}%+ Intraday Gains")
    print("=" * 100)
    print(f"Calculation: (Day's High - Day's Open) / Day's Open * 100")
    print("=" * 100)
    
    # Create display table
    display_cols = [
        'date_str', 'open', 'high', 'low', 'close', 
        'intraday_gain_pct', 'daily_return_pct', 'volume'
    ]
    
    display_df = big_days_df[display_cols].copy()
    
    # Round numeric columns for better display
    display_df['open'] = display_df['open'].round(2)
    display_df['high'] = display_df['high'].round(2)
    display_df['low'] = display_df['low'].round(2)
    display_df['close'] = display_df['close'].round(2)
    display_df['intraday_gain_pct'] = display_df['intraday_gain_pct'].round(2)
    display_df['daily_return_pct'] = display_df['daily_return_pct'].round(2)
    display_df['volume'] = display_df['volume'].apply(lambda x: f"{x:,}")
    
    # Rename columns for display
    display_df.columns = [
        'Date', 'Open', 'High', 'Low', 'Close', 
        'Intraday Gain %', 'Daily Return %', 'Volume'
    ]
    
    # Print table
    print(display_df.to_string(index=False, max_rows=None, max_cols=None))
    
    # Summary statistics
    print("\n" + "=" * 100)
    print("SUMMARY STATISTICS")
    print("=" * 100)
    print(f"Total Big Up Days: {len(big_days_df)}")
    print(f"Average Intraday Gain: {big_days_df['intraday_gain_pct'].mean():.2f}%")
    print(f"Maximum Intraday Gain: {big_days_df['intraday_gain_pct'].max():.2f}%")
    print(f"Minimum Intraday Gain: {big_days_df['intraday_gain_pct'].min():.2f}%")
    
    # Best day details
    best_day = big_days_df.iloc[0]
    print(f"\nBEST DAY: {best_day['date_str']}")
    print(f"  Open: ${best_day['open']:.2f}")
    print(f"  High: ${best_day['high']:.2f}")
    print(f"  Intraday Gain: {best_day['intraday_gain_pct']:.2f}%")
    print(f"  Volume: {best_day['volume']:,}")
    
    # Year breakdown
    big_days_df['year'] = big_days_df['date'].dt.year
    year_counts = big_days_df['year'].value_counts().sort_index()
    
    print(f"\nBY YEAR:")
    for year, count in year_counts.items():
        print(f"  {year}: {count} big up days")
    
    print("=" * 100)

def save_to_database(big_days_df, db_path="spxl_backtest.db", table_name="spxl_big_days_7pct"):
    """
    Save the big days results to SQLite database table
    
    Args:
        big_days_df (pandas.DataFrame): DataFrame with big up days
        db_path (str): Path to the SQLite database
        table_name (str): Name of the table to create/update
    """
    
    if big_days_df.empty:
        print("No data to save to database")
        return
    
    try:
        # Connect to database
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()
        
        # Drop table if exists and create new one
        cursor.execute(f"DROP TABLE IF EXISTS {table_name}")
        
        # Create table
        create_table_sql = f"""
        CREATE TABLE {table_name} (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            date TEXT NOT NULL,
            symbol TEXT NOT NULL,
            open_price REAL NOT NULL,
            high_price REAL NOT NULL,
            low_price REAL NOT NULL,
            close_price REAL NOT NULL,
            adj_close REAL NOT NULL,
            volume INTEGER NOT NULL,
            intraday_gain_pct REAL NOT NULL,
            daily_return_pct REAL NOT NULL,
            date_unix INTEGER NOT NULL,
            analysis_date TEXT NOT NULL
        )
        """
        
        cursor.execute(create_table_sql)
        
        # Prepare data for insertion
        insert_data = []
        current_time = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        
        for _, row in big_days_df.iterrows():
            insert_data.append((
                row['date_str'],           # date
                row['symbol'],             # symbol
                row['open'],               # open_price
                row['high'],               # high_price
                row['low'],                # low_price
                row['close'],              # close_price
                row['adj_close'],          # adj_close
                row['volume'],             # volume
                row['intraday_gain_pct'],  # intraday_gain_pct
                row['daily_return_pct'],   # daily_return_pct
                row['date'].timestamp(),   # date_unix
                current_time               # analysis_date
            ))
        
        # Insert data
        insert_sql = f"""
        INSERT INTO {table_name} (
            date, symbol, open_price, high_price, low_price, close_price, 
            adj_close, volume, intraday_gain_pct, daily_return_pct, 
            date_unix, analysis_date
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """
        
        cursor.executemany(insert_sql, insert_data)
        
        # Commit changes
        conn.commit()
        conn.close()
        
        print(f"\n✅ Successfully saved {len(big_days_df)} records to table '{table_name}' in {db_path}")
        print(f"   Table columns: date, symbol, open_price, high_price, low_price, close_price,")
        print(f"                  adj_close, volume, intraday_gain_pct, daily_return_pct")
        
    except sqlite3.Error as e:
        print(f"❌ Database error while saving: {e}")
    except Exception as e:
        print(f"❌ Error while saving to database: {e}")

def main():
    """Main function"""
    print("SPXL Big Days Analyzer")
    print("Analyzing days with 7%+ intraday gains (High vs Open)")
    print("-" * 60)
    
    # Set parameters
    db_path = "spxl_backtest.db"
    min_gain_percent = 7.0
    
    # Allow command line override of gain percentage
    if len(sys.argv) > 1:
        try:
            min_gain_percent = float(sys.argv[1])
            print(f"Using custom minimum gain threshold: {min_gain_percent}%")
        except ValueError:
            print(f"Invalid gain percentage, using default: {min_gain_percent}%")
    
    # Analyze big days
    big_days = analyze_spxl_big_days(db_path, min_gain_percent)
    
    # Display results
    display_results(big_days, min_gain_percent)
    
    # Save to database table
    if not big_days.empty:
        save_to_database(big_days, db_path, "spxl_big_days_7pct")
        
        # Also save to CSV for backup
        csv_filename = f"spxl_big_days_{min_gain_percent}pct.csv"
        big_days.to_csv(csv_filename, index=False)
        print(f"📄 Backup CSV saved to: {csv_filename}")

if __name__ == "__main__":
    main()