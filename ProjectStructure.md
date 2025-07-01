# Project Structure - backtest-sell-limit

## Overview
The **backtest-sell-limit** project is a hybrid Python/Go application designed for backtesting stock portfolio strategies with a focus on sell limit orders. It combines a sophisticated backtesting engine with a web API for data management and portfolio analysis, targeting S&P 500 stocks.

## Project Architecture

```
backtest-sell-limit/
├── cmd/web/                    # Go web application
│   ├── main.go                 # Main HTTP server with stock data APIs
│   └── config.go               # Configuration management system
├── pkg/types/                  # Go type definitions
│   └── types.go                # Data structures for stocks and config
├── strategies/                 # Python backtesting strategies
│   ├── __init__.py             # Module initialization
│   ├── bracket_strategy.py     # Bracket order implementation
│   ├── portfolio_strategy.py   # Main portfolio strategy (20% sell limit)
│   └── sma_strategy.py         # SMA-based strategy with database logging
├── sql/                        # Database schema and queries
├── tests/                      # Test suite
├── config/                     # Configuration files
├── html/                       # Web interface templates
├── archive/                    # Archived database files and backups
└── tmp/                        # Temporary build artifacts
```

## Core Components

### 1. Go Web Application (`cmd/web/`)

**main.go** - Primary HTTP server providing:
- **Stock Data API**: Real-time data from Yahoo Finance
- **Historical Data Management**: OHLCV data storage and retrieval
- **S&P 500 Integration**: Constituent list parsing and management
- **Database Browser**: REST endpoints for SQLite table exploration
- **Portfolio Backtesting**: HTTP triggers for Python backtesting

**Key Features:**
- Concurrent data fetching with goroutines
- SQLite with WAL mode and connection pooling (25 max connections)
- Rate limiting and retry logic for external APIs
- Streaming log output for long operations

**config.go** - Configuration management:
- Multi-path config file search
- Command-line override support (`--config` flag)
- Environment-aware (DEV, Prod, Local, Hosted)

### 2. Go Type System (`pkg/types/`)

**types.go** - Core data structures:
- `Config`: Application configuration
- `StockData`: Real-time stock information
- `HistoricalData`: Time series OHLCV data
- `SP500Stock`: S&P 500 constituent information

### 3. Python Backtesting Engine

**portfolio_backtest.py** - Main backtesting orchestrator:
- Backtrader framework integration
- Database-driven historical data sourcing
- Multi-stock S&P 500 portfolio processing
- Comprehensive logging with custom DatabaseLogHandler
- Performance tracking and metrics calculation

**db_manager.py** - Database management:
- Singleton SQLite connection management
- Automatic lock detection and cleanup
- Transaction management with commit/rollback
- SQL file loading system
- Robust error recovery mechanisms

**database_log_handler.py** - Custom logging:
- Database-backed log storage
- Structured logging with metadata
- Trade and order tracking integration

### 4. Strategy Framework (`strategies/`)

**portfolio_strategy.py** - Main strategy:
- Bracket orders with 20% sell limits
- Comprehensive trade tracking
- PnL calculation and performance metrics
- Database integration for strategy history

**bracket_strategy.py** - Bracket order implementation:
- Market buy orders for all stocks
- Automatic 20% take profit orders
- No stop loss (upside-focused)

**sma_strategy.py** - Technical analysis strategy:
- 20-period Simple Moving Average
- Price vs SMA crossover signals
- Database logging for all trades

## Database Schema

### Stock Data Tables
- **`sp500_list_2025_jun`**: S&P 500 constituents (ticker, security_name)
- **`stock_data`**: Real-time stock information (price, volume, market cap, etc.)
- **`stock_historical_data`**: OHLCV time series data with Unix timestamps

### Backtesting Tables
- **`backtest_strategies`**: Strategy performance summary (returns, date ranges)
- **`backtest_daily_values`**: Daily portfolio value progression
- **`strategy_history`**: Trade-level details with PnL tracking
- **`logs`**: Comprehensive system and trade logging

### Portfolio Tables
- **`portfolio`**: Current portfolio holdings
- **`portfolio_details`**: Detailed position information
- **`portfolio_summary`**: Portfolio-wide metrics
- **`portfolio_by_sector`**: Sector allocation analysis

## Data Flow

### 1. Data Ingestion
```
Yahoo Finance API → Go HTTP Client → SQLite Database → Python Backtest Engine
```

### 2. Backtesting Process
```
SQLite Historical Data → Backtrader Strategy → Trade Execution → 
Database Logging → Performance Metrics → Results Storage
```

### 3. Web API
```
HTTP Request → Go Router → Database Query → JSON Response
```

## Key Functionality

### Backtesting Features
- **Multi-asset Processing**: Entire S&P 500 portfolio backtesting
- **Bracket Order Strategy**: 20% sell limit implementation
- **Performance Analytics**: Return and risk metrics
- **Trade-level Tracking**: Detailed execution history

### Data Management
- **Real-time Feeds**: Live Yahoo Finance data
- **Historical Storage**: Multi-year price history
- **S&P 500 Tracking**: Automated constituent updates
- **Database Browser**: Web interface for data exploration

### System Features
- **Concurrent Processing**: Parallel data fetching
- **Error Resilience**: Automatic retry and recovery
- **Lock Management**: SQLite lock cleanup system
- **Comprehensive Logging**: Multi-level database logging

## Build & Run

### Go Application
```bash
# Development with hot reload
air

# Manual build
go build -o tmp/main cmd/web/main.go cmd/web/config.go
./tmp/main --config config/config.json
```

### Python Backtesting
```bash
# Run with lock cleanup
./run_backtest.sh

# Direct execution
python3 portfolio_backtest.py
```

### Database Management
```bash
# Clear locks
./kill_db_locks.sh

# Custom database
./kill_db_locks.sh custom_database.db
```

## Configuration

### Go Config (`config/config.json`)
```json
{
  "ENV": "Local",
  "TopLevelDir": "/path/to/project/",
  "Port": "8081",
  "BacktestDB": "backtest_sell_limits.db"
}
```

### Python Dependencies
- Core: `backtrader`, `pandas`, `sqlite3`
- Development: `pytest`, `pytest-cov`

## API Endpoints

### Stock Data
- `GET /api/stock/{symbol}` - Current stock data
- `GET /api/stock/historical/{symbol}` - Historical data
- `GET /api/stock/historical/fill` - Fill historical data for all stocks

### Database Browser
- `GET /api/tables` - List all database tables
- `GET /api/tables/{table}` - Browse table contents with pagination

### Portfolio
- `GET /api/portfolio/backtest` - Run portfolio backtest

### Documentation
- `GET /readme` - Rendered project documentation

## Recent Refactoring Improvements

1. **Configuration System**: Centralized config.json management
2. **Port Configuration**: Moved from hardcoded to configurable
3. **Database Browsing**: Enhanced SQLite exploration capabilities
4. **Code Organization**: Streamlined portfolio_backtest.py
5. **Lock Management**: Improved database lock handling
6. **SQL Separation**: Moved queries to dedicated files

## Current State

The project is production-ready with:
- **Clean Architecture**: Clear Go/Python separation
- **Robust Database Design**: Well-normalized schema
- **Error Resilience**: Comprehensive lock and error handling
- **Extensible Framework**: Easy strategy addition
- **Production Features**: Config management, logging, monitoring

The recent refactoring has resulted in a maintainable, modular codebase with improved separation of concerns and reduced complexity.