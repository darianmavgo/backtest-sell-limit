-- SQLite-compatible script to fill data gaps and prevent early position exits
-- This creates continuous data for all symbols from start to end date

-- 1. Create helper table with all dates in the range
DROP TABLE IF EXISTS all_dates;
CREATE TEMPORARY TABLE all_dates AS
WITH RECURSIVE date_range(date_unix) AS (
    SELECT MIN(date) as date_unix FROM stock_historical_data
    UNION ALL
    SELECT date_unix + 86400  -- Add 1 day in seconds
    FROM date_range
    WHERE date_unix < (SELECT MAX(date) FROM stock_historical_data)
)
SELECT date_unix FROM date_range;

-- 2. Find missing symbol-date combinations
DROP TABLE IF EXISTS missing_data;
CREATE TEMPORARY TABLE missing_data AS
SELECT 
    s.symbol,
    d.date_unix as date
FROM 
    (SELECT DISTINCT symbol FROM stock_historical_data) s
CROSS JOIN 
    all_dates d
LEFT JOIN 
    stock_historical_data shd ON s.symbol = shd.symbol AND d.date_unix = shd.date
WHERE 
    shd.date IS NULL;

-- 3. Insert dummy data using forward-fill logic (last known price)
INSERT INTO stock_historical_data (symbol, date, open, high, low, close, volume)
SELECT 
    md.symbol,
    md.date,
    COALESCE(
        (SELECT close FROM stock_historical_data 
         WHERE symbol = md.symbol AND date < md.date 
         ORDER BY date DESC LIMIT 1), 
        100.0  -- Default price if no prior data
    ) as open,
    COALESCE(
        (SELECT close FROM stock_historical_data 
         WHERE symbol = md.symbol AND date < md.date 
         ORDER BY date DESC LIMIT 1), 
        100.0
    ) as high,
    COALESCE(
        (SELECT close FROM stock_historical_data 
         WHERE symbol = md.symbol AND date < md.date 
         ORDER BY date DESC LIMIT 1), 
        100.0
    ) as low,
    COALESCE(
        (SELECT close FROM stock_historical_data 
         WHERE symbol = md.symbol AND date < md.date 
         ORDER BY date DESC LIMIT 1), 
        100.0
    ) as close,
    0 as volume  -- Zero volume for dummy data
FROM 
    missing_data md;

-- 4. Show summary of filled data
SELECT 
    'Data gaps filled for' as info,
    COUNT(*) as records_added,
    MIN(date) as earliest_fill,
    MAX(date) as latest_fill
FROM stock_historical_data 
WHERE volume = 0;
