INSERT INTO strategy_history (
    strategy_name,
    symbol,
    trade_type,
    trade_status,
    quantity,
    price,
    value,
    pnl,
    pnl_percent,
    commission,
    trade_date
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?); 