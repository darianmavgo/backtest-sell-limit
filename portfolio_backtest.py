import sqlite3
import backtrader as bt
import pandas as pd
from datetime import datetime, timedelta
import warnings
import logging



# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

DB_FILE = "backtest_sell_limits.db"
TICKER_TABLE = "sp500_list_2025_jun"
START_DATE = "2024-05-01"
END_DATE = "2025-06-26"
INITIAL_CASH = 1_000_000.0 # A large amount to ensure any trade can be made

def get_sp500_tickers():
    """Fetches the list of S&P 500 tickers from the SQLite database."""
    try:
        con = sqlite3.connect(DB_FILE)
        query = f"SELECT ticker FROM {TICKER_TABLE} WHERE is_active = 1"
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
        query = """
        SELECT 
            datetime(date, 'unixepoch') as date,
            open,
            high,
            low,
            close,
            adj_close,
            volume
        FROM stock_historical_data
        WHERE symbol = ?
        AND date >= strftime('%s', ?)
        AND date <= strftime('%s', ?)
        ORDER BY date
        """
        df = pd.read_sql_query(
            query, 
            conn, 
            params=[symbol, start_date, end_date],
            parse_dates={'date': '%Y-%m-%d %H:%M:%S'}
        )
        df.set_index('date', inplace=True)
        return df
    except Exception as e:
        logger.error(f"Error fetching historical data for {symbol}: {e}")
        return None
    finally:
        conn.close()

class PortfolioStrategy(bt.Strategy):
    """A simple buy and hold strategy for the portfolio"""
    
    def __init__(self):
        self.orders = {}  # Keep track of orders
        self.initial_cash = self.broker.getvalue()
        self.stocks_bought = False
        
    def next(self):
        # Buy stocks on the first day
        if not self.stocks_bought:
            for data in self.datas:
                # Calculate the number of shares we can buy
                size = 1  # Buy 1 share of each stock
                
                # Create a buy order
                self.orders[data._name] = self.buy(data=data, size=size)
                logger.info(f'Buying {size} shares of {data._name} at {data.close[0]}')
            
            self.stocks_bought = True
        
        # Log daily portfolio value
        portfolio_value = self.broker.getvalue()
        logger.info(f'Date: {self.data0.datetime.date()} Portfolio Value: ${portfolio_value:.2f}')

def get_portfolio_data():
    """Get portfolio data with current prices from SQLite database"""
    conn = sqlite3.connect('backtest_sell_limits.db')
    try:
        query = """
        SELECT 
            p.symbol,
            p.shares,
            p.purchase_price,
            p.purchase_date,
            s.price as current_price,
            s.open_price as open,
            s.high,
            s.low,
            s.price as close,
            s.volume,
            s.last_updated
        FROM portfolio p
        JOIN stock_data s ON p.symbol = s.symbol
        WHERE p.shares > 0
        """
        df = pd.read_sql_query(query, conn)
        return df
    finally:
        conn.close()

def calculate_portfolio_value():
    """Calculate current portfolio value"""
    df = get_portfolio_data()
    
    # Calculate total value
    total_value = (df['shares'] * df['current_price']).sum()
    total_cost = (df['shares'] * df['purchase_price']).sum()
    total_return = ((total_value - total_cost) / total_cost) * 100
    
    logger.info("\n=== Portfolio Summary ===")
    logger.info(f"Number of Stocks: {len(df)}")
    logger.info(f"Total Cost: ${total_cost:,.2f}")
    logger.info(f"Current Value: ${total_value:,.2f}")
    logger.info(f"Total Return: {total_return:.2f}%")
    
    # Show top 5 gainers and losers
    df['return'] = ((df['current_price'] - df['purchase_price']) / df['purchase_price']) * 100
    
    logger.info("\nTop 5 Gainers:")
    top_gainers = df.nlargest(5, 'return')
    for _, row in top_gainers.iterrows():
        logger.info(f"{row['symbol']}: {row['return']:.2f}%")
    
    logger.info("\nTop 5 Losers:")
    top_losers = df.nsmallest(5, 'return')
    for _, row in top_losers.iterrows():
        logger.info(f"{row['symbol']}: {row['return']:.2f}%")

def run_backtest():
    """Run backtest using historical data from SQLite database"""
    # Create a cerebro entity
    cerebro = bt.Cerebro()
    
    # Set initial cash
    cerebro.broker.setcash(INITIAL_CASH)
    
    # Get portfolio symbols
    symbols = get_sp500_tickers()
    logger.info(f"Found {len(symbols)} symbols in portfolio")
    
    # Add data feeds for each symbol
    for symbol in symbols:
        logger.info(f"Fetching historical data for {symbol}...")
        df = get_historical_data(symbol, START_DATE, END_DATE)
        if df is not None and not df.empty:
            # Create data feed
            data = bt.feeds.PandasData(
                dataname=df,
                fromdate=datetime.strptime(START_DATE, '%Y-%m-%d'),
                todate=datetime.strptime(END_DATE, '%Y-%m-%d')
            )
            cerebro.adddata(data, name=symbol)
            logger.info(f"Added data feed for {symbol}")
        else:
            logger.warning(f"No data available for {symbol}")
    
    # Add the strategy
    cerebro.addstrategy(PortfolioStrategy)
    
    # Run the backtest
    logger.info("\nStarting Portfolio Backtest...")
    initial_value = cerebro.broker.getvalue()
    results = cerebro.run()
    final_value = cerebro.broker.getvalue()
    
    # Calculate and display results
    total_return = ((final_value - initial_value) / initial_value) * 100
    logger.info("\n=== Backtest Results ===")
    logger.info(f"Start Date: {START_DATE}")
    logger.info(f"End Date: {END_DATE}")
    logger.info(f"Initial Portfolio Value: ${initial_value:,.2f}")
    logger.info(f"Final Portfolio Value: ${final_value:,.2f}")
    logger.info(f"Total Return: {total_return:.2f}%")

if __name__ == "__main__":
    calculate_portfolio_value()

    sp500_tickers = get_sp500_tickers()
    if sp500_tickers:
        run_backtest()
    else:
        print("No tickers found. Exiting.") 