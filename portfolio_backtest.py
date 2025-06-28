import sqlite3
import backtrader as bt
import pandas as pd
from datetime import datetime, timedelta, date
import warnings
import logging
import sys
import os

def load_sql_query(filename):
    """Load SQL query from file in sql/ directory"""
    sql_path = os.path.join("sql", filename)
    with open(sql_path, 'r') as f:
        return f.read().strip()

class BracketStrategy(bt.Strategy):
    """
    This strategy implements a bracket order that buys 1 share of every
    available stock at the market price. Once a buy order is executed,
    it automatically places a limit sell order to take profit at 20%
    above the execution price. It does not include a stop-loss leg.
    """

    def log(self, txt, dt=None):
        """ Logging function for this strategy """
        dt = dt or self.datas[0].datetime.date(0)
        print(f'{dt.isoformat()} - {txt}')

    def __init__(self):
        """
        Conception: __init__ is called when the strategy is created.
        We can initialize attributes here. For this strategy, no
        special indicators or attributes are needed initially.
        """
        self.log('Strategy Initialized')
        # This dictionary will keep track of the take-profit sell order
        # associated with each stock's buy order.
        self.profit_takers = {}


    def notify_order(self, order):
        """
        This method is called for any status change in an order.
        It is the heart of our bracket order logic.
        """
        # 1. If order is submitted/accepted, do nothing
        if order.status in [order.Submitted, order.Accepted]:
            return

        # 2. Check if an order has been completed
        if order.status in [order.Completed]:
            if order.isbuy():
                # This was a buy order that has been executed
                self.log(
                    f'BUY EXECUTED for {order.data._name}: '
                    f'Price: {order.executed.price:.2f}, '
                    f'Size: {order.executed.size}, '
                    f'Cost: {order.executed.value:.2f}'
                )

                # --- BRACKET LOGIC ---
                # Calculate the take profit price (20% above execution price)
                profit_price = order.executed.price * 1.20

                # Create the corresponding limit sell order to take profit
                self.log(
                    f'CREATING TAKE PROFIT SELL for {order.data._name} '
                    f'at Price: {profit_price:.2f}'
                )
                profit_order = self.sell(
                    data=order.data,
                    size=order.executed.size,
                    price=profit_price,
                    exectype=bt.Order.Limit
                )
                # Store the reference to the profit-taking order
                self.profit_takers[order.data._name] = profit_order

            elif order.issell():
                # This was a sell order that has been executed
                self.log(
                    f'SELL EXECUTED (Take Profit) for {order.data._name}: '
                    f'Price: {order.executed.price:.2f}, '
                    f'Size: {order.executed.size}, '
                    f'Value: {order.executed.value:.2f}'
                )
                # Since the sell order (our profit taker) executed,
                # we can remove its reference.
                if order.data._name in self.profit_takers:
                    del self.profit_takers[order.data._name]


        # 3. Handle other order statuses like Canceled, Margin, or Rejected
        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            self.log(f'Order for {order.data._name} was Canceled/Margin/Rejected')


    def start(self):
        """
        Birth: This method is called once the minimum period of all
        indicators is met. Since we have no indicators, it's called
        at the very beginning. We use it to place our initial buy orders.
        """
        self.log('Strategy Starting. Placing initial buy orders.')
        for d in self.datas:
            # Place a market order to buy 1 share of each stock
            self.buy(data=d, size=1)

    def stop(self):
        """
        Death: This method is called at the end of the backtest.
        A good place to print final results.
        """
        self.log(f'Strategy Stopping. Final Portfolio Value: {self.broker.getvalue():.2f}')

class DatabaseLogHandler(logging.Handler):
    """A custom logging handler that writes logs to an SQLite database."""
    def __init__(self, db_file, timeout=5.0):
        super().__init__()
        self.db_file = db_file
        self.timeout = timeout
        self.create_table()

    def create_table(self):
        """Create the logs table if it doesn't exist."""
        with sqlite3.connect(self.db_file, timeout=self.timeout) as conn:
            conn.execute(load_sql_query("create_logs_table.sql"))
            conn.commit()

    def emit(self, record):
        """Emit a log record."""
        try:
            with sqlite3.connect(self.db_file, timeout=self.timeout) as conn:
                message = self.format(record)
                conn.execute(load_sql_query("insert_log_entry.sql"), (
                    datetime.fromtimestamp(record.created).strftime('%Y-%m-%d %H:%M:%S.%f'),
                    record.name,
                    record.levelname,
                    message,
                    getattr(record, 'symbol', None),
                    getattr(record, 'order_type', None),
                    getattr(record, 'status', None),
                    getattr(record, 'price', None),
                    getattr(record, 'size', None),
                    getattr(record, 'order_ref', None),
                    getattr(record, 'parent_ref', None)
                ))
                conn.commit()
        except Exception:
            import traceback
            traceback.print_exc(file=sys.stderr)

    def __del__(self):
        """The connection is no longer stored on the instance."""
        pass

