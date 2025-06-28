# Backtest Sell Limit

A Python and Go application for backtesting stock portfolio strategies with a focus on sell limit orders.

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

### Setup
1. Clone the repository
2. Install dependencies:
```bash
pip install -r requirements.txt
pip install -r requirements-dev.txt
```

### Running Tests
```bash
pytest tests/
```

### Running Backtests
```bash
./run_backtest.sh
```

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
- Efficient database queries
- Connection pooling
- Transaction batching
- Indexed database tables

## Contributing
1. Fork the repository
2. Create a feature branch
3. Implement changes with tests
4. Submit a pull request

## License
[Add your license here] 