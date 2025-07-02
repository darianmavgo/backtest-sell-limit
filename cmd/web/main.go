package main

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"crypto/tls"

	"github.com/darianmavgo/backtest-sell-limit/pkg/types"
	_ "github.com/mattn/go-sqlite3"
)

var (
	BacktestDB *sql.DB
)

// initDB initializes the SQLite database with WAL mode and creates the schema
func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", C.BacktestDB+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create the stock_historical_data table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS stock_historical_data (
			symbol TEXT,
			date INTEGER,
			open REAL,
			high REAL,
			low REAL,
			close REAL,
			adj_close REAL,
			volume INTEGER,
			PRIMARY KEY (symbol, date)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create stock_historical_data table: %v", err)
	}

	return db, nil
}



func createTables(db *sql.DB) error {
	var err error
	// Create the stock_data table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS ticker_list (
			symbol TEXT PRIMARY KEY,
			company_name TEXT,
			price REAL,
			change_amount REAL,
			change_percent REAL,
			volume INTEGER,
			market_cap INTEGER,
			previous_close REAL,
			open_price REAL,
			high REAL,
			low REAL,
			fifty_two_week_high REAL,
			fifty_two_week_low REAL,
			last_updated INTEGER,
			UNIQUE(symbol)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create stock_data table: %v", err)
	}

	// Create the stock_historical_data table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS stock_historical_data (
			symbol TEXT,
			date INTEGER,
			open REAL,
			high REAL,
			low REAL,
			close REAL,
			adj_close REAL,
			volume INTEGER,
			PRIMARY KEY (symbol, date)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create stock_historical_data table: %v", err)
	}

	return nil
}