def setup_logging():
    """Configure logging to use the DatabaseLogHandler."""
    logger = logging.getLogger()
    logger.setLevel(logging.INFO)
    # Clear any existing handlers
    logger.handlers = []
    db_handler = DatabaseLogHandler(DB_FILE)
    logger.addHandler(db_handler)
    return logger

# Global settings
DB_FILE = "backtest_sell_limits.db"
TICKER_TABLE = "sp500_list_2025_jun"
START_DATE = "2024-05-01"
END_DATE = "2025-06-26"
INITIAL_CASH = 1_000_000.0 # A large amount to ensure any trade can be made

def get_sp500_tickers():
    """Fetches the list of S&P 500 tickers from the SQLite database."""
    try:
        con = sqlite3.connect(DB_FILE)
        query_template = load_sql_query("get_sp500_tickers.sql")
        query = query_template.format(ticker_table=TICKER_TABLE)
        df = pd.read_sql_query(query, con)
        return df['ticker'].tolist()
    except Exception as e:
        print(f"Error fetching tickers: {e}")
        return []
    finally:
        if 'con' in locals() and con:
            con.close()

def get_historical_data(symbol, start_date, end_date):
    """Get historical data for a symbol from SQLite database"""
    try:
        conn = sqlite3.connect(DB_FILE)
        query = load_sql_query("get_historical_data.sql")
        df = pd.read_sql_query(
            query, 
            conn, 
            params=[symbol, start_date, end_date],
            parse_dates={'date': '%Y-%m-%d %H:%M:%S'}
        )
        df.set_index('date', inplace=True)
        return df
    except Exception as e:
        logging.error(f"Error fetching historical data for {symbol}: {e}")
        return None
    finally:
        conn.close()

def save_backtest_results(strategy_name, daily_values, initial_value, final_value, total_return):
    """Save backtest results to SQLite database"""
    conn = sqlite3.connect(DB_FILE)
    try:
        logging.info(f"Saving backtest results for strategy: {strategy_name}")
        logging.info(f"Initial value: ${initial_value:,.2f}")
        logging.info(f"Final value: ${final_value:,.2f}")
        logging.info(f"Total return: {total_return:.2f}%")
        logging.info(f"Number of daily values: {len(daily_values)}")
        
        # Create tables if they don't exist
        conn.execute(load_sql_query("create_backtest_strategies_table.sql"))
        conn.execute(load_sql_query("create_backtest_daily_values_table.sql"))
        
        # NOTE: backtest_order_history table is no longer created or used.
        # All order history is now in the 'logs' table.

        # Save strategy performance
        conn.execute(load_sql_query("insert_backtest_strategy.sql"), 
                    (strategy_name, START_DATE, END_DATE, initial_value, final_value, total_return))
        logging.info("Saved strategy performance")

        # Save daily values
        for date, value in daily_values:
            conn.execute(load_sql_query("insert_backtest_daily_value.sql"), 
                        (strategy_name, date.strftime('%Y-%m-%d'), value))
        logging.info("Saved daily values")

        # NOTE: Order history is no longer saved here. It's logged directly.

        conn.commit()
        logging.info("Database changes committed")
    except Exception as e:
        logging.error(f"Error saving backtest results: {e}")
        conn.rollback()
    finally:
        conn.close()

