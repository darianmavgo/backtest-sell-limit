import backtrader as bt
import sqlite3
import json
import datetime
import sys
import pandas as pd
import os

class StrategyHistory(bt.Strategy):
    params = (
        ('db_path', 'strategy_history.db'),
        ('symbol', 'TSLA'),
    )

    def __init__(self):
        self.order_history = []
        self.trade_history = []
        self.conn = sqlite3.connect(self.p.db_path)
        self.cursor = self.conn.cursor()
        self.create_table()
        
        # Sample indicators for a basic strategy
        self.sma = bt.indicators.SimpleMovingAverage(self.data.close, period=20)
        self.order = None

    def create_table(self):
        self.cursor.execute('''
            CREATE TABLE IF NOT EXISTS strategy_history (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                timestamp TEXT,
                type TEXT,
                symbol TEXT,
                price REAL,
                size REAL,
                status TEXT,
                order_id TEXT
            )
        ''')
        self.conn.commit()

    def log_order(self, order):
        timestamp = datetime.datetime.now().isoformat()
        order_type = 'BUY' if order.isbuy() else 'SELL'
        status = order.getstatusname()
        self.cursor.execute('''
            INSERT INTO strategy_history (timestamp, type, symbol, price, size, status, order_id)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        ''', (timestamp, f"ORDER_{order_type}", self.p.symbol, order.price, order.size, status, str(order.ref)))
        self.conn.commit()
        
        # Collect for web output
        self.order_history.append({
            'timestamp': timestamp,
            'type': f"ORDER_{order_type}",
            'symbol': self.p.symbol,
            'price': order.price,
            'size': order.size,
            'status': status,
            'order_id': str(order.ref)
        })

    def log_trade(self, trade):
        timestamp = datetime.datetime.now().isoformat()
        trade_type = 'BUY' if trade.long else 'SELL'
        status = 'CLOSED' if trade.isclosed else 'OPEN'
        self.cursor.execute('''
            INSERT INTO strategy_history (timestamp, type, symbol, price, size, status, order_id)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        ''', (timestamp, f"TRADE_{trade_type}", self.p.symbol, trade.price, trade.size, status, str(trade.ref)))
        self.conn.commit()
        
        # Collect for web output
        self.trade_history.append({
            'timestamp': timestamp,
            'type': f"TRADE_{trade_type}",
            'symbol': self.p.symbol,
            'price': trade.price,
            'size': trade.size,
            'status': status,
            'order_id': str(trade.ref)
        })

    def notify_order(self, order):
        if order.status in [order.Completed, order.Canceled, order.Margin, order.Rejected]:
            self.log_order(order)
            self.order = None

    def notify_trade(self, trade):
        self.log_trade(trade)

    def next(self):
        # Simple SMA crossover strategy for demonstration
        if self.order is None:
            if self.data.close[0] > self.sma[0] and self.data.close[-1] <= self.sma[-1]:
                self.order = self.buy(size=100)
            elif self.data.close[0] < self.sma[0] and self.data.close[-1] >= self.sma[-1]:
                self.order = self.sell(size=100)

    def stop(self):
        # Output history as JSON for web response
        history = {'orders': self.order_history, 'trades': self.trade_history}
        print(json.dumps(history, indent=2))
        self.conn.close()

def get_historical_data_from_db(symbol='TSLA', start_date='2024-06-01', end_date='2024-12-31'):
    """Get historical data from the backtest database"""
    db_path = '/Users/darianhickman/Documents/Github/backtest-sell-limit/backtest_sell_limits.db'
    
    if not os.path.exists(db_path):
        print(f"❌ Database {db_path} not found.")
        return None
    
    try:
        conn = sqlite3.connect(db_path)
        
        # Try to get data from stock_historical_data table
        query = """
        SELECT date, open, high, low, close, volume 
        FROM stock_historical_data 
        WHERE symbol = ? AND date BETWEEN ? AND ?
        ORDER BY date
        """
        
        df = pd.read_sql_query(query, conn, params=[symbol, start_date, end_date])
        conn.close()
        
        if df.empty:
            print(f"❌ No historical data found for {symbol} in database.")
            return None
        
        # Convert date column to datetime
        df['date'] = pd.to_datetime(df['date'])
        df.set_index('date', inplace=True)
        
        print(f"✅ Loaded {len(df)} records for {symbol} from database")
        return df
        
    except Exception as e:
        print(f"❌ Error reading from database: {e}")
        return None

def run_backtest(symbol):
    """Run backtest with data from database"""
    try:
        print(f"🚀 Starting backtest for {symbol}...")
        
        # Get historical data
        df = get_historical_data_from_db(symbol)
        
        if df is None or df.empty:
            print("❌ No data available for backtesting")
            return
        
        # Create Cerebro instance
        cerebro = bt.Cerebro()
        
        # Add strategy
        cerebro.addstrategy(StrategyHistory, symbol=symbol)
        
        # Create data feed from pandas DataFrame
        data = bt.feeds.PandasData(
            dataname=df,
            datetime=None,  # Use DataFrame index
            open='open',
            high='high', 
            low='low',
            close='close',
            volume='volume',
            openinterest=-1
        )
        
        cerebro.adddata(data, name=symbol)
        
        # Set initial cash
        cerebro.broker.setcash(100000.0)
        
        print(f"💰 Initial Portfolio Value: ${cerebro.broker.getvalue():,.2f}")
        
        # Run backtest
        result = cerebro.run()
        
        final_value = cerebro.broker.getvalue()
        print(f"💰 Final Portfolio Value: ${final_value:,.2f}")
        print(f"📈 Total Return: ${final_value - 100000:,.2f} ({((final_value/100000)-1)*100:.2f}%)")
        
    except Exception as e:
        print(f"❌ Error running backtest: {e}")
        import traceback
        traceback.print_exc()

if __name__ == '__main__':
    # Allow symbol to be passed as command line argument
    symbol = sys.argv[1] if len(sys.argv) > 1 else 'SPY'
    run_backtest(symbol)