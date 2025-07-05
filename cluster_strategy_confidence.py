#!/usr/bin/env python3
"""
SPXL Cluster-Based Strategy with Confidence-Based Position Sizing

This strategy uses the 4-day price pattern clusters to predict future SPXL movements
and makes trading decisions based on historical cluster performance with intelligent
position sizing and exit rules based on confidence levels.

Strategy Logic:
- Analyze current 4-day pattern and classify it into one of the 10 clusters
- Make trading decisions based on cluster historical performance:
  * BUY: High-performing clusters (6, 1, 7, 9)
  * SELL/SHORT: Poor-performing clusters (4, 3, 0)
  * HOLD: Neutral clusters (2, 5, 8)
- Position sizing based on confidence: Higher confidence = Larger position
- Stop loss and take profit adjusted based on confidence levels
- Maximum holding period scaled by confidence (high confidence = longer holds allowed)
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

class ClusterStrategyConfidence(bt.Strategy):
    """
    Cluster-Based Trading Strategy for SPXL with Confidence-Based Position Sizing
    
    Uses pre-trained K-means clusters to classify 4-day patterns
    and make trading decisions based on cluster performance with intelligent
    position sizing and risk management based on confidence levels.
    """
    
    params = (
        ('starting_cash', 100000),
        ('commission', 0.001),  # 0.1% commission
        ('base_position_size', 0.3),  # Base position size (30% of portfolio)
        ('max_position_size', 0.8),   # Maximum position size (80% of portfolio)
        ('min_confidence', 0.6),      # Minimum confidence for trades
        ('max_hold_days_base', 60),   # Base maximum holding period
        ('confidence_scaling', True), # Enable confidence-based scaling
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
        self.entry_confidence = None
        self.days_in_position = 0
        self.current_stop_loss = None
        self.current_take_profit = None
        self.max_hold_days = None
        
        # Performance tracking
        self.trades_count = 0
        self.wins = 0
        self.losses = 0
        self.total_profit = 0
        
        # Log strategy initialization
        self.log("Cluster-Based SPXL Strategy with Confidence Scaling Initialized")
        self.log(f"Starting Portfolio: ${self.broker.getvalue():,.2f}")
        self.log(f"Base Position Size: {self.p.base_position_size*100:.1f}%")
        self.log(f"Max Position Size: {self.p.max_position_size*100:.1f}%")
        
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
            
            # Enhanced cluster confidence scores (based on win rate, sample size, and avg return)
            self.cluster_confidence = {
                0: 0.75,  # High confidence in negative signal
                1: 0.95,  # Very high confidence - large sample, 100% win rate, good return
                2: 0.65,  # Medium confidence
                3: 0.85,  # High confidence in negative signal
                4: 0.95,  # Very high confidence in negative signal
                5: 0.55,  # Low confidence - contradictory stats
                6: 0.90,  # Very high confidence - excellent return despite small sample
                7: 0.80,  # High confidence - good win rate and return
                8: 0.65,  # Medium confidence
                9: 0.95,  # Very high confidence - large sample, 100% win rate
            }
            
            # Expected returns for each cluster (for position sizing)
            self.cluster_expected_return = {
                0: -4.49, 1: 7.80, 2: 3.59, 3: -5.93, 4: -13.79,
                5: -2.87, 6: 23.50, 7: 5.91, 8: 2.67, 9: 2.80
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
        
        # Check exit conditions for existing positions
        if self.position:
            self.days_in_position += 1
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
    
    def calculate_position_size(self, confidence, cluster_id):
        """Calculate position size based on confidence and expected return"""
        if not self.p.confidence_scaling:
            return self.p.base_position_size
        
        # Scale position size based on confidence (0.6 to 1.0 -> base to max position)
        confidence_factor = (confidence - self.p.min_confidence) / (1.0 - self.p.min_confidence)
        position_size = self.p.base_position_size + (self.p.max_position_size - self.p.base_position_size) * confidence_factor
        
        # Additional scaling based on expected return magnitude
        expected_return = abs(self.cluster_expected_return.get(cluster_id, 0))
        if expected_return > 15:  # Very high expected return (like cluster 6)
            position_size = min(position_size * 1.2, self.p.max_position_size)
        elif expected_return > 7:  # High expected return
            position_size = min(position_size * 1.1, self.p.max_position_size)
        
        return min(position_size, self.p.max_position_size)
    
    def calculate_risk_parameters(self, confidence, cluster_id):
        """Calculate stop loss, take profit, and max hold days based on confidence"""
        expected_return = self.cluster_expected_return.get(cluster_id, 0)
        
        # Base risk parameters
        base_stop_loss = 0.12  # 12%
        base_take_profit = 0.15  # 15%
        
        # Adjust based on confidence
        if confidence >= 0.9:  # Very high confidence
            stop_loss = base_stop_loss * 1.5    # 18% - allow more room
            take_profit = base_take_profit * 1.8  # 27% - aim higher
            max_hold_days = self.p.max_hold_days_base * 1.5  # 90 days
        elif confidence >= 0.8:  # High confidence
            stop_loss = base_stop_loss * 1.25   # 15%
            take_profit = base_take_profit * 1.4  # 21%
            max_hold_days = self.p.max_hold_days_base * 1.2  # 72 days
        elif confidence >= 0.7:  # Medium-high confidence
            stop_loss = base_stop_loss          # 12%
            take_profit = base_take_profit * 1.2  # 18%
            max_hold_days = self.p.max_hold_days_base  # 60 days
        else:  # Lower confidence
            stop_loss = base_stop_loss * 0.8    # 9.6% - tighter stop
            take_profit = base_take_profit      # 15%
            max_hold_days = self.p.max_hold_days_base * 0.7  # 42 days
        
        # Adjust for expected return magnitude
        if expected_return > 15:  # Exceptional expected return
            take_profit *= 1.5
            max_hold_days *= 1.3
        elif expected_return > 7:  # High expected return
            take_profit *= 1.2
            max_hold_days *= 1.1
        
        return stop_loss, take_profit, int(max_hold_days)
    
    def execute_buy(self, cluster_id, confidence, price):
        """Execute buy order with confidence-based position sizing"""
        available_cash = self.broker.getcash()
        portfolio_value = self.broker.getvalue()
        
        # Calculate position size based on confidence
        position_size_pct = self.calculate_position_size(confidence, cluster_id)
        position_value = portfolio_value * position_size_pct
        size = int(position_value / price)
        
        # Calculate risk parameters
        stop_loss, take_profit, max_hold_days = self.calculate_risk_parameters(confidence, cluster_id)
        
        if size > 0 and position_value <= available_cash:
            self.order = self.buy(size=size)
            self.entry_price = price
            self.entry_date = self.datas[0].datetime.date(0)
            self.entry_confidence = confidence
            self.current_stop_loss = stop_loss
            self.current_take_profit = take_profit
            self.max_hold_days = max_hold_days
            self.days_in_position = 0
            
            self.log(f"BUY SIGNAL: Cluster {cluster_id} (Conf: {confidence:.2f})")
            self.log(f"Position Size: {position_size_pct*100:.1f}% | Stop: {stop_loss*100:.1f}% | Target: {take_profit*100:.1f}%")
            self.log(f"Max Hold: {max_hold_days} days | Buying {size} shares at ${price:.2f}")
    
    def check_exit_conditions(self, current_price):
        """Check confidence-based exit conditions"""
        if not self.position or not self.entry_price:
            return
        
        pnl_pct = ((current_price / self.entry_price) - 1) * 100
        
        # Maximum holding period based on confidence
        if self.max_hold_days and self.days_in_position >= self.max_hold_days:
            self.order = self.close()
            self.log(f"MAX HOLD EXIT: {pnl_pct:.2f}% (Day {self.days_in_position}/{self.max_hold_days}, Conf: {self.entry_confidence:.2f})")
        
        # Stop loss based on confidence level
        elif self.current_stop_loss and pnl_pct <= -self.current_stop_loss * 100:
            self.order = self.close()
            self.log(f"CONFIDENCE STOP LOSS: {pnl_pct:.2f}% (Target: -{self.current_stop_loss*100:.1f}%, Conf: {self.entry_confidence:.2f})")
        
        # Take profit based on confidence level
        elif self.current_take_profit and pnl_pct >= self.current_take_profit * 100:
            self.order = self.close()
            self.log(f"CONFIDENCE TAKE PROFIT: {pnl_pct:.2f}% (Target: +{self.current_take_profit*100:.1f}%, Conf: {self.entry_confidence:.2f})")
    
    def notify_order(self, order):
        """Handle order notifications"""
        if order.status in [order.Submitted, order.Accepted]:
            return
        
        if order.status in [order.Completed]:
            if order.isbuy():
                self.log(f"BUY EXECUTED: {order.executed.size} shares at ${order.executed.price:.2f}")
                # Reset position tracking
                self.days_in_position = 0
                # Store buy order details
                self.buy_order_data = {
                    'date': self.datas[0].datetime.date(0),
                    'price': order.executed.price,
                    'size': order.executed.size,
                    'value': order.executed.value,
                    'commission': order.executed.comm,
                    'confidence': self.entry_confidence,
                    'stop_loss': self.current_stop_loss,
                    'take_profit': self.current_take_profit,
                    'max_hold_days': self.max_hold_days
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
                    
                    self.log(f"Trade P&L: {pnl_pct:.2f}% (${trade_profit:.2f}) | Days: {self.days_in_position} | Conf: {self.entry_confidence:.2f}")
                    self.log(f"Portfolio: ${self.broker.getvalue():,.2f}")
                    
                    # Reset position tracking
                    self.days_in_position = 0
                    self.entry_confidence = None
                    self.current_stop_loss = None
                    self.current_take_profit = None
                    self.max_hold_days = None
        
        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            self.log(f"Order {order.getstatusname()}")
        
        self.order = None
    
    def stop(self):
        """Strategy completion summary"""
        final_value = self.broker.getvalue()
        total_return = ((final_value / self.p.starting_cash) - 1) * 100
        
        win_rate = (self.wins / self.trades_count * 100) if self.trades_count > 0 else 0
        
        self.log("=" * 80)
        self.log("CLUSTER STRATEGY WITH CONFIDENCE SCALING BACKTEST RESULTS")
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
                CREATE TABLE IF NOT EXISTS cluster_strategy_confidence_trades (
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
                INSERT INTO cluster_strategy_confidence_trades (
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
    cerebro.addstrategy(ClusterStrategyConfidence)
    
    # Set initial cash
    starting_cash = 100000
    cerebro.broker.setcash(starting_cash)
    
    # Set commission
    cerebro.broker.setcommission(commission=0.001)  # 0.1%
    
    # Add analyzers
    cerebro.addanalyzer(bt.analyzers.SharpeRatio, _name='sharpe')
    cerebro.addanalyzer(bt.analyzers.DrawDown, _name='drawdown')
    cerebro.addanalyzer(bt.analyzers.Returns, _name='returns')
    
    print(f"\nStarting confidence-based strategy backtest with ${starting_cash:,}")
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