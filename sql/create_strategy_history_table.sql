CREATE TABLE IF NOT EXISTS strategy_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    strategy_name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    trade_type TEXT NOT NULL,  -- 'LONG', 'SHORT', etc.
    trade_status TEXT NOT NULL,  -- 'OPEN', 'CLOSE', etc.
    size REAL NOT NULL,
    price REAL NOT NULL,
    value REAL NOT NULL,
    pnl REAL,
    pnl_percent REAL,
    commission REAL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (symbol) REFERENCES stock_data(symbol)
); 