// fetchStockData fetches stock data for a given ticker using a free API
func fetchStockData(ticker string) (*types.StockData, error) {
	// Using Yahoo Finance API (free alternative)
	url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s", ticker)

	// Create a custom transport with TLS config
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Only for development/testing
		},
	}

	// Create a new client with the custom transport
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: tr,
	}

	// Create request with User-Agent header (Yahoo Finance requires this)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %v", ticker, err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data for %s: %v", ticker, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed for %s: status %d", ticker, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response for %s: %v", ticker, err)
	}

	// Parse Yahoo Finance response
	var yahooResp struct {
		Chart struct {
			Result []struct {
				Meta struct {
					Currency             string  `json:"currency"`
					Symbol               string  `json:"symbol"`
					RegularMarketPrice   float64 `json:"regularMarketPrice"`
					PreviousClose        float64 `json:"previousClose"`
					RegularMarketOpen    float64 `json:"regularMarketOpen"`
					RegularMarketDayHigh float64 `json:"regularMarketDayHigh"`
					RegularMarketDayLow  float64 `json:"regularMarketDayLow"`
					RegularMarketVolume  int64   `json:"regularMarketVolume"`
					MarketCap            int64   `json:"marketCap"`
					FiftyTwoWeekHigh     float64 `json:"fiftyTwoWeekHigh"`
					FiftyTwoWeekLow      float64 `json:"fiftyTwoWeekLow"`
					LongName             string  `json:"longName"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON for %s: %v", ticker, err)
	}

	if len(yahooResp.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data returned for %s", ticker)
	}

	meta := yahooResp.Chart.Result[0].Meta
	currentPrice := meta.RegularMarketPrice
	previousClose := meta.PreviousClose
	change := currentPrice - previousClose
	changePercent := (change / previousClose) * 100

	return &types.StockData{
		Symbol:           ticker,
		CompanyName:      meta.LongName,
		Price:            currentPrice,
		ChangeAmount:     change,
		ChangePercent:    changePercent,
		Volume:           meta.RegularMarketVolume,
		MarketCap:        meta.MarketCap,
		PreviousClose:    previousClose,
		OpenPrice:        meta.RegularMarketOpen,
		High:             meta.RegularMarketDayHigh,
		Low:              meta.RegularMarketDayLow,
		FiftyTwoWeekHigh: meta.FiftyTwoWeekHigh,
		FiftyTwoWeekLow:  meta.FiftyTwoWeekLow,
		LastUpdated:      int64(meta.RegularMarketPrice),
		Date:             time.Now(),
		Open:             meta.RegularMarketOpen,
		Close:            currentPrice,
		AdjClose:         currentPrice,
	}, nil
}

// saveStockData saves stock data to the database
func saveStockData(stock *types.StockData) error {
	stmt, err := BacktestDB.Prepare(`
		INSERT OR REPLACE INTO stock_data (
			symbol, company_name, price, change_amount, change_percent,
			volume, market_cap, previous_close, open_price, high, low,
			fifty_two_week_high, fifty_two_week_low, last_updated
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		stock.Symbol,
		stock.CompanyName,
		stock.Price,
		stock.ChangeAmount,
		stock.ChangePercent,
		stock.Volume,
		stock.MarketCap,
		stock.PreviousClose,
		stock.OpenPrice,
		stock.High,
		stock.Low,
		stock.FiftyTwoWeekHigh,
		stock.FiftyTwoWeekLow,
		stock.LastUpdated,
	)
	if err != nil {
		return fmt.Errorf("failed to insert stock data: %v", err)
	}

	return nil
}

// fetchAllSP500Data fetches data for all S&P 500 stocks
func fetchAllSP500Data() error {
	// Get list of active tickers
	tickers, err := getActiveSP500Tickers(BacktestDB)
	if err != nil {
		return fmt.Errorf("failed to get tickers: %v", err)
	}

	// Process each ticker
	for _, symbol := range tickers {
		// Get historical data
		data, err := fetchHistoricalData(symbol)
		if err != nil {
			log.Printf("Failed to fetch historical data for %s: %v", symbol, err)
			continue
		}

		// Save to database
		if err := saveHistoricalData(BacktestDB, symbol, data); err != nil {
			log.Printf("Failed to save historical data for %s: %v", symbol, err)
			continue
		}
	}

	return nil
}


// fetchHistoricalTickerData fetches historical data for a single ticker from Yahoo Finance
func fetchHistoricalTickerData(ticker string, startDate, endDate time.Time) ([]types.HistoricalData, error) {
	// Yahoo Finance API URL
	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d&includeAdjustedClose=true",
		ticker,
		startDate.Unix(),
		endDate.Unix(),
	)

	// Create a custom transport with TLS config
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Only for development/testing
		},
	}

	// Create a new client with the custom transport
	client := &http.Client{
		Timeout:   20 * time.Second,
		Transport: tr,
	}

	// Create a new request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Add required headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko)")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Origin", "https://finance.yahoo.com")
	req.Header.Set("Referer", fmt.Sprintf("https://finance.yahoo.com/quote/%s", ticker))

	// Make the request with retries
	var resp *http.Response
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		resp, err = client.Do(req)
		if err != nil {
			if i == maxRetries-1 {
				return nil, fmt.Errorf("failed to fetch data after %d retries: %v", maxRetries, err)
			}
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}

		if resp.StatusCode == 429 { // Too Many Requests
			if i == maxRetries-1 {
				return nil, fmt.Errorf("rate limit exceeded after %d retries", maxRetries)
			}
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			resp.Body.Close()
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
		}
		break
	}
	defer resp.Body.Close()

	// Handle gzip compression
	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %v", err)
		}
		defer gzReader.Close()
		reader = gzReader
	default:
		reader = resp.Body
	}

	// Read response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Parse JSON response
	var yahooResp struct {
		Chart struct {
			Result []struct {
				Meta struct {
					Symbol string  `json:"symbol"`
					First  float64 `json:"firstTradeDate"`
				} `json:"meta"`
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
					Adjclose []struct {
						Adjclose []float64 `json:"adjclose"`
					} `json:"adjclose"`
				} `json:"indicators"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &yahooResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v, body: %s", err, string(body))
	}

	// Check for API errors
	if yahooResp.Chart.Error != nil {
		return nil, fmt.Errorf("API error: %s - %s", yahooResp.Chart.Error.Code, yahooResp.Chart.Error.Description)
	}

	if len(yahooResp.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data returned")
	}

	result := yahooResp.Chart.Result[0]
	if len(result.Indicators.Quote) == 0 {
		return nil, fmt.Errorf("no quote data returned")
	}

	quote := result.Indicators.Quote[0]
	adjclose := result.Indicators.Adjclose[0]

	var historicalData []types.HistoricalData
	for i, ts := range result.Timestamp {
		if i >= len(quote.Open) || i >= len(quote.High) || i >= len(quote.Low) ||
			i >= len(quote.Close) || i >= len(quote.Volume) || i >= len(adjclose.Adjclose) {
			continue
		}

		historicalData = append(historicalData, types.HistoricalData{
			Symbol:   ticker,
			Date:     time.Unix(ts, 0),
			Open:     quote.Open[i],
			High:     quote.High[i],
			Low:      quote.Low[i],
			Close:    quote.Close[i],
			AdjClose: adjclose.Adjclose[i],
			Volume:   quote.Volume[i],
		})
	}

	// Add delay to avoid rate limiting
	time.Sleep(100 * time.Millisecond)

	return historicalData, nil
}










func init() {
	// Initialize configuration
	InitConfig()

	// Initialize database
	var err error
	BacktestDB, err = initDB()
	if err != nil {
		log.Fatalln("DB failure ", err)
	}

	// Create tables
	if err := createTables(BacktestDB); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
}

func main() {
	// Setup routes
	r := setupRoutes()

	// Start the server
	fmt.Printf("Server is running on port %s\n", C.Port)
	log.Fatal(http.ListenAndServe(":"+C.Port, r))
}
