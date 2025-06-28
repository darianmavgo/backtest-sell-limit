# Backtest Sell Limit

A Python and Go application for backtesting stock portfolio strategies with a focus on sell limit orders. The application includes both a backtesting engine and a web API for data management and portfolio analysis.

## Features

### Portfolio Backtesting
- Backtest various portfolio strategies
- Support for bracket order strategies
- Historical data analysis
- Performance metrics calculation
- SQLite database for storing backtest results

### Database Management
- Centralized SQLite connection management
- Transaction handling and error recovery
- Efficient query execution
- Comprehensive logging system

### Strategy Implementation
- Modular strategy framework
- Bracket order strategy implementation
- Portfolio strategy implementation
- Easy to extend with new strategies

### Gmail Integration
- Login with Google OAuth2
- Fetch emails with specific labels
- Store email content in SQLite database
- Fix email dates from various formats

### S&P 500 Stock Data
- Fetch real-time stock data for all S&P 500 companies
- Concurrent processing using goroutines for fast data retrieval
- Store comprehensive stock information including:
  - Current price and price changes
  - Volume and market cap
  - Daily high/low and 52-week high/low
  - Previous close and opening prices
- SQLite database storage with proper indexing

## API Endpoints

### Authentication
- `GET /login` - Initiates Google OAuth2 login flow
- `GET /callback` - OAuth2 callback handler for processing authentication

### Email Processing
- `GET /batchget` - Fetches and processes emails from Gmail with specific labels
- `GET /fixdate` - Fixes and standardizes email dates in the database

### S&P 500 Data
- `GET /api/sp500/update` - Fetches current S&P 500 constituents from Wikipedia and stores them in the database
- `GET /api/sp500/list` - Returns the current list of S&P 500 stocks
- `GET /api/stock/{symbol}` - Fetches real-time data for a specific stock
- `GET /api/stock/historical/{symbol}` - Retrieves historical data for a specific stock
- `GET /api/stock/historical/fill` - Fills the historical_data table with data for all S&P 500 stocks

### Portfolio Management
- `GET /api/portfolio` - Returns the current portfolio holdings and values
- `GET /api/portfolio/backtest` - Runs backtesting simulation on the portfolio
- `GET /api/portfolio/performance` - Retrieves historical portfolio performance metrics

### System Status
- `GET /` - Home page with navigation and system status
- `GET /health` - Health check endpoint for monitoring
- `GET /metrics` - Returns system metrics and statistics

## Project Structure

```
backtest-sell-limit/
├── strategies/           # Strategy implementations
│   ├── bracket_strategy.py
│   └── portfolio_strategy.py
├── tests/               # Test suite
│   ├── conftest.py
│   └── test_portfolio_backtest.py
├── sql/                 # SQL queries
│   ├── create_tables/
│   └── queries/
├── portfolio_backtest.py # Main backtesting logic
├── db_manager.py        # Database connection management
└── database_log_handler.py # Logging system
```

## Database Schema

### Stock Data Tables
1. `sp500_list_2025_jun`
   - `ticker` (TEXT PRIMARY KEY)
   - `security_name` (TEXT)

2. `stock_data`
   - `symbol` (TEXT PRIMARY KEY)
   - `company_name` (TEXT)
   - `price` (REAL)
   - `change_amount` (REAL)
   - `change_percent` (REAL)
   - `volume` (INTEGER)
   - `market_cap` (INTEGER)
   - `previous_close` (REAL)
   - `open_price` (REAL)
   - `high` (REAL)
   - `low` (REAL)
   - `fifty_two_week_high` (REAL)
   - `fifty_two_week_low` (REAL)
   - `last_updated` (TIMESTAMP)

3. `stock_historical_data`
   - `symbol` (TEXT)
   - `date` (TIMESTAMP)
   - `open` (REAL)
   - `high` (REAL)
   - `low` (REAL)
   - `close` (REAL)
   - `adj_close` (REAL)
   - `volume` (INTEGER)

### Backtest Tables
1. `backtest_strategies`
   - Strategy configuration and parameters
   - Execution timestamps
   - Performance metrics

2. `backtest_daily_values`
   - Daily portfolio values
   - Asset allocations
   - Transaction records

3. `logs`
   - Detailed execution logs
   - Error tracking
   - Performance monitoring

## Development

### Requirements
- Python 3.8+
- SQLite3
- Required Python packages in `requirements.txt`
- Development dependencies in `requirements-dev.txt`
- Go 1.x for API components

### Setup
1. Clone the repository
2. Install dependencies:
```bash
pip install -r requirements.txt
pip install -r requirements-dev.txt
```

### Running Tests
```bash
# Run Python tests
pytest tests/

# Run Go tests
go test -v
```

### Running Backtests
```bash
./run_backtest.sh
```

### Hot Reload (API Server)
The Go API server supports hot reloading using `air`. To start the server with hot reload:

```bash
air
```

This will automatically rebuild and restart the server when code changes are detected.

## Architecture

### Database Connection Management
- Singleton SQLiteConnectionManager class
- Automatic connection recovery
- Transaction management
- Query execution with error handling

### Logging System
- Custom DatabaseLogHandler
- Structured logging to SQLite
- Query-based log analysis
- Performance monitoring

### Strategy Framework
- Abstract base classes for strategies
- Standardized interface for new strategies
- Built-in performance metrics
- Historical data integration

## Performance Optimization

### Backtesting Engine
- Efficient database queries
- Connection pooling
- Transaction batching
- Indexed database tables

### API Performance
- 20 concurrent goroutines for parallel API calls
- Worker pool pattern for efficient resource management
- Timeout handling for API requests
- Error handling and logging for failed requests

## Dependencies
- SQLite3 for database storage
- Google APIs for Gmail integration
- Yahoo Finance API for stock data
- Standard Go libraries for HTTP and concurrency
- Python packages listed in requirements.txt

## Configuration
- Update the `credentialsFile` constant in `main.go` with your Google OAuth2 credentials file path
- Configure database settings in `db_manager.py`
- Set up strategy parameters in the respective strategy files

## Contributing
1. Fork the repository
2. Create a feature branch
3. Implement changes with tests
4. Submit a pull request

## License
[Add your license here] 