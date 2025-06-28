-- Query to show only executed sell orders (where we hit our 20% target)
SELECT 
    symbol,
    order_type,
    date(execution_date) as trade_date,
    round(execution_price, 2) as price,
    quantity,
    is_main_order,
    parent_order_type,
    order_ref,
    parent_ref
FROM backtest_order_history
WHERE order_type = 'Sell'
ORDER BY execution_date; 