class PortfolioStrategy(bt.Strategy):
    """A strategy that buys stocks and sets sell limits at 20% above purchase price"""
    
    def __init__(self):
        self.orders = {}  # Keep track of buy orders
        self.sell_orders = {}  # Keep track of sell limit orders
        self.initial_cash = self.broker.getvalue()
        self.stocks_bought = False
        self.purchase_prices = {}  # Keep track of purchase prices
        self.daily_values = []  # Track daily portfolio values
        
    def next(self):
        # Track daily portfolio value
        self.daily_values.append((self.datetime.date(), self.broker.getvalue()))
        
        # Buy stocks on the first day
        if not self.stocks_bought:
            for data in self.datas:
                # Check if we have data
                if len(data) > 0:
                    # Buy 1 share of each stock using bracket order
                    size = 1
                    
                    # Calculate target price (20% above current price)
                    current_price = data.close[0]
                    target_price = current_price * 1.20
                    
                    # Create market buy order first
                    buy_order = self.buy(data=data, size=size)
                    
                    # Create sell limit order at target price
                    sell_order = self.sell(data=data, 
                                         size=size, 
                                         exectype=bt.Order.Limit,
                                         price=target_price,
                                         parent=buy_order,
                                         transmit=True)
                    
                    self.purchase_prices[data._name] = current_price
                    logging.info(
                        f"Placed bracket order for {data._name}", 
                        extra={'symbol': data._name, 'price': current_price}
                    )
                else:
                    logging.warning(f"No data available for {data._name}", extra={'symbol': data._name})
            
            self.stocks_bought = True
    
    def notify_order(self, order):
        """Log order notifications to the database."""
        extra_data = {
            'symbol': order.data._name,
            'order_type': order.ordtypename(),
            'status': order.getstatusname(),
            'order_ref': order.ref,
            'parent_ref': order.parent.ref if order.parent else None
        }
        
        message = f"Order Notification: {order.data._name} - {order.ordtypename()} - {order.getstatusname()}"

        if order.status == order.Completed:
            extra_data['price'] = order.executed.price
            extra_data['size'] = order.executed.size
            message = f"Order Completed: {order.data._name} - {order.ordtypename()} at {order.executed.price}"
            logging.info(message, extra=extra_data)
        elif order.status in [order.Canceled, order.Margin, order.Rejected]:
            logging.warning(message, extra=extra_data)
        else: # Submitted, Accepted, Partial
            logging.info(message, extra=extra_data)

    def stop(self):
        # Calculate return
        final_value = self.broker.getvalue()
        total_return = (final_value - self.initial_cash) / self.initial_cash * 100
        
        # Save results
        save_backtest_results(
            "PortfolioStrategy_20_percent_sell_limit",
            self.daily_values,
            self.initial_cash,
            final_value,
            total_return
        )

def get_portfolio_data():
    """Get current portfolio data from SQLite database"""
    try:
        con = sqlite3.connect(DB_FILE)
        query = load_sql_query("get_portfolio_data.sql")
        df = pd.read_sql_query(query, con)
        return df
    except Exception as e:
        print(f"Error fetching portfolio data: {e}")
        return pd.DataFrame()
    finally:
        con.close()

def calculate_portfolio_value():
    """Calculate current portfolio value"""
    df = get_portfolio_data()
    if df.empty:
        return 0
    
    total_value = (df['shares'] * df['current_price']).sum()
    return total_value

def clear_backtest_history(strategy_name):
    """Clears all previous backtest data for a given strategy and the logs."""
    conn = sqlite3.connect(DB_FILE)
    try:
        logging.info(f"Clearing backtest history for strategy: {strategy_name}")
        # Execute each DROP statement from the SQL file
        drop_statements = load_sql_query("clear_backtest_tables.sql").split(';')
        for statement in drop_statements:
            if statement.strip():
                conn.execute(statement)
        conn.commit()
        logging.info("Successfully cleared backtest history and logs.")
    except Exception as e:
        # Use print because logger might not be configured yet or is being cleared
        print(f"Error clearing backtest history: {e}")
    finally:
        conn.close()

def run_backtest():
    """Run the portfolio backtest"""
    strategy_name = "PortfolioStrategy_20_percent_sell_limit"
    
    # Clear old data before setting up logging
    clear_backtest_history(strategy_name)

    # Setup logging to the database
    logger = setup_logging()

    logger.info(f"Starting backtest for strategy: {strategy_name}")
    logger.info(f"Date range: {START_DATE} to {END_DATE}")

    # Set up Backtrader
    cerebro = bt.Cerebro()
    
    # Add our strategy
    cerebro.addstrategy(PortfolioStrategy)
    # cerebro.addstrategy(BracketStrategy)
    
    # Get portfolio data
    portfolio_df = get_portfolio_data()
    
    # Add data feeds for each stock in portfolio
    for symbol in get_sp500_tickers():  # Changed to use sp500_tickers instead of portfolio
        try:
            # Get historical data from SQLite
            con = sqlite3.connect(DB_FILE)
            query = load_sql_query("get_backtest_historical_data.sql")
            df = pd.read_sql_query(
                query,
                con,
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
        finally:
            con.close()
    
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