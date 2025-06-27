# Backtest Sell Limit

A Go web application that processes Gmail backtest emails and fetches S&P 500 stock data.

## Features

### Gmail Integration
- Login with Google OAuth2
- Fetch emails with specific labels
- Store email content in SQLite database
- Fix email dates from various formats

### S&P 500 Stock Data
- Fetch real-time stock data for all S&P 500 companies
- Concurrent processing using goroutines for fast data retrieval
- Store comprehensive stock information including:
  - Current price and price changes
  - Volume and market cap
  - Daily high/low and 52-week high/low
  - Previous close and opening prices
- SQLite database storage with proper indexing

## Endpoints

- `/` - Home page with navigation
- `/login` - Google OAuth2 login
- `/callback` - OAuth2 callback handler
- `/batchget` - Fetch emails from Gmail
- `/fixdate` - Fix email dates
- `/sp500` - Fetch S&P 500 stock data (NEW)

## Usage

1. Start the server:
   ```bash
   go run main.go
   ```

2. Open http://localhost:8080 in your browser

3. Use the web interface to:
   - Login with Google for email processing
   - Fetch S&P 500 data using the "Fetch S&P 500 Data" button

## Database Schema

### Emails Table
Stores processed Gmail messages with headers and content.

### Stock Data Table
Stores S&P 500 stock information with the following fields:
- symbol, company_name, price, change_amount, change_percent
- volume, market_cap, previous_close, open_price
- high, low, fifty_two_week_high, fifty_two_week_low
- last_updated timestamp

## Performance

The S&P 500 data fetching uses:
- 20 concurrent goroutines for parallel API calls
- Worker pool pattern for efficient resource management
- Timeout handling for API requests
- Error handling and logging for failed requests

## Dependencies

- SQLite3 for database storage
- Google APIs for Gmail integration
- Yahoo Finance API for stock data
- Standard Go libraries for HTTP and concurrency

## Configuration

Update the `credentialsFile` constant in `main.go` with your Google OAuth2 credentials file path. 