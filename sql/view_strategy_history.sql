-- View all strategy history trades
SELECT 
    id,
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
    trade_date,
    timestamp
FROM strategy_history
ORDER BY timestamp DESC;