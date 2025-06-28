-- Query to show detailed order history for ABBV stock
SELECT 
    symbol,
    order_type,
    date(execution_date) as trade_date,
    round(execution_price, 2) as price,
    quantity,
    round(execution_price * quantity, 2) as total_value,
    is_main_order,
    parent_order_type,
    order_ref,
    parent_ref,
    -- Market data at time of execution
    round(tick_open, 2) as market_open,
    round(tick_high, 2) as market_high,
    round(tick_low, 2) as market_low,
    round(tick_close, 2) as market_close,
    tick_volume as volume
FROM backtest_order_history
WHERE symbol = 'ABBV'
ORDER BY execution_date, order_type DESC;  -- Shows buys before sells on same day 