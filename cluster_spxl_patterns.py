#!/usr/bin/env python3
"""
SPXL 4-Day Price Pattern Clustering

This script clusters SPXL 4-day price change patterns into 10 clusters using K-means clustering.
It analyzes rolling 4-day windows of price changes to identify common market behavior patterns.

Uses scikit-learn for high-quality clustering analysis.
"""

import sqlite3
import pandas as pd
import numpy as np
from sklearn.cluster import KMeans
from sklearn.preprocessing import StandardScaler
from sklearn.decomposition import PCA
from sklearn.metrics import silhouette_score
import matplotlib.pyplot as plt
import seaborn as sns
from datetime import datetime
import warnings
warnings.filterwarnings('ignore')

def load_spxl_data(db_path="spxl_backtest.db"):
    """
    Load SPXL historical data from database
    
    Args:
        db_path (str): Path to SQLite database
        
    Returns:
        pandas.DataFrame: SPXL historical data
    """
    
    try:
        conn = sqlite3.connect(db_path)
        
        query = """
        SELECT 
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
        
        df = pd.read_sql_query(query, conn)
        conn.close()
        
        # Convert Unix timestamp to datetime
        df['date'] = pd.to_datetime(df['date'], unit='s')
        df = df.sort_values('date').reset_index(drop=True)
        
        print(f"Loaded {len(df)} SPXL trading days")
        return df
        
    except Exception as e:
        print(f"Error loading data: {e}")
        return pd.DataFrame()

def create_4day_patterns(df):
    """
    Create 4-day price change patterns
    
    Args:
        df (pandas.DataFrame): SPXL historical data
        
    Returns:
        pandas.DataFrame: 4-day patterns with features
    """
    
    # Calculate daily returns
    df['daily_return'] = df['close'].pct_change() * 100
    df['daily_range'] = ((df['high'] - df['low']) / df['open']) * 100
    df['open_to_close'] = ((df['close'] - df['open']) / df['open']) * 100
    df['gap'] = ((df['open'] - df['close'].shift(1)) / df['close'].shift(1)) * 100
    
    patterns = []
    
    # Create rolling 4-day windows
    for i in range(3, len(df)):  # Start from index 3 to have 4 days
        pattern = {
            'start_date': df.iloc[i-3]['date'],
            'end_date': df.iloc[i]['date'],
            'start_price': df.iloc[i-3]['open'],
            'end_price': df.iloc[i]['close'],
            
            # 4-day returns
            'day1_return': df.iloc[i-3]['daily_return'],
            'day2_return': df.iloc[i-2]['daily_return'],
            'day3_return': df.iloc[i-1]['daily_return'],
            'day4_return': df.iloc[i]['daily_return'],
            
            # 4-day ranges (volatility)
            'day1_range': df.iloc[i-3]['daily_range'],
            'day2_range': df.iloc[i-2]['daily_range'],
            'day3_range': df.iloc[i-1]['daily_range'],
            'day4_range': df.iloc[i]['daily_range'],
            
            # 4-day open-to-close moves
            'day1_oc': df.iloc[i-3]['open_to_close'],
            'day2_oc': df.iloc[i-2]['open_to_close'],
            'day3_oc': df.iloc[i-1]['open_to_close'],
            'day4_oc': df.iloc[i]['open_to_close'],
            
            # Pattern summary stats
            'total_4day_return': ((df.iloc[i]['close'] / df.iloc[i-3]['open']) - 1) * 100,
            'avg_daily_return': (df.iloc[i-3:i+1]['daily_return'].mean()),
            'volatility': df.iloc[i-3:i+1]['daily_return'].std(),
            'max_daily_return': df.iloc[i-3:i+1]['daily_return'].max(),
            'min_daily_return': df.iloc[i-3:i+1]['daily_return'].min(),
            'trend_direction': 1 if df.iloc[i]['close'] > df.iloc[i-3]['open'] else -1,
            
            # Volume pattern
            'avg_volume': df.iloc[i-3:i+1]['volume'].mean(),
            'volume_trend': 1 if df.iloc[i]['volume'] > df.iloc[i-3]['volume'] else -1,
        }
        
        patterns.append(pattern)
    
    patterns_df = pd.DataFrame(patterns)
    
    # Remove any rows with NaN values
    patterns_df = patterns_df.dropna()
    
    print(f"Created {len(patterns_df)} 4-day patterns")
    return patterns_df

def perform_clustering(patterns_df, n_clusters=10):
    """
    Perform K-means clustering on 4-day patterns
    
    Args:
        patterns_df (pandas.DataFrame): 4-day patterns
        n_clusters (int): Number of clusters
        
    Returns:
        tuple: (clustered_df, kmeans_model, scaler, feature_names)
    """
    
    # Select features for clustering
    feature_columns = [
        'day1_return', 'day2_return', 'day3_return', 'day4_return',
        'day1_range', 'day2_range', 'day3_range', 'day4_range',
        'day1_oc', 'day2_oc', 'day3_oc', 'day4_oc',
        'total_4day_return', 'avg_daily_return', 'volatility',
        'max_daily_return', 'min_daily_return', 'trend_direction'
    ]
    
    # Prepare feature matrix
    X = patterns_df[feature_columns].values
    
    # Standardize features
    scaler = StandardScaler()
    X_scaled = scaler.fit_transform(X)
    
    # Perform K-means clustering
    print(f"Performing K-means clustering with {n_clusters} clusters...")
    kmeans = KMeans(n_clusters=n_clusters, random_state=42, n_init=10)
    cluster_labels = kmeans.fit_predict(X_scaled)
    
    # Add cluster labels to dataframe
    clustered_df = patterns_df.copy()
    clustered_df['cluster'] = cluster_labels
    
    # Calculate silhouette score
    silhouette_avg = silhouette_score(X_scaled, cluster_labels)
    print(f"Silhouette Score: {silhouette_avg:.3f}")
    
    return clustered_df, kmeans, scaler, feature_columns

def analyze_clusters(clustered_df):
    """
    Analyze and describe each cluster
    
    Args:
        clustered_df (pandas.DataFrame): Clustered patterns
    """
    
    print("\n" + "=" * 100)
    print("CLUSTER ANALYSIS - 4-DAY SPXL PRICE PATTERNS")
    print("=" * 100)
    
    cluster_summary = []
    
    for cluster_id in sorted(clustered_df['cluster'].unique()):
        cluster_data = clustered_df[clustered_df['cluster'] == cluster_id]
        
        summary = {
            'cluster': cluster_id,
            'count': len(cluster_data),
            'percentage': (len(cluster_data) / len(clustered_df)) * 100,
            'avg_4day_return': cluster_data['total_4day_return'].mean(),
            'avg_volatility': cluster_data['volatility'].mean(),
            'avg_daily_return': cluster_data['avg_daily_return'].mean(),
            'win_rate': (cluster_data['total_4day_return'] > 0).mean() * 100,
            'best_4day_return': cluster_data['total_4day_return'].max(),
            'worst_4day_return': cluster_data['total_4day_return'].min(),
            'trend_up_pct': (cluster_data['trend_direction'] > 0).mean() * 100,
        }
        
        cluster_summary.append(summary)
        
        print(f"\n📊 CLUSTER {cluster_id} ({len(cluster_data)} patterns, {summary['percentage']:.1f}%)")
        print(f"   Average 4-Day Return: {summary['avg_4day_return']:.2f}%")
        print(f"   Average Volatility: {summary['avg_volatility']:.2f}%")
        print(f"   Win Rate: {summary['win_rate']:.1f}%")
        print(f"   Best 4-Day Return: {summary['best_4day_return']:.2f}%")
        print(f"   Worst 4-Day Return: {summary['worst_4day_return']:.2f}%")
        print(f"   Upward Trend: {summary['trend_up_pct']:.1f}%")
        
        # Pattern characteristics
        daily_returns = [
            cluster_data['day1_return'].mean(),
            cluster_data['day2_return'].mean(),
            cluster_data['day3_return'].mean(),
            cluster_data['day4_return'].mean()
        ]
        print(f"   Daily Pattern: [{daily_returns[0]:.1f}%, {daily_returns[1]:.1f}%, {daily_returns[2]:.1f}%, {daily_returns[3]:.1f}%]")
    
    # Convert to DataFrame for easier analysis
    summary_df = pd.DataFrame(cluster_summary)
    
    print("\n" + "=" * 100)
    print("CLUSTER SUMMARY TABLE")
    print("=" * 100)
    
    display_df = summary_df.copy()
    for col in ['avg_4day_return', 'avg_volatility', 'avg_daily_return', 'win_rate', 'best_4day_return', 'worst_4day_return', 'trend_up_pct']:
        display_df[col] = display_df[col].round(2)
    
    print(display_df.to_string(index=False))
    
    # Find best and worst performing clusters
    best_cluster = summary_df.loc[summary_df['avg_4day_return'].idxmax()]
    worst_cluster = summary_df.loc[summary_df['avg_4day_return'].idxmin()]
    
    print(f"\n🏆 BEST PERFORMING CLUSTER: {best_cluster['cluster']} (Avg: {best_cluster['avg_4day_return']:.2f}%)")
    print(f"📉 WORST PERFORMING CLUSTER: {worst_cluster['cluster']} (Avg: {worst_cluster['avg_4day_return']:.2f}%)")
    
    return summary_df

def save_results_to_db(clustered_df, db_path="spxl_backtest.db"):
    """
    Save clustering results to database
    
    Args:
        clustered_df (pandas.DataFrame): Clustered patterns
        db_path (str): Path to SQLite database
    """
    
    try:
        conn = sqlite3.connect(db_path)
        cursor = conn.cursor()
        
        # Drop table if exists
        cursor.execute("DROP TABLE IF EXISTS spxl_4day_clusters")
        
        # Create table
        create_sql = """
        CREATE TABLE spxl_4day_clusters (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            start_date TEXT NOT NULL,
            end_date TEXT NOT NULL,
            cluster INTEGER NOT NULL,
            start_price REAL NOT NULL,
            end_price REAL NOT NULL,
            total_4day_return REAL NOT NULL,
            avg_daily_return REAL NOT NULL,
            volatility REAL NOT NULL,
            trend_direction INTEGER NOT NULL,
            day1_return REAL,
            day2_return REAL,
            day3_return REAL,
            day4_return REAL,
            analysis_date TEXT NOT NULL
        )
        """
        
        cursor.execute(create_sql)
        
        # Prepare data for insertion
        current_time = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        
        insert_data = []
        for _, row in clustered_df.iterrows():
            insert_data.append((
                row['start_date'].strftime('%Y-%m-%d'),
                row['end_date'].strftime('%Y-%m-%d'),
                int(row['cluster']),
                row['start_price'],
                row['end_price'],
                row['total_4day_return'],
                row['avg_daily_return'],
                row['volatility'],
                int(row['trend_direction']),
                row['day1_return'],
                row['day2_return'],
                row['day3_return'],
                row['day4_return'],
                current_time
            ))
        
        # Insert data
        insert_sql = """
        INSERT INTO spxl_4day_clusters (
            start_date, end_date, cluster, start_price, end_price,
            total_4day_return, avg_daily_return, volatility, trend_direction,
            day1_return, day2_return, day3_return, day4_return, analysis_date
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        """
        
        cursor.executemany(insert_sql, insert_data)
        conn.commit()
        conn.close()
        
        print(f"\n✅ Saved {len(clustered_df)} clustered patterns to 'spxl_4day_clusters' table")
        
    except Exception as e:
        print(f"❌ Error saving to database: {e}")

def main():
    """Main function"""
    print("SPXL 4-Day Price Pattern Clustering Analysis")
    print("Using K-means clustering with scikit-learn")
    print("-" * 60)
    
    # Load data
    df = load_spxl_data()
    if df.empty:
        print("No data available for clustering")
        return
    
    # Create 4-day patterns
    patterns_df = create_4day_patterns(df)
    if patterns_df.empty:
        print("Could not create patterns")
        return
    
    # Perform clustering
    clustered_df, kmeans, scaler, features = perform_clustering(patterns_df, n_clusters=10)
    
    # Analyze clusters
    summary_df = analyze_clusters(clustered_df)
    
    # Save results
    save_results_to_db(clustered_df)
    
    # Save summary to CSV
    summary_df.to_csv('spxl_cluster_summary.csv', index=False)
    clustered_df.to_csv('spxl_4day_patterns_clustered.csv', index=False)
    
    print(f"\n📄 Results saved to:")
    print(f"   - Database table: spxl_4day_clusters")
    print(f"   - CSV files: spxl_cluster_summary.csv, spxl_4day_patterns_clustered.csv")

if __name__ == "__main__":
    main()