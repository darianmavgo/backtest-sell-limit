INSERT INTO strategy_history (
    strategy_name,
    symbol,
    trade_type,
    trade_status,
    size,
    price,
    value,
    pnl,
    pnl_percent,
    commission
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?); 