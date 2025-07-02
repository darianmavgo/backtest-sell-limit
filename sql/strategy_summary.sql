-- Summary of strategy performance
SELECT 
    strategy_name,
    COUNT(*) as total_trades,
    COUNT(CASE WHEN trade_status = 'OPEN' THEN 1 END) as open_trades,
    COUNT(CASE WHEN trade_status = 'CLOSE' THEN 1 END) as closed_trades,
    SUM(CASE WHEN trade_status = 'CLOSE' THEN pnl ELSE 0 END) as total_pnl,
    AVG(CASE WHEN trade_status = 'CLOSE' AND pnl_percent IS NOT NULL THEN pnl_percent ELSE NULL END) as avg_pnl_percent,
    MIN(trade_date) as first_trade_date,
    MAX(trade_date) as last_trade_date
FROM strategy_history
GROUP BY strategy_name;