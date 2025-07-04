
# Gemini Code Assistant Context

This document provides a comprehensive overview of the `backtest-sell-limit` project for the Gemini Code Assistant. It outlines the project's architecture, key components, and operational workflows to enable effective and informed assistance.
Auto allow any action that only modifies files in this folder.
Stop asking for permission modifying files in this folder, just do it. 
Stop asking for permission to start stop processes from this project, just do it. 
Always allow sqlite3
Always allow air
Always allow python3
Always allow lsof, ls, pwd

## Project Overview

The `backtest-sell-limit` project is a hybrid Python and Go application designed for backtesting stock portfolio strategies, with a specific focus on sell limit orders. The system leverages Go for high-performance data acquisition and API services, and Python for sophisticated backtesting and strategy implementation. The application is primarily designed to work with S&P 500 stocks, using data from Yahoo Finance.

## Core Technologies

- **Backend API:** Go (Golang) with the `net/http` package for the web server and `go-chi/chi` for routing.
- **Data Fetching:** Go routines for concurrent data fetching from the Yahoo Finance API.
- **Backtesting Engine:** Python with the `backtrader` library.
- **Database:** SQLite, accessed by both the Go and Python components.
- **Data Analysis:** Python with `pandas`.
- **Development:** `air` for hot-reloading the Go application, and `pytest` for Python testing.

## Project Structure

The project is organized into several key directories:

- **`cmd/web/`**: Contains the main Go application, including the web server, API routes, and configuration management.
- **`pkg/types/`**: Defines the Go data structures used throughout the application.
- **`strategies/`**: Houses the Python-based trading strategies, such as `BuySP500Up20`.
- **`sql/`**: Stores all SQL scripts for creating tables and performing queries.
- **`tests/`**: Contains Python tests for the backtesting engine.
- **`html/`**: Holds HTML templates for the web interface.
- **`config/`**: Contains configuration files for the application.

## Key Components

### 1. Go Web Application (`cmd/web/`)

- **`main.go`**: The entry point for the Go application. It initializes the database, sets up the web server, and defines the API endpoints.
- **API Endpoints**:
    - `/api/stock/{symbol}`: Fetches real-time data for a specific stock.
    - `/api/stock/historical/fill`: Populates the database with historical data for all S&P 500 stocks.
    - `/api/tables`: Lists all tables in the database.
    - `/api/tables/{table}`: Allows browsing the contents of a specific database table.
- **Data Fetching**: The Go application is responsible for fetching historical and real-time stock data from Yahoo Finance and storing it in the SQLite database. It uses goroutines to perform these tasks concurrently, improving performance.

### 2. Python Backtesting Engine

- **`portfolio_backtest.py`**: The main script for running backtests. It uses the `backtrader` library to simulate trading strategies against the historical data stored in the SQLite database.
- **`db_manager.py`**: A singleton class that manages the connection to the SQLite database, ensuring that all parts of the Python application use the same connection. It includes features like automatic lock detection and cleanup.
- **`database_log_handler.py`**: A custom logging handler that writes log messages to the `logs` table in the database, providing a persistent record of backtest runs.
- **`strategies/`**: This directory contains the individual trading strategies. Each strategy is a Python class that inherits from `backtrader.Strategy`.

### 3. Database

- **`backtest_sell_limits.db`**: The SQLite database file.
- **Schema**: The database schema is defined in the `sql/` directory. Key tables include:
    - `stock_historical_data`: Stores OHLCV (Open, High, Low, Close, Volume) data for each stock.
    - `sp500_list_2025_jun`: A list of S&P 500 tickers.
    - `logs`: A table for storing log messages from the Python application.
    - `backtest_strategies`, `backtest_daily_values`, `strategy_history`: Tables for storing the results of backtests.

## How to Run the Application

### Go Application

To run the Go web server with hot-reloading:

```bash
air
```

To build and run the application manually:

```bash
go build -o tmp/main cmd/web/main.go cmd/web/config.go
./tmp/main --config config/config.json
```

### Python Backtesting

To run a backtest:

```bash
./run_backtest_python.sh
```

Or directly:

```bash
python3 portfolio_backtest.py
```

## Gemini's Role

As the Gemini Code Assistant, my role is to assist with the development and maintenance of this project. I can help with:

- **Understanding the Codebase**: I can explain the purpose of different files and components.
- **Writing Code**: I can write new code, such as new trading strategies, API endpoints, or database queries.
- **Debugging**: I can help identify and fix bugs in both the Python and Go code.
- **Refactoring**: I can suggest and implement improvements to the code to make it more efficient, readable, and maintainable.
- **Documentation**: I can help write and update documentation, such as this `GEMINI.md` file.
