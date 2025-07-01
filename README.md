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

### Home & Navigation
- [`GET /`](#home) - Home page with navigation and system status

### Authentication
- [`GET /login`](#authentication) - Initiates Google OAuth2 login flow
- [`GET /callback`](#authentication) - OAuth2 callback handler for processing authentication

### Email Processing
- [`GET /batchget`](#email-processing) - Fetches and processes emails from Gmail with specific labels
- [`GET /fixdate`](#email-processing) - Fixes and standardizes email dates in the database

### Stock Data Management
- [`GET /api/stock/{symbol}`](#stock-data) - Fetches real-time data for a specific stock symbol
- [`GET /api/stock/historical/{symbol}`](#stock-data) - Retrieves historical data for a specific stock symbol
- [`GET /api/stock/historical/fill`](#stock-data) - Fills historical data for all S&P 500 stocks
- [`GET /sp500`](#stock-data) - Fetches current stock data for all S&P 500 companies
- [`GET /api/sp500/update`](#stock-data) - Updates S&P 500 constituents list from Wikipedia
- [`GET /api/sp500/list`](#stock-data) - Returns the current list of S&P 500 stocks

### Portfolio Management
- [`GET /api/portfolio/backtest`](#portfolio-management) - Runs backtesting simulation on the portfolio
- [`GET /api/portfolio`](#portfolio-management) - Returns current portfolio holdings and values
- [`GET /api/portfolio/performance`](#portfolio-management) - Retrieves historical portfolio performance metrics

### Database Management
- [`GET /api/tables`](#database-management) - Lists all available database tables
- [`GET /api/tables/{table}`](#database-management) - Retrieves paginated data from a specific table
  - Query parameters: `page` (default: 1), `pageSize` (default: 100)

### System Status & Monitoring
- [`GET /health`](#system-monitoring) - Health check endpoint for monitoring
- [`GET /metrics`](#system-monitoring) - Returns system metrics and statistics

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

## Detailed API Documentation

### Home
The home page provides navigation to all major features and system status information.

**Endpoint:** `GET /`  
**Description:** Serves the main HTML interface with links to authentication, email processing, and stock data features.  
**Response:** HTML page with navigation buttons and system information.

### Authentication
OAuth2 integration with Google for secure email access.

**Login Endpoint:** `GET /login`  
**Description:** Initiates Google OAuth2 authentication flow.  
**Response:** JSON with authorization URL for Google login.

**Callback Endpoint:** `GET /callback`  
**Description:** Handles OAuth2 callback from Google authentication.  
**Parameters:**
- `state` - OAuth2 state parameter for security
- `code` - Authorization code from Google

### Email Processing
Gmail integration for fetching and processing backtest-related emails.

**Batch Get Endpoint:** `GET /batchget`  
**Description:** Fetches emails from Gmail with specified labels and stores them in the database.  
**Parameters:**
- `label` - Gmail label to filter emails (optional, defaults to configured label)

**Fix Date Endpoint:** `GET /fixdate`  
**Description:** Standardizes and fixes email date formats in the database.  
**Response:** JSON with processing results and number of emails updated.

### Stock Data
Comprehensive stock data management for S&P 500 companies.

**Individual Stock Data:** `GET /api/stock/{symbol}`  
**Description:** Retrieves real-time stock data for a specific symbol.  
**Parameters:**
- `{symbol}` - Stock ticker symbol (e.g., AAPL, GOOGL)
**Response:** JSON with current price, volume, market cap, and other metrics.

**Historical Stock Data:** `GET /api/stock/historical/{symbol}`  
**Description:** Fetches historical price data for a specific stock.  
**Parameters:**
- `{symbol}` - Stock ticker symbol
- Query parameters for date range and period

**Fill Historical Data:** `GET /api/stock/historical/fill`  
**Description:** Bulk operation to fill historical data for all S&P 500 stocks.  
**Response:** Streaming progress updates during data fetching process.

**S&P 500 Data Fetch:** `GET /sp500`  
**Description:** Fetches current stock data for all S&P 500 companies using concurrent processing.  
**Response:** JSON with operation results and processing statistics.

**Update S&P 500 List:** `GET /api/sp500/update`  
**Description:** Updates the S&P 500 constituents list from Wikipedia.  
**Response:** JSON with updated company list and changes.

**List S&P 500 Stocks:** `GET /api/sp500/list`  
**Description:** Returns the current list of S&P 500 companies.  
**Response:** JSON array of stock symbols and company names.

### Portfolio Management
Backtesting and portfolio analysis functionality.

**Portfolio Backtest:** `GET /api/portfolio/backtest`  
**Description:** Runs a comprehensive backtest simulation on the portfolio using historical data.  
**Response:** Streaming results with performance metrics, trade history, and analysis.

**Portfolio Holdings:** `GET /api/portfolio`  
**Description:** Returns current portfolio holdings and calculated values.  
**Response:** JSON with positions, current values, and allocation percentages.

**Portfolio Performance:** `GET /api/portfolio/performance`  
**Description:** Retrieves historical performance metrics and analytics.  
**Response:** JSON with returns, volatility, drawdowns, and benchmark comparisons.

### Database Management
Direct database access and table browsing capabilities.

**List Tables:** `GET /api/tables`  
**Description:** Returns a list of all available database tables.  
**Response:** JSON array of table names.

**Table Data:** `GET /api/tables/{table}`  
**Description:** Retrieves paginated data from a specific database table.  
**Parameters:**
- `{table}` - Table name
- `page` - Page number (default: 1)
- `pageSize` - Records per page (default: 100)
**Response:** JSON array of table records with pagination.

### System Monitoring
Health checks and system metrics for monitoring application status.

**Health Check:** `GET /health`  
**Description:** Basic health check endpoint for load balancers and monitoring systems.  
**Response:** JSON with system status and basic metrics.

**System Metrics:** `GET /metrics`  
**Description:** Detailed system metrics including database connections, processing statistics, and performance data.  
**Response:** JSON with comprehensive system metrics.
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