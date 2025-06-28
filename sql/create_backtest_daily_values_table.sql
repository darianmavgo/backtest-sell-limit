CREATE TABLE IF NOT EXISTS backtest_daily_values (
    strategy_name TEXT,
    date TEXT,
    portfolio_value REAL,
    PRIMARY KEY (strategy_name, date)
); 