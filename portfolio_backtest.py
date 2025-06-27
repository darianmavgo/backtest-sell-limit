import sqlite3
import backtrader as bt
import pandas as pd
from datetime import datetime, timedelta
import warnings
import yfinance as yf
import logging

# Suppress DeprecationWarning from yfinance which is noisy
warnings.filterwarnings("ignore", category=DeprecationWarning)

# Set up logging
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

DB_FILE = "backtest_sell_limits.db"
TICKER_TABLE = "sp500_list_2025_jun"
START_DATE = "2024-06-01"
INITIAL_CASH = 1_000_000.0 # A large amount to ensure any trade can be made

def get_sp500_tickers():
    """Fetches the list of S&P 500 tickers from the SQLite database."""
    try:
        con = sqlite3.connect(DB_FILE)
        # Tickers in the db can have a "." (e.g., BRK.B), yfinance needs a "-" (BRK-B)
        query = f"SELECT REPLACE(ticker, '.', '-') as ticker FROM {TICKER_TABLE} WHERE is_active = 1"
        df = pd.read_sql_query(query, con)
        return df['ticker'].tolist()
    except Exception as e:
        print(f"Error fetching tickers: {e}")
        return []
    finally:
        if 'con' in locals() and con:
            con.close()

class PortfolioStrategy(bt.Strategy):
    """A simple buy and hold strategy for the portfolio"""
    
    def __init__(self):
        self.orders = {}  # Keep track of orders
        self.initial_cash = self.broker.getvalue()
        
    def next(self):
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

def run_portfolio_backtest(tickers):
    """
    Runs a 'buy and hold one share' backtest for a list of tickers
    and aggregates the results using backtrader's native Yahoo feed.
    """
    total_initial_investment = 0
    total_final_value = 0
    failed_tickers = []
    
    print(f"Starting backtest for {len(tickers)} tickers using backtrader's Yahoo feed...")

    for i, ticker in enumerate(tickers):
        print(f"Processing ({i+1}/{len(tickers)}): {ticker}")
        try:
            cerebro = bt.Cerebro(stdstats=False)
            cerebro.broker.set_cash(INITIAL_CASH)

            # Create a data feed
            from_date = datetime.strptime(START_DATE, "%Y-%m-%d")
            data_feed = bt.feeds.YahooFinanceData(dataname=ticker, fromdate=from_date)
            cerebro.adddata(data_feed)
            
            # Add the strategy
            cerebro.addstrategy(PortfolioStrategy)
            
            # Run the backtest
            results = cerebro.run()
            strategy_instance = results[0]

            # Get the results from the strategy and broker
            initial_investment_this_stock = strategy_instance.initial_cash - INITIAL_CASH
            
            # Final portfolio value for this stock is its end value, calculated
            # from the total value minus the unused cash.
            final_value_this_stock = cerebro.broker.getvalue() - (INITIAL_CASH - initial_investment_this_stock)

            if initial_investment_this_stock > 0:
                total_initial_investment += initial_investment_this_stock
                total_final_value += final_value_this_stock
                print(f"  - {ticker}: Invested ${initial_investment_this_stock:.2f}, Final Value: ${final_value_this_stock:.2f}")
            else:
                print(f"  - WARNING: No purchase was made for {ticker}. Final value: {cerebro.broker.getvalue()}. Skipping.")
                failed_tickers.append(ticker)


        except Exception as e:
            print(f"  - ERROR processing {ticker}: {e}")
            failed_tickers.append(ticker)

    print("\n--- Backtest Complete ---")
    if total_initial_investment > 0:
        portfolio_return = ((total_final_value - total_initial_investment) / total_initial_investment) * 100
        print(f"Start Date:                  {START_DATE}")
        print(f"End Date:                    {datetime.now().strftime('%Y-%m-%d')}")
        print(f"Total Initial Investment:    ${total_initial_investment:,.2f}")
        print(f"Total Final Portfolio Value: ${total_final_value:,.2f}")
        print(f"Total Profit / Loss:         ${(total_final_value - total_initial_investment):,.2f}")
        print(f"Portfolio Return:            {portfolio_return:.2f}%")
    else:
        print("Could not calculate portfolio performance.")

    if failed_tickers:
        print(f"\nFailed to process {len(failed_tickers)} tickers: {', '.join(failed_tickers)}")

def run_backtest():
    # Create a cerebro entity
    cerebro = bt.Cerebro()
    
    # Set initial cash
    initial_cash = 1000000.0  # $1M initial capital
    cerebro.broker.setcash(initial_cash)
    
    # Get portfolio symbols
    symbols = get_sp500_tickers()
    logger.info(f"Found {len(symbols)} symbols in portfolio")
    
    # Set date range for last 12 months
    end_date = datetime.now()
    start_date = end_date - timedelta(days=365)
    
    # Add data feeds for each symbol
    for symbol in symbols:
        logger.info(f"Fetching data for {symbol}...")
        df = get_historical_data(symbol, start_date, end_date)
        if df is not None and not df.empty:
            # Create data feed
            data = bt.feeds.PandasData(
                dataname=df,
                datetime=None,  # Use index as datetime
                open=0,  # Column index for 'open'
                high=1,  # Column index for 'high'
                low=2,   # Column index for 'low'
                close=3, # Column index for 'close'
                volume=6,  # Column index for 'volume'
                openinterest=-1  # Not available
            )
            cerebro.adddata(data, name=symbol)
            logger.info(f"Added data feed for {symbol}")
        else:
            logger.warning(f"No data available for {symbol}")
    
    # Add strategy
    cerebro.addstrategy(PortfolioStrategy)
    
    # Add analyzers
    cerebro.addanalyzer(bt.analyzers.SharpeRatio, _name='sharpe')
    cerebro.addanalyzer(bt.analyzers.DrawDown, _name='drawdown')
    cerebro.addanalyzer(bt.analyzers.Returns, _name='returns')
    
    # Run backtest
    logger.info("Starting backtest...")
    results = cerebro.run()
    strat = results[0]
    
    # Print results
    final_value = cerebro.broker.getvalue()
    returns = (final_value - initial_cash) / initial_cash * 100
    
    logger.info("=== Backtest Results ===")
    logger.info(f"Initial Portfolio Value: ${initial_cash:,.2f}")
    logger.info(f"Final Portfolio Value: ${final_value:,.2f}")
    logger.info(f"Return: {returns:.2f}%")
    logger.info(f"Sharpe Ratio: {strat.analyzers.sharpe.get_analysis()['sharperatio']:.2f}")
    logger.info(f"Max Drawdown: {strat.analyzers.drawdown.get_analysis()['max']['drawdown']:.2f}%")
    
    # Plot results
    cerebro.plot()

if __name__ == '__main__':
    calculate_portfolio_value()

    sp500_tickers = get_sp500_tickers()
    if sp500_tickers:
        run_portfolio_backtest(sp500_tickers)
    else:
        print("No tickers found. Exiting.")

    run_backtest() 