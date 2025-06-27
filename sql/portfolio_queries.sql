-- =====================================================
-- S&P 500 Portfolio Quick Reference Queries
-- =====================================================

-- 💰 Total investment required for 1 share of each S&P 500 stock
SELECT 
    '💰 Total Investment: $' || PRINTF('%.2f', total_current_value) as summary
FROM portfolio_summary;

-- 📊 Portfolio overview
SELECT * FROM portfolio_summary;

-- 🔝 Top 10 most valuable holdings
SELECT 
    symbol,
    company_name,
    '$' || PRINTF('%.2f', current_price) as price,
    '$' || PRINTF('%.2f', current_value) as total_value
FROM top_10_holdings;

-- 💸 Top 10 most expensive individual stocks
SELECT 
    symbol,
    company_name,
    '$' || PRINTF('%.2f', current_price) as price
FROM portfolio_details 
ORDER BY current_price DESC 
LIMIT 10;

-- �� Top 10 cheapest stocks (potential bargains)
SELECT 
    symbol,
    company_name,
    '$' || PRINTF('%.2f', current_price) as price
FROM portfolio_details 
ORDER BY current_price ASC 
LIMIT 10;

-- 📈 Sector distribution by value
SELECT * FROM portfolio_by_sector ORDER BY sector_value DESC;

-- 🎯 Portfolio details for specific stocks
SELECT 
    symbol,
    company_name,
    '$' || PRINTF('%.2f', current_price) as price,
    PRINTF('%.2f', daily_change_percent) || '%' as daily_change
FROM portfolio_details 
WHERE symbol IN ('AAPL', 'MSFT', 'GOOGL', 'AMZN', 'NVDA', 'TSLA');

-- 💪 Stocks over $1000 (high-value stocks)
SELECT 
    symbol,
    company_name,
    '$' || PRINTF('%.2f', current_price) as price
FROM portfolio_details 
WHERE current_price > 1000 
ORDER BY current_price DESC;

-- 🏷️ Stocks under $20 (penny stocks relative to S&P 500)
SELECT 
    symbol,
    company_name,
    '$' || PRINTF('%.2f', current_price) as price,
    market_cap
FROM portfolio_details 
WHERE current_price < 20 
ORDER BY current_price ASC;

-- 📊 Price distribution analysis
SELECT 
    CASE 
        WHEN current_price < 50 THEN 'Under $50'
        WHEN current_price < 100 THEN '$50-$100' 
        WHEN current_price < 200 THEN '$100-$200'
        WHEN current_price < 500 THEN '$200-$500'
        WHEN current_price < 1000 THEN '$500-$1000'
        ELSE 'Over $1000'
    END as price_range,
    COUNT(*) as stock_count,
    '$' || PRINTF('%.0f', SUM(current_value)) as total_value
FROM portfolio_details
GROUP BY 
    CASE 
        WHEN current_price < 50 THEN 'Under $50'
        WHEN current_price < 100 THEN '$50-$100'
        WHEN current_price < 200 THEN '$100-$200'
        WHEN current_price < 500 THEN '$200-$500'
        WHEN current_price < 1000 THEN '$500-$1000'
        ELSE 'Over $1000'
    END
ORDER BY MIN(current_price);
