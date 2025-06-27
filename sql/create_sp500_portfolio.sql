-- =====================================================
-- S&P 500 Portfolio Management SQL
-- Creates a portfolio with 1 share of each S&P 500 stock
-- =====================================================

-- Drop existing tables and views if they exist
DROP VIEW IF EXISTS portfolio_summary;
DROP VIEW IF EXISTS portfolio_details;
DROP TABLE IF EXISTS portfolio;

-- Create portfolio table to track stock holdings
CREATE TABLE portfolio (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    shares REAL NOT NULL DEFAULT 0,
    purchase_price REAL, -- Price at which shares were "purchased"
    purchase_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (symbol) REFERENCES stock_data(symbol),
    UNIQUE(symbol)
);

-- Create index for faster lookups
CREATE INDEX idx_portfolio_symbol ON portfolio(symbol);

-- Insert 1 share of each S&P 500 stock into portfolio
-- Using current price as the "purchase price"
INSERT INTO portfolio (symbol, shares, purchase_price)
SELECT 
    symbol,
    1.0 as shares,
    price as purchase_price
FROM stock_data
WHERE symbol IS NOT NULL 
  AND price > 0
ON CONFLICT(symbol) DO UPDATE SET
    shares = 1.0,
    purchase_price = excluded.purchase_price,
    purchase_date = CURRENT_TIMESTAMP;

-- =====================================================
-- PORTFOLIO DETAILS VIEW
-- Shows individual stock holdings with current values
-- =====================================================
CREATE VIEW portfolio_details AS
SELECT 
    p.symbol,
    sd.company_name,
    p.shares,
    p.purchase_price,
    sd.price as current_price,
    (p.shares * p.purchase_price) as purchase_value,
    (p.shares * sd.price) as current_value,
    (p.shares * sd.price) - (p.shares * p.purchase_price) as unrealized_gain_loss,
    CASE 
        WHEN p.purchase_price > 0 THEN 
            ROUND(((sd.price - p.purchase_price) / p.purchase_price) * 100, 2)
        ELSE 0 
    END as gain_loss_percent,
    sd.change_percent as daily_change_percent,
    (p.shares * sd.change_amount) as daily_gain_loss,
    sd.volume,
    sd.market_cap,
    p.purchase_date
FROM portfolio p
JOIN stock_data sd ON p.symbol = sd.symbol
WHERE p.shares > 0
ORDER BY current_value DESC;

-- =====================================================
-- PORTFOLIO SUMMARY VIEW
-- Shows overall portfolio statistics
-- =====================================================
CREATE VIEW portfolio_summary AS
SELECT 
    COUNT(*) as total_stocks,
    SUM(shares) as total_shares,
    ROUND(SUM(purchase_value), 2) as total_purchase_value,
    ROUND(SUM(current_value), 2) as total_current_value,
    ROUND(SUM(unrealized_gain_loss), 2) as total_unrealized_gain_loss,
    ROUND(SUM(daily_gain_loss), 2) as total_daily_gain_loss,
    CASE 
        WHEN SUM(purchase_value) > 0 THEN 
            ROUND((SUM(unrealized_gain_loss) / SUM(purchase_value)) * 100, 2)
        ELSE 0 
    END as total_gain_loss_percent,
    ROUND(AVG(current_price), 2) as avg_stock_price,
    ROUND(MIN(current_price), 2) as min_stock_price,
    ROUND(MAX(current_price), 2) as max_stock_price,
    (SELECT symbol FROM portfolio_details WHERE current_value = (SELECT MAX(current_value) FROM portfolio_details)) as highest_value_holding,
    (SELECT symbol FROM portfolio_details WHERE current_value = (SELECT MIN(current_value) FROM portfolio_details)) as lowest_value_holding,
    COUNT(CASE WHEN unrealized_gain_loss > 0 THEN 1 END) as stocks_with_gains,
    COUNT(CASE WHEN unrealized_gain_loss < 0 THEN 1 END) as stocks_with_losses,
    ROUND(AVG(gain_loss_percent), 2) as avg_gain_loss_percent
FROM portfolio_details;

-- =====================================================
-- ADDITIONAL USEFUL VIEWS
-- =====================================================

-- Top 10 most valuable holdings
CREATE VIEW top_10_holdings AS
SELECT 
    symbol,
    company_name,
    shares,
    current_price,
    current_value,
    gain_loss_percent
FROM portfolio_details
ORDER BY current_value DESC
LIMIT 10;

-- Top 10 best performers (by percentage gain)
CREATE VIEW top_10_gainers AS
SELECT 
    symbol,
    company_name,
    current_price,
    purchase_price,
    gain_loss_percent,
    unrealized_gain_loss
FROM portfolio_details
WHERE gain_loss_percent > 0
ORDER BY gain_loss_percent DESC
LIMIT 10;

-- Top 10 worst performers (by percentage loss)
CREATE VIEW top_10_losers AS
SELECT 
    symbol,
    company_name,
    current_price,
    purchase_price,
    gain_loss_percent,
    unrealized_gain_loss
FROM portfolio_details
WHERE gain_loss_percent < 0
ORDER BY gain_loss_percent ASC
LIMIT 10;

-- Sector-like grouping by first letter (simplified)
CREATE VIEW portfolio_by_sector AS
SELECT 
    UPPER(SUBSTR(symbol, 1, 1)) as sector_group,
    COUNT(*) as stock_count,
    ROUND(SUM(current_value), 2) as sector_value,
    ROUND(AVG(gain_loss_percent), 2) as avg_sector_performance
FROM portfolio_details
GROUP BY UPPER(SUBSTR(symbol, 1, 1))
ORDER BY sector_value DESC;

-- =====================================================
-- SAMPLE QUERIES TO RUN AFTER SETUP
-- =====================================================

/*
-- View portfolio summary
SELECT * FROM portfolio_summary;

-- View top 10 holdings
SELECT * FROM top_10_holdings;

-- View portfolio details for specific stocks
SELECT * FROM portfolio_details WHERE symbol IN ('AAPL', 'MSFT', 'GOOGL', 'AMZN', 'NVDA');

-- Check total cost to buy 1 share of each S&P 500 stock
SELECT 
    'Total cost for 1 share of each S&P 500 stock: $' || 
    ROUND(SUM(price), 2) as investment_required
FROM stock_data WHERE price > 0;

-- Most expensive stocks in portfolio
SELECT symbol, company_name, current_price 
FROM portfolio_details 
ORDER BY current_price DESC 
LIMIT 10;

-- Cheapest stocks in portfolio  
SELECT symbol, company_name, current_price 
FROM portfolio_details 
ORDER BY current_price ASC 
LIMIT 10;
*/
