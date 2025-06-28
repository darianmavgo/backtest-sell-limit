CREATE TABLE IF NOT EXISTS logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT,
    logger_name TEXT,
    level TEXT,
    message TEXT,
    symbol TEXT,
    order_type TEXT,
    status TEXT,
    price REAL,
    size INTEGER,
    order_ref INTEGER,
    parent_ref INTEGER
); 