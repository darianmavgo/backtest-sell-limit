-- Query to show profitable, completed sell orders from the logs
WITH buys AS (
    -- Get the initial buy price for each main order
    SELECT
        order_ref,
        symbol,
        price as buy_price,
        timestamp as buy_date
    FROM logs
    WHERE 
        status = 'Completed'
        AND order_type = 'Buy'
        AND parent_ref IS NULL -- This identifies the main buy order of a bracket
),
sells AS (
    -- Get the sell price for each completed child order
    SELECT
        parent_ref,
        price as sell_price,
        timestamp as sell_date
    FROM logs
    WHERE
        status = 'Completed'
        AND order_type = 'Sell'
)
SELECT
    b.symbol,
    b.buy_date,
    round(b.buy_price, 2) as buy_price,
    s.sell_date,
    round(s.sell_price, 2) as sell_price,
    round((s.sell_price - b.buy_price) * 100 / b.buy_price, 2) as percent_gain
FROM buys b
JOIN sells s ON b.order_ref = s.parent_ref
WHERE percent_gain >= 19.9 -- Show trades that hit our ~20% target
ORDER BY percent_gain DESC; 