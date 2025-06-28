SELECT 
    datetime(date, 'unixepoch') as date,
    open,
    high,
    low,
    close,
    adj_close,
    volume
FROM stock_historical_data
WHERE symbol = ?
AND date >= strftime('%s', ?)
AND date <= strftime('%s', ?)
ORDER BY date; 