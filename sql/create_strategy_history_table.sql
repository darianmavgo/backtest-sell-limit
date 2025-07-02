CREATE TABLE IF NOT EXISTS strategy_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    strategy_name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    trade_type TEXT NOT NULL,  -- 'BUY', 'SELL', 'LONG', 'SHORT', etc.
    trade_status TEXT NOT NULL,  -- 'OPEN', 'CLOSE', 'FILLED', 'PENDING', etc.
    quantity REAL NOT NULL,
    price REAL NOT NULL,
    value REAL NOT NULL,
    pnl REAL DEFAULT 0,
    pnl_percent REAL DEFAULT 0,
    commission REAL DEFAULT 0,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    trade_date TEXT NOT NULL,  -- Date of the actual trade in YYYY-MM-DD format
    notes TEXT,
    FOREIGN KEY (symbol) REFERENCES stock_data(symbol)
);