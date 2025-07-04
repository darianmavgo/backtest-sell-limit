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
	SPXLBacktestDB *sql.DB
)

// initSPXLDB initializes the SQLite database for SPXL with WAL mode and creates the schema
func initSPXLDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", C.SPXLBacktestDB+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open SPXL database: %v", err)
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
		return nil, fmt.Errorf("failed to create stock_historical_data table for SPXL: %v", err)
	}

	return db, nil
}

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

// getSymbolsFromTable retrieves a list of symbols from a specified table.
func getSymbolsFromTable(db *sql.DB, tableName string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT symbol FROM %s", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query symbols from table %s: %v", tableName, err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan symbol: %v", err)
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// fillHistoricalDataHandler fetches and fills historical data based on query parameters.
func fillHistoricalDataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting historical data fill process...")

	// Parse query parameters
	symbolsParam := r.URL.Query().Get("symbols")
	tableName := r.URL.Query().Get("table_name")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	var symbols []string
	var err error

	// Determine symbols to fetch
	if symbolsParam != "" {
		symbols = strings.Split(symbolsParam, ",")
	} else if tableName != "" {
		// Determine which database to use based on table name
		dbToUse := BacktestDB // Default to BacktestDB
		if tableName == "spxl_tickers" {
			dbToUse = SPXLBacktestDB
		}
		symbols, err = getSymbolsFromTable(dbToUse, tableName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting symbols from table %s: %v", tableName, err), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Either 'symbols' or 'table_name' parameter is required.", http.StatusBadRequest)
		return
	}

	// Parse start and end dates
	endDate := time.Now()
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			http.Error(w, "Invalid 'end_date' format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
	}

	startDate := endDate.AddDate(-5, 0, 0) // Default to 5 years ago
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			http.Error(w, "Invalid 'start_date' format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
	}

	log.Printf("Fetching historical data for %d symbols from %s to %s", len(symbols), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	var wg sync.WaitGroup
	results := make(chan struct { symbol string; err error }, len(symbols))

	for _, symbol := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			log.Printf("Processing %s...", s)
			historicalData, fetchErr := fetchHistoricalTickerData(s, startDate, endDate)
			if fetchErr != nil {
				results <- struct { symbol string; err error }{s, fmt.Errorf("failed to fetch data: %v", fetchErr)}
				return
			}

			// Determine which database to save to
			dbToSave := BacktestDB // Default to BacktestDB
			if tableName == "spxl_tickers" || s == "SPXL" {
				dbToSave = SPXLBacktestDB
			}

			if saveErr := saveHistoricalData(dbToSave, s, historicalData); saveErr != nil {
				results <- struct { symbol string; err error }{s, fmt.Errorf("failed to save data: %v", saveErr)}
				return
			}
			results <- struct { symbol string; err error }{s, nil}
		}(symbol)
	}

	wg.Wait()
	close(results)

	successCount := 0
	failures := make(map[string]string)

	for res := range results {
		if res.err != nil {
			failures[res.symbol] = res.err.Error()
		} else {
			successCount++
		}
	}

	response := map[string]interface{}{
		"status":        "completed",
		"processed":     len(symbols),
		"success_count": successCount,
		"failures":      failures,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// saveHistoricalData saves historical stock data to the database
func saveHistoricalData(db *sql.DB, symbol string, data []types.HistoricalData) error {
	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO stock_historical_data (
			symbol, date, open, high, low, close, adj_close, volume
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, hd := range data {
		_, err = stmt.Exec(
			symbol,
			hd.Date.Unix(),
			hd.Open,
			hd.High,
			hd.Low,
			hd.Close,
			hd.AdjClose,
			hd.Volume,
		)
		if err != nil {
			return fmt.Errorf("failed to insert historical data for %s on %s: %v", symbol, hd.Date.Format("2006-01-02"), err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
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

	SPXLBacktestDB, err = initSPXLDB()
	if err != nil {
		log.Fatalln("SPXL DB failure ", err)
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

// getSymbolsFromTable retrieves a list of symbols from a specified table.
func getSymbolsFromTable(db *sql.DB, tableName string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT symbol FROM %s", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query symbols from table %s: %v", tableName, err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan symbol: %v", err)
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// fillHistoricalDataHandler fetches and fills historical data based on query parameters.
func fillHistoricalDataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting historical data fill process...")

	// Parse query parameters
	symbolsParam := r.URL.Query().Get("symbols")
	tableName := r.URL.Query().Get("table_name")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	var symbols []string
	var err error

	// Determine symbols to fetch
	if symbolsParam != "" {
		symbols = strings.Split(symbolsParam, ",")
	} else if tableName != "" {
		// Determine which database to use based on table name
		dbToUse := BacktestDB // Default to BacktestDB
		if tableName == "spxl_tickers" {
			dbToUse = SPXLBacktestDB
		}
		symbols, err = getSymbolsFromTable(dbToUse, tableName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting symbols from table %s: %v", tableName, err), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Either 'symbols' or 'table_name' parameter is required.", http.StatusBadRequest)
		return
	}

	// Parse start and end dates
	endDate := time.Now()
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			http.Error(w, "Invalid 'end_date' format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
	}

	startDate := endDate.AddDate(-5, 0, 0) // Default to 5 years ago
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			http.Error(w, "Invalid 'start_date' format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
	}

	log.Printf("Fetching historical data for %d symbols from %s to %s", len(symbols), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	var wg sync.WaitGroup
	results := make(chan struct { symbol string; err error }, len(symbols))

	for _, symbol := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			log.Printf("Processing %s...", s)
			historicalData, fetchErr := fetchHistoricalTickerData(s, startDate, endDate)
			if fetchErr != nil {
				results <- struct { symbol string; err error }{s, fmt.Errorf("failed to fetch data: %v", fetchErr)}
				return
			}

			// Determine which database to save to
			dbToSave := BacktestDB // Default to BacktestDB
			if tableName == "spxl_tickers" || s == "SPXL" {
				dbToSave = SPXLBacktestDB
			}

			if saveErr := saveHistoricalData(dbToSave, s, historicalData); saveErr != nil {
				results <- struct { symbol string; err error }{s, fmt.Errorf("failed to save data: %v", saveErr)}
				return
			}
			results <- struct { symbol string; err error }{s, nil}
		}(symbol)
	}

	wg.Wait()
	close(results)

	successCount := 0
	failures := make(map[string]string)

	for res := range results {
		if res.err != nil {
			failures[res.symbol] = res.err.Error()
		} else {
			successCount++
		}
	}

	response := map[string]interface{}{
		"status":        "completed",
		"processed":     len(symbols),
		"success_count": successCount,
		"failures":      failures,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// saveHistoricalData saves historical stock data to the database
func saveHistoricalData(db *sql.DB, symbol string, data []types.HistoricalData) error {
	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO stock_historical_data (
			symbol, date, open, high, low, close, adj_close, volume
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, hd := range data {
		_, err = stmt.Exec(
			symbol,
			hd.Date.Unix(),
			hd.Open,
			hd.High,
			hd.Low,
			hd.Close,
			hd.AdjClose,
			hd.Volume,
		)
		if err != nil {
			return fmt.Errorf("failed to insert historical data for %s on %s: %v", symbol, hd.Date.Format("2006-01-02"), err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
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

	SPXLBacktestDB, err = initSPXLDB()
	if err != nil {
		log.Fatalln("SPXL DB failure ", err)
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

// getSymbolsFromTable retrieves a list of symbols from a specified table.
func getSymbolsFromTable(db *sql.DB, tableName string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT symbol FROM %s", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query symbols from table %s: %v", tableName, err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan symbol: %v", err)
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// fillHistoricalDataHandler fetches and fills historical data based on query parameters.
func fillHistoricalDataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting historical data fill process...")

	// Parse query parameters
	symbolsParam := r.URL.Query().Get("symbols")
	tableName := r.URL.Query().Get("table_name")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	var symbols []string
	var err error

	// Determine symbols to fetch
	if symbolsParam != "" {
		symbols = strings.Split(symbolsParam, ",")
	} else if tableName != "" {
		// Determine which database to use based on table name
		dbToUse := BacktestDB // Default to BacktestDB
		if tableName == "spxl_tickers" {
			dbToUse = SPXLBacktestDB
		}
		symbols, err = getSymbolsFromTable(dbToUse, tableName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting symbols from table %s: %v", tableName, err), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Either 'symbols' or 'table_name' parameter is required.", http.StatusBadRequest)
		return
	}

	// Parse start and end dates
	endDate := time.Now()
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			http.Error(w, "Invalid 'end_date' format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
	}

	startDate := endDate.AddDate(-5, 0, 0) // Default to 5 years ago
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			http.Error(w, "Invalid 'start_date' format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
	}

	log.Printf("Fetching historical data for %d symbols from %s to %s", len(symbols), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	var wg sync.WaitGroup
	results := make(chan struct { symbol string; err error }, len(symbols))

	for _, symbol := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			log.Printf("Processing %s...", s)
			historicalData, fetchErr := fetchHistoricalTickerData(s, startDate, endDate)
			if fetchErr != nil {
				results <- struct { symbol string; err error }{s, fmt.Errorf("failed to fetch data: %v", fetchErr)}
				return
			}

			// Determine which database to save to
			dbToSave := BacktestDB // Default to BacktestDB
			if tableName == "spxl_tickers" || s == "SPXL" {
				dbToSave = SPXLBacktestDB
			}

			if saveErr := saveHistoricalData(dbToSave, s, historicalData); saveErr != nil {
				results <- struct { symbol string; err error }{s, fmt.Errorf("failed to save data: %v", saveErr)}
				return
			}
			results <- struct { symbol string; err error }{s, nil}
		}(symbol)
	}

	wg.Wait()
	close(results)

	successCount := 0
	failures := make(map[string]string)

	for res := range results {
		if res.err != nil {
			failures[res.symbol] = res.err.Error()
		} else {
			successCount++
		}
	}

	response := map[string]interface{}{
		"status":        "completed",
		"processed":     len(symbols),
		"success_count": successCount,
		"failures":      failures,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// saveHistoricalData saves historical stock data to the database
func saveHistoricalData(db *sql.DB, symbol string, data []types.HistoricalData) error {
	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO stock_historical_data (
			symbol, date, open, high, low, close, adj_close, volume
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	for _, hd := range data {
		_, err = stmt.Exec(
			symbol,
			hd.Date.Unix(),
			hd.Open,
			hd.High,
			hd.Low,
			hd.Close,
			hd.AdjClose,
			hd.Volume,
		)
		if err != nil {
			return fmt.Errorf("failed to insert historical data for %s on %s: %v", symbol, hd.Date.Format("2006-01-02"), err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
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

	SPXLBacktestDB, err = initSPXLDB()
	if err != nil {
		log.Fatalln("SPXL DB failure ", err)
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


// getSymbolsFromTable retrieves a list of symbols from a specified table.
func getSymbolsFromTable(db *sql.DB, tableName string) ([]string, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT symbol FROM %s", tableName))
	if err != nil {
		return nil, fmt.Errorf("failed to query symbols from table %s: %v", tableName, err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan symbol: %v", err)
		}
		symbols = append(symbols, symbol)
	}

	return symbols, nil
}

// fillHistoricalDataHandler fetches and fills historical data based on query parameters.
func fillHistoricalDataHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Starting historical data fill process...")

	// Parse query parameters
	symbolsParam := r.URL.Query().Get("symbols")
	tableName := r.URL.Query().Get("table_name")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")

	var symbols []string
	var err error

	// Determine symbols to fetch
	if symbolsParam != "" {
		symbols = strings.Split(symbolsParam, ",")
	} else if tableName != "" {
		// Determine which database to use based on table name
		dbToUse := BacktestDB // Default to BacktestDB
		if tableName == "spxl_tickers" {
			dbToUse = SPXLBacktestDB
		}
		symbols, err = getSymbolsFromTable(dbToUse, tableName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting symbols from table %s: %v", tableName, err), http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Either 'symbols' or 'table_name' parameter is required.", http.StatusBadRequest)
		return
	}

	// Parse start and end dates
	endDate := time.Now()
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			http.Error(w, "Invalid 'end_date' format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
	}

	startDate := endDate.AddDate(-5, 0, 0) // Default to 5 years ago
	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			http.Error(w, "Invalid 'start_date' format. Use YYYY-MM-DD.", http.StatusBadRequest)
			return
		}
	}

	log.Printf("Fetching historical data for %d symbols from %s to %s", len(symbols), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	var wg sync.WaitGroup
	results := make(chan struct { symbol string; err error }, len(symbols))

	for _, symbol := range symbols {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			log.Printf("Processing %s...", s)
			historicalData, fetchErr := fetchHistoricalTickerData(s, startDate, endDate)
			if fetchErr != nil {
				results <- struct { symbol string; err error }{s, fmt.Errorf("failed to fetch data: %v", fetchErr)}
				return
			}

			// Determine which database to save to
			dbToSave := BacktestDB // Default to BacktestDB
			if tableName == "spxl_tickers" || s == "SPXL" {
				dbToSave = SPXLBacktestDB
			}

			if saveErr := saveHistoricalData(dbToSave, s, historicalData); saveErr != nil {
				results <- struct { symbol string; err error }{s, fmt.Errorf("failed to save data: %v", saveErr)}
				return
			}
			results <- struct { symbol string; err error }{s, nil}
		}(symbol)
	}

	wg.Wait()
	close(results)

	successCount := 0
	failures := make(map[string]string)

	for res := range results {
		if res.err != nil {
			failures[res.symbol] = res.err.Error()
		} else {
			successCount++
		}
	}

	response := map[string]interface{}{
		"status":        "completed",
		"processed":     len(symbols),
		"success_count": successCount,
		"failures":      failures,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

	SPXLBacktestDB, err = initSPXLDB()
	if err != nil {
		log.Fatalln("SPXL DB failure ", err)
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
