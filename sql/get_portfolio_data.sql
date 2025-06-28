SELECT 
    p.symbol,
    p.shares,
    p.purchase_price as cost_basis,
    s.price as current_price
FROM portfolio p
JOIN stock_data s ON p.symbol = s.symbol
WHERE p.shares > 0; 