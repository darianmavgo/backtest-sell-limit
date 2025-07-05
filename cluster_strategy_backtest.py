#!/usr/bin/env python3
"""
SPXL Cluster-Based Strategy Backtest

This strategy uses the 4-day price pattern clusters to predict future SPXL movements
and make trading decisions based on historical cluster performance.

Strategy Logic:
- Analyze current 4-day pattern and classify it into one of the 10 clusters
- Make trading decisions based on cluster historical performance:
  * BUY: High-performing clusters (6, 1, 7, 9)
  * SELL/SHORT: Poor-performing clusters (4, 3, 0)
  * HOLD: Neutral clusters (2, 5, 8)
"""

import backtrader as bt
import sqlite3
import pandas as pd
import numpy as np
from datetime import datetime, timedelta
from sklearn.cluster import KMeans
from sklearn.preprocessing import StandardScaler
import pickle
import warnings
warnings.filterwarnings('ignore')

class ClusterStrategy(bt.Strategy):
    """
    Cluster-Based Trading Strategy for SPXL
    
    Uses pre-trained K-means clusters to classify 4-day patterns
    and make trading decisions based on cluster performance
    """
    
    params = (
        ('starting_cash', 100000),
        ('commission', 0.001),  # 0.1% commission
        ('position_size', 0.95),  # Use 95% of available cash
        ('stop_loss', 0.15),  # 15% stop loss
        ('take_profit', 0.20),  # 20% take profit
        ('min_confidence', 0.6),  # Minimum confidence for trades
    )
    
    def __init__(self):
        # Load pre-trained cluster model and performance data
        self.load_cluster_data()
        
        # Track recent price data for pattern matching
        self.price_history = []
        self.returns_history = []
        
        # Position tracking
        self.order = None
        self.entry_price = None
        self.entry_date = None
        
        # Performance tracking
        self.trades_count = 0
        self.wins = 0
        self.losses = 0
        self.total_profit = 0
        
        # Log strategy initialization
        self.log("Cluster-Based SPXL Strategy Initialized")
        self.log(f"Starting Portfolio: ${self.broker.getvalue():,.2f}")
        
    def load_cluster_data(self):
        """Load cluster model and performance statistics"""
        try:
            # Load cluster performance from database
            conn = sqlite3.connect('spxl_backtest.db')
            
            # Get cluster statistics
            query = """
            SELECT 
                cluster,
                COUNT(*) as count,
                AVG(total_4day_return) as avg_return,
                AVG(volatility) as avg_volatility,
                SUM(CASE WHEN total_4day_return > 0 THEN 1 ELSE 0 END) * 1.0 / COUNT(*) as win_rate
            FROM spxl_4day_clusters 
            GROUP BY cluster
            ORDER BY cluster
            """
            
            self.cluster_stats = pd.read_sql_query(query, conn)
            conn.close()
            
            # Define cluster trading signals based on performance
            self.cluster_signals = {
                0: 'SELL',     # -4.49% avg return, 9.6% win rate
                1: 'BUY',      # 7.80% avg return, 100% win rate
                2: 'HOLD',     # 3.59% avg return, 79.7% win rate
                3: 'SELL',     # -5.93% avg return, 1.3% win rate
                4: 'SELL',     # -13.79% avg return, 0% win rate
                5: 'HOLD',     # -2.87% avg return, 0% win rate
                6: 'BUY',      # 23.50% avg return, 100% win rate
                7: 'BUY',      # 5.91% avg return, 100% win rate
                8: 'HOLD',     # 2.67% avg return, 72.2% win rate
                9: 'BUY',      # 2.80% avg return, 100% win rate
            }
            
            # Cluster confidence scores (based on win rate and sample size)
            self.cluster_confidence = {
                0: 0.7,   # High confidence in negative signal
                1: 0.9,   # Very high confidence - large sample, 100% win rate
                2: 0.6,   # Medium confidence
                3: 0.8,   # High confidence in negative signal
                4: 0.9,   # Very high confidence in negative signal
                5: 0.5,   # Low confidence - contradictory stats
                6: 0.7,   # High confidence but tiny sample
                7: 0.6,   # Medium confidence - tiny sample
                8: 0.6,   # Medium confidence
                9: 0.9,   # Very high confidence - large sample, 100% win rate
            }
            
            self.log("Cluster model and statistics loaded successfully")
            
        except Exception as e:
            self.log(f"Error loading cluster data: {e}")
            # Default conservative strategy if cluster data fails
            self.cluster_signals = {i: 'HOLD' for i in range(10)}
            self.cluster_confidence = {i: 0.5 for i in range(10)}
    
    def classify_current_pattern(self):
        """
        Classify current 4-day pattern into a cluster
        Returns cluster ID and confidence
        """
        if len(self.returns_history) < 4:
            return None, 0.0
        
        # Get last 4 days of data
        recent_returns = self.returns_history[-4:]
        recent_prices = self.price_history[-4:]
        
        if len(recent_returns) < 4 or len(recent_prices) < 4:
            return None, 0.0
        
        # Calculate pattern features (same as training)
        features = self.calculate_pattern_features(recent_returns, recent_prices)
        
        # Simple pattern matching based on key characteristics
        # (In production, you'd use the actual trained model)
        cluster_id = self.simple_pattern_matching(features)
        confidence = self.cluster_confidence.get(cluster_id, 0.5)
        
        return cluster_id, confidence
    
    def calculate_pattern_features(self, returns, prices):
        """Calculate features for pattern classification"""
        day1_ret, day2_ret, day3_ret, day4_ret = returns
        
        # Calculate additional features
        total_4day_return = ((prices[-1] / prices[0]) - 1) * 100
        avg_daily_return = np.mean(returns)
        volatility = np.std(returns)
        max_return = max(returns)
        min_return = min(returns)
        trend_direction = 1 if prices[-1] > prices[0] else -1
        
        return {
            'day1_return': day1_ret,
            'day2_return': day2_ret,
            'day3_return': day3_ret,
            'day4_return': day4_ret,
            'total_4day_return': total_4day_return,
            'avg_daily_return': avg_daily_return,
            'volatility': volatility,
            'max_return': max_return,
            'min_return': min_return,
            'trend_direction': trend_direction,
        }
    
    def simple_pattern_matching(self, features):
        """
        Simple rule-based pattern matching to classify patterns
        (Simplified version of the ML model)
        """
        avg_ret = features['avg_daily_return']
        volatility = features['volatility']
        total_ret = features['total_4day_return']
        trend = features['trend_direction']
        
        # Rule-based classification based on cluster characteristics
        if total_ret > 15:  # Explosive gains
            return 6
        elif total_ret < -10:  # Crash pattern
            return 4
        elif total_ret < -4 and volatility > 3:  # Sharp drop
            return 3
        elif avg_ret > 2 and volatility < 3 and trend > 0:  # Steady uptrend
            return 1
        elif avg_ret > 0.5 and volatility < 2 and trend > 0:  # Small steady gains
            return 9
        elif total_ret < 0 and volatility < 2.5:  # Gradual decline
            return 5
        elif volatility > 4 and total_ret > 0:  # Volatile recovery
            if features['day1_return'] < -3:
                return 8
            else:
                return 2
        elif total_ret < -3 and volatility > 3:  # Sharp decline
            return 0
        else:  # Default to neutral
            return 5
    
    def next(self):
        """Main strategy logic called on each bar"""
        current_price = self.data.close[0]
        current_date = self.datas[0].datetime.date(0)
        
        # Update price and returns history
        if len(self.price_history) == 0:
            self.price_history.append(current_price)
            self.returns_history.append(0.0)
        else:
            prev_price = self.price_history[-1]
            daily_return = ((current_price / prev_price) - 1) * 100
            
            self.price_history.append(current_price)
            self.returns_history.append(daily_return)
            
            # Keep only last 10 days for efficiency
            if len(self.price_history) > 10:
                self.price_history = self.price_history[-10:]
                self.returns_history = self.returns_history[-10:]
        
        # Skip if we have pending orders
        if self.order:
            return
        
        # Check stop loss and take profit for existing positions
        if self.position:
            self.check_exit_conditions(current_price)
            return
        
        # Need at least 4 days of data for pattern classification
        if len(self.returns_history) < 4:
            return
        
        # Classify current 4-day pattern
        cluster_id, confidence = self.classify_current_pattern()
        
        if cluster_id is None or confidence < self.p.min_confidence:
            return
        
        # Get trading signal for this cluster
        signal = self.cluster_signals.get(cluster_id, 'HOLD')
        
        # Execute trading decision
        if signal == 'BUY' and not self.position:
            self.execute_buy(cluster_id, confidence, current_price)
        elif signal == 'SELL' and not self.position:
            # For this strategy, we'll avoid shorting and just stay in cash
            # In a real implementation, you could add short selling here
            pass
    
    def execute_buy(self, cluster_id, confidence, price):
        """Execute buy order"""
        available_cash = self.broker.getcash()
        position_value = available_cash * self.p.position_size
        size = int(position_value / price)
        
        if size > 0:
            self.order = self.buy(size=size)
            self.entry_price = price
            self.entry_date = self.datas[0].datetime.date(0)
            
            self.log(f"BUY SIGNAL: Cluster {cluster_id} (Conf: {confidence:.2f})")
            self.log(f"Buying {size} shares at ${price:.2f}")
    
    def check_exit_conditions(self, current_price):
        """Check stop loss and take profit conditions"""
        if not self.position or not self.entry_price:
            return
        
        pnl_pct = ((current_price / self.entry_price) - 1) * 100
        
        # Stop loss
        if pnl_pct <= -self.p.stop_loss * 100:
            self.order = self.close()
            self.log(f"STOP LOSS: {pnl_pct:.2f}% loss")
        
        # Take profit
        elif pnl_pct >= self.p.take_profit * 100:
            self.order = self.close()
            self.log(f"TAKE PROFIT: {pnl_pct:.2f}% gain")
    
    def notify_order(self, order):
        """Handle order notifications"""
        if order.status in [order.Submitted, order.Accepted]:
            return
        
        if order.status in [order.Completed]:
            if order.isbuy():
                self.log(f"BUY EXECUTED: {order.executed.size} shares at ${order.executed.price:.2f}")
                # Store buy order details
                self.buy_order_data = {
                    'date': self.datas[0].datetime.date(0),
                    'price': order.executed.price,
                    'size': order.executed.size,
                    'value': order.executed.value,
                    'commission': order.executed.comm
                }
                
            elif order.issell():
                self.log(f"SELL EXECUTED: {order.executed.size} shares at ${order.executed.price:.2f}")
                
                # Calculate trade performance and save to database
                if self.entry_price and hasattr(self, 'buy_order_data'):
                    pnl_pct = ((order.executed.price / self.entry_price) - 1) * 100
                    trade_profit = order.executed.size * (order.executed.price - self.entry_price)
                    
                    self.trades_count += 1
                    self.total_profit += trade_profit
                    
                    if pnl_pct > 0:
                        self.wins += 1
                    else:
                        self.losses += 1
                    
                    # Save trade to database
                    self.save_trade_to_db(order, pnl_pct, trade_profit)
                    
                    self.log(f"Trade P&L: {pnl_pct:.2f}% (${trade_profit:.2f})")
                    self.log(f"Portfolio: ${self.broker.getvalue():,.2f}")
        
        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            self.log(f"Order {order.getstatusname()}")
        
        self.order = None
    
    def stop(self):
        """Strategy completion summary"""
        final_value = self.broker.getvalue()
        total_return = ((final_value / self.p.starting_cash) - 1) * 100
        
        win_rate = (self.wins / self.trades_count * 100) if self.trades_count > 0 else 0
        
        self.log("=" * 80)
        self.log("CLUSTER STRATEGY BACKTEST RESULTS")
        self.log("=" * 80)
        self.log(f"Starting Portfolio: ${self.p.starting_cash:,.2f}")
        self.log(f"Final Portfolio: ${final_value:,.2f}")
        self.log(f"Total Return: {total_return:.2f}%")
        self.log(f"Total Trades: {self.trades_count}")
        self.log(f"Wins: {self.wins} | Losses: {self.losses}")
        self.log(f"Win Rate: {win_rate:.1f}%")
        
        if self.trades_count > 0:
            avg_profit = self.total_profit / self.trades_count
            self.log(f"Average Profit per Trade: ${avg_profit:.2f}")
        
        self.log("=" * 80)
    
    def save_trade_to_db(self, sell_order, pnl_pct, trade_profit):
        """Save completed trade to SQLite database"""
        try:
            conn = sqlite3.connect('spxl_backtest.db')
            cursor = conn.cursor()
            
            # Create table if it doesn't exist
            cursor.execute("""
                CREATE TABLE IF NOT EXISTS cluster_strategy_trades (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    entry_date TEXT NOT NULL,
                    exit_date TEXT NOT NULL,
                    entry_price REAL NOT NULL,
                    exit_price REAL NOT NULL,
                    shares INTEGER NOT NULL,
                    entry_value REAL NOT NULL,
                    exit_value REAL NOT NULL,
                    trade_profit REAL NOT NULL,
                    pnl_percentage REAL NOT NULL,
                    entry_commission REAL NOT NULL,
                    exit_commission REAL NOT NULL,
                    total_commission REAL NOT NULL,
                    net_profit REAL NOT NULL,
                    portfolio_value REAL NOT NULL,
                    created_at TEXT NOT NULL
                )
            """)
            
            # Calculate trade details
            total_commission = self.buy_order_data['commission'] + sell_order.executed.comm
            net_profit = trade_profit - total_commission
            current_time = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
            
            # Insert trade record
            cursor.execute("""
                INSERT INTO cluster_strategy_trades (
                    entry_date, exit_date, entry_price, exit_price, shares,
                    entry_value, exit_value, trade_profit, pnl_percentage,
                    entry_commission, exit_commission, total_commission, net_profit,
                    portfolio_value, created_at
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
            """, (
                self.buy_order_data['date'].strftime('%Y-%m-%d'),
                self.datas[0].datetime.date(0).strftime('%Y-%m-%d'),
                self.buy_order_data['price'],
                sell_order.executed.price,
                sell_order.executed.size,
                self.buy_order_data['value'],
                sell_order.executed.value,
                trade_profit,
                pnl_pct,
                self.buy_order_data['commission'],
                sell_order.executed.comm,
                total_commission,
                net_profit,
                self.broker.getvalue(),
                current_time
            ))
            
            conn.commit()
            conn.close()
            
            self.log(f"Trade saved to database: ID {cursor.lastrowid}")
            
        except Exception as e:
            self.log(f"Error saving trade to database: {e}")
    
    def log(self, txt, dt=None):
        """Enhanced logging"""
        dt = dt or self.datas[0].datetime.date(0)
        print(f'{dt.isoformat()} | {txt}')

def load_spxl_data():
    """Load SPXL data from database"""
    try:
        conn = sqlite3.connect('spxl_backtest.db')
        
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
        
        return df
        
    except Exception as e:
        print(f"Error loading SPXL data: {e}")
        return pd.DataFrame()

def run_backtest():
    """Run the cluster-based strategy backtest"""
    print("Loading SPXL data for cluster strategy backtest...")
    
    # Load data
    df = load_spxl_data()
    if df.empty:
        print("No data available for backtesting")
        return
    
    print(f"Loaded {len(df)} days of SPXL data")
    print(f"Date range: {df['date'].min().date()} to {df['date'].max().date()}")
    
    # Create Backtrader cerebro engine
    cerebro = bt.Cerebro()
    
    # Convert pandas DataFrame to Backtrader data feed
    data = bt.feeds.PandasData(
        dataname=df,
        datetime='date',
        open='open',
        high='high',
        low='low',
        close='close',
        volume='volume',
        openinterest=None
    )
    
    # Add data to cerebro
    cerebro.adddata(data)
    
    # Add strategy
    cerebro.addstrategy(ClusterStrategy)
    
    # Set initial cash
    starting_cash = 100000
    cerebro.broker.setcash(starting_cash)
    
    # Set commission
    cerebro.broker.setcommission(commission=0.001)  # 0.1%
    
    # Add analyzers
    cerebro.addanalyzer(bt.analyzers.SharpeRatio, _name='sharpe')
    cerebro.addanalyzer(bt.analyzers.DrawDown, _name='drawdown')
    cerebro.addanalyzer(bt.analyzers.Returns, _name='returns')
    
    print(f"\nStarting backtest with ${starting_cash:,}")
    print("=" * 60)
    
    # Run backtest
    results = cerebro.run()
    
    # Get results
    strat = results[0]
    
    # Print analyzer results
    print("\n" + "=" * 60)
    print("ANALYZER RESULTS")
    print("=" * 60)
    
    sharpe = strat.analyzers.sharpe.get_analysis()
    drawdown = strat.analyzers.drawdown.get_analysis()
    returns = strat.analyzers.returns.get_analysis()
    
    print(f"Sharpe Ratio: {sharpe.get('sharperatio', 'N/A')}")
    print(f"Max Drawdown: {drawdown.get('max', {}).get('drawdown', 'N/A'):.2f}%")
    print(f"Total Return: {returns.get('rtot', 'N/A'):.4f}")
    
    final_value = cerebro.broker.getvalue()
    print(f"\nFinal Portfolio Value: ${final_value:,.2f}")
    print(f"Total Return: {((final_value / starting_cash) - 1) * 100:.2f}%")

if __name__ == "__main__":
    run_backtest()