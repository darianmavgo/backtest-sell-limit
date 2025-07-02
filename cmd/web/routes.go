package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"database/sql"

	"github.com/darianmavgo/backtest-sell-limit/pkg/types"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gomarkdown/markdown"
	nethtml "golang.org/x/net/html"
)

// homeHandler renders the README.md file as HTML
func homeHandler(w http.ResponseWriter, r *http.Request) {
	readmeHandler(w, r)
}

// readmeHandler serves the raw README markdown with route links
func readmeHandler(w http.ResponseWriter, r *http.Request) {
	// Read the README.md file
	readmeContent, err := os.ReadFile("README.md")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read README.md: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert markdown to HTML with clickable routes
	htmlContent := convertMarkdownToHTML(string(readmeContent))

	// Set content type to HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlContent))
}

// portfolioBacktestHandler runs the portfolio backtest and streams the output
func portfolioBacktestHandler(w http.ResponseWriter, r *http.Request) {
	// Get list of active tickers
	tickers, err := getActiveSP500Tickers(BacktestDB)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get tickers: %v", err), http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "completed",
	})
}

// sp500Handler handles S&P 500 data fetching and updating
func sp500Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: "Method not allowed",
			Data:    nil,
		})
		return
	}

	stocks, err := fetchSP500List()
	if err != nil {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch S&P 500 list: %v", err),
			Data:    nil,
		})
		return
	}

	// Update each stock's data
	for _, stock := range stocks {
		data, err := fetchHistoricalData(stock.Symbol)
		if err != nil {
			log.Printf("Failed to fetch historical data for %s: %v", stock.Symbol, err)
			continue
		}

		if err := saveHistoricalData(BacktestDB, stock.Symbol, data); err != nil {
			log.Printf("Failed to save historical data for %s: %v", stock.Symbol, err)
			continue
		}
	}

	sendJSONResponse(w, types.HandlerResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully updated %d S&P 500 stocks", len(stocks)),
		Data:    stocks,
	})
}

// historicalDataHandler handles requests for historical stock data
func historicalDataHandler(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		http.Error(w, "Symbol is required", http.StatusBadRequest)
		return
	}

	// Query the database for historical data
	rows, err := BacktestDB.Query(`
		SELECT date, open, high, low, close, adj_close, volume
		FROM stock_historical_data 
		WHERE symbol = ?
		ORDER BY date DESC
	`, symbol)
	if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var data []types.StockData
	for rows.Next() {
		var d types.StockData
		var timestamp int64
		err := rows.Scan(
			&timestamp,
			&d.Open,
			&d.High,
			&d.Low,
			&d.Close,
			&d.AdjClose,
			&d.Volume,
		)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error scanning row: %v", err), http.StatusInternalServerError)
			return
		}
		d.Symbol = symbol
		d.Date = time.Unix(timestamp, 0)
		data = append(data, d)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// updateSP500Handler fetches the current S&P 500 list and updates the database
func updateSP500Handler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: "Method not allowed",
		})
		return
	}

	// Create the table if it doesn't exist
	_, err := BacktestDB.Exec(`
		CREATE TABLE IF NOT EXISTS sp500_list_2025_jun (
			ticker TEXT PRIMARY KEY,
			security_name TEXT
		)
	`)
	if err != nil {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create table: %v", err),
		})
		return
	}

	// Fetch S&P 500 constituents from local file
	stocks, err := fetchSP500List()
	if err != nil {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch S&P 500 list: %v", err),
		})
		return
	}

	// Begin transaction
	tx, err := BacktestDB.Begin()
	if err != nil {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to begin transaction: %v", err),
		})
		return
	}
	defer tx.Rollback()

	// Clear existing data
	_, err = tx.Exec("DELETE FROM sp500_list_2025_jun")
	if err != nil {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to clear existing data: %v", err),
		})
		return
	}

	// Insert new stocks
	stmt, err := tx.Prepare(`
		INSERT INTO sp500_list_2025_jun (ticker, security_name)
		VALUES (?, ?)
	`)
	if err != nil {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to prepare statement: %v", err),
		})
		return
	}
	defer stmt.Close()

	for _, stock := range stocks {
		_, err = stmt.Exec(stock.Symbol, stock.SecurityName)
		if err != nil {
			sendJSONResponse(w, types.HandlerResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to insert stock %s: %v", stock.Symbol, err),
			})
			return
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to commit transaction: %v", err),
		})
		return
	}
}

// stockHandler handles requests for stock data
func stockHandler(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		http.Error(w, "Symbol is required", http.StatusBadRequest)
		return
	}

	// Query the database for stock data
	var stock types.StockData
	err := BacktestDB.QueryRow(`
		SELECT symbol, company_name, price, change_amount, change_percent, 
			   volume, market_cap, previous_close, open_price, high, low, 
			   fifty_two_week_high, fifty_two_week_low, last_updated
		FROM stock_data 
		WHERE symbol = ?
	`, symbol).Scan(
		&stock.Symbol,
		&stock.CompanyName,
		&stock.Price,
		&stock.ChangeAmount,
		&stock.ChangePercent,
		&stock.Volume,
		&stock.MarketCap,
		&stock.PreviousClose,
		&stock.OpenPrice,
		&stock.High,
		&stock.Low,
		&stock.FiftyTwoWeekHigh,
		&stock.FiftyTwoWeekLow,
		&stock.LastUpdated,
	)

	if err == sql.ErrNoRows {
		http.Error(w, "Stock not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, fmt.Sprintf("Database error: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stock)
}

// fillHistoricalDataHandler fills historical data for all stocks
func fillHistoricalDataHandler(w http.ResponseWriter, r *http.Request) {
	// Get list of S&P 500 stocks
	stocks, err := fetchSP500List()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get S&P 500 list: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Starting historical data download for %d S&P 500 stocks", len(stocks))

	// Process each ticker
	completed := 0
	for _, stock := range stocks {
		log.Printf("Processing %s (%d/%d)", stock.Symbol, completed+1, len(stocks))
		
		// Get historical data
		data, err := fetchHistoricalData(stock.Symbol)
		if err != nil {
			log.Printf("Failed to fetch historical data for %s: %v", stock.Symbol, err)
			continue
		}

		// Save to database
		if err := saveHistoricalData(BacktestDB, stock.Symbol, data); err != nil {
			log.Printf("Failed to save historical data for %s: %v", stock.Symbol, err)
			continue
		}
		
		completed++
		log.Printf("Completed %s (%d/%d)", stock.Symbol, completed, len(stocks))
	}

	log.Printf("Historical data download completed. Processed %d out of %d stocks", completed, len(stocks))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "completed",
		"processed": completed,
		"total": len(stocks),
	})
}

// listSP500Handler returns the current list of S&P 500 stocks
func listSP500Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: "Method not allowed",
			Data:    nil,
		})
		return
	}

	stocks, err := fetchSP500List()
	if err != nil {
		sendJSONResponse(w, types.HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch S&P 500 list: %v", err),
			Data:    nil,
		})
		return
	}

	sendJSONResponse(w, types.HandlerResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully retrieved %d S&P 500 stocks", len(stocks)),
		Data:    stocks,
	})
}

// tablesHandler returns list of all database tables
func tablesHandler(w http.ResponseWriter, r *http.Request) {
	// Query to get all table names
	rows, err := BacktestDB.Query(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get tables: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan table name: %v", err), http.StatusInternalServerError)
			return
		}
		tables = append(tables, tableName)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

// tableDataHandler returns paginated data from a specific table
func tableDataHandler(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")
	page := r.URL.Query().Get("page")
	pageSize := r.URL.Query().Get("pageSize")

	if page == "" {
		page = "1"
	}
	if pageSize == "" {
		pageSize = "100"
	}

	pageNum, _ := strconv.Atoi(page)
	pageSizeNum, _ := strconv.Atoi(pageSize)
	offset := (pageNum - 1) * pageSizeNum

	// Validate table name exists
	var exists bool
	err := BacktestDB.QueryRow(`
		SELECT 1 FROM sqlite_master 
		WHERE type='table' AND name=? AND name NOT LIKE 'sqlite_%'
	`, tableName).Scan(&exists)
	if err != nil || !exists {
		http.Error(w, "Table not found", http.StatusNotFound)
		return
	}

	// Execute the query
	rows, err := BacktestDB.Query(fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", tableName, pageSizeNum, offset))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query table: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get columns: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare result
	var result []map[string]interface{}
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		// Scan the result into the values
		if err := rows.Scan(valuePtrs...); err != nil {
			http.Error(w, fmt.Sprintf("Failed to scan row: %v", err), http.StatusInternalServerError)
			return
		}

		// Create a map for this row
		row := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			row[col] = v
		}
		result = append(result, row)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// setupRoutes configures all the application routes
func setupRoutes() *chi.Mux {
	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Home route - render README.md
	r.Get("/", homeHandler)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Stock data routes
		r.Get("/stock/{symbol}", stockHandler)
		r.Get("/stock/historical/{symbol}", historicalDataHandler)
		r.Get("/stock/historical/fill", fillHistoricalDataHandler)

		// Portfolio routes
		r.Get("/portfolio/backtest", portfolioBacktestHandler)

		// S&P 500 routes
		r.Get("/sp500", listSP500Handler)

		// Database browsing routes
		r.Get("/tables", tablesHandler)
		r.Get("/tables/{table}", tableDataHandler)
	})

	// Documentation routes
	r.Get("/readme", readmeHandler)

	return r
}

// sendJSONResponse sends a JSON response with the given HandlerResponse
func sendJSONResponse(w http.ResponseWriter, response types.HandlerResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// fetchSP500List fetches the current S&P 500 constituents from local HTML file
func fetchSP500List() ([]types.SP500Stock, error) {
	// Read the local HTML file
	content, err := os.ReadFile("sp500.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read sp500.html: %v", err)
	}

	// Parse the HTML document
	doc, err := nethtml.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	var stocks []types.SP500Stock
	var f func(*nethtml.Node)
	f = func(n *nethtml.Node) {
		if n.Type == nethtml.ElementNode && n.Data == "table" {
			// Check if this is the S&P 500 table
			for _, a := range n.Attr {
				if a.Key == "id" && a.Val == "constituents" {
					// Found the right table, now parse rows
					var currentStock types.SP500Stock
					var inRow bool
					var colIndex int

					var parseRow func(*nethtml.Node)
					parseRow = func(n *nethtml.Node) {
						if n.Type == nethtml.ElementNode {
							switch n.Data {
							case "tr":
								if n.Parent != nil && n.Parent.Data == "tbody" {
									inRow = true
									colIndex = 0
									currentStock = types.SP500Stock{}
								}
							case "td":
								if !inRow {
									return
								}
								switch colIndex {
								case 0: // Symbol column
									// Find the first anchor tag
									for c := n.FirstChild; c != nil; c = c.NextSibling {
										if c.Type == nethtml.ElementNode && c.Data == "a" {
											if c.FirstChild != nil {
												currentStock.Symbol = strings.TrimSpace(c.FirstChild.Data)
											}
											break
										}
									}
								case 1: // Security Name column
									// Find the first anchor tag
									for c := n.FirstChild; c != nil; c = c.NextSibling {
										if c.Type == nethtml.ElementNode && c.Data == "a" {
											if c.FirstChild != nil {
												currentStock.SecurityName = strings.TrimSpace(c.FirstChild.Data)
											}
											break
										}
									}
									// After getting both columns, add to stocks if valid
									if currentStock.Symbol != "" && currentStock.SecurityName != "" {
										stocks = append(stocks, currentStock)
									}
								}
								colIndex++
							}
						}
						for c := n.FirstChild; c != nil; c = c.NextSibling {
							parseRow(c)
						}
					}

					// Parse all rows in the table
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						parseRow(c)
					}
					return
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	if len(stocks) == 0 {
		return nil, fmt.Errorf("no stocks found in HTML file")
	}

	return stocks, nil
}

// fetchHistoricalData fetches historical data for a given symbol
func fetchHistoricalData(symbol string) ([]types.StockData, error) {
	// Set date range (last 2 years)
	endDate := time.Now()
	startDate := endDate.AddDate(-2, 0, 0)
	
	// Fetch data using the existing function
	data, err := fetchHistoricalTickerData(symbol, startDate, endDate)
	if err != nil {
		return nil, err
	}
	
	// Convert to StockData format
	var stockData []types.StockData
	for _, d := range data {
		stockData = append(stockData, types.StockData{
			Symbol:   d.Symbol,
			Date:     d.Date,
			Open:     d.Open,
			High:     d.High,
			Low:      d.Low,
			Close:    d.Close,
			AdjClose: d.AdjClose,
			Volume:   d.Volume,
		})
	}
	
	return stockData, nil
}

// saveHistoricalData saves historical stock data to the database
func saveHistoricalData(db *sql.DB, symbol string, data []types.StockData) error {
	// Begin transaction
	tx, err := BacktestDB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Prepare statement
	stmt, err := tx.Prepare(`
		INSERT INTO stock_historical_data (
			symbol, date, open, high, low, close, adj_close, volume
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(symbol, date) DO UPDATE SET
			open = excluded.open,
			high = excluded.high,
			low = excluded.low,
			close = excluded.close,
			adj_close = excluded.adj_close,
			volume = excluded.volume
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Insert data
	for _, d := range data {
		_, err = stmt.Exec(
			symbol,
			d.Date.Unix(),
			d.Open,
			d.High,
			d.Low,
			d.Close,
			d.AdjClose,
			d.Volume,
		)
		if err != nil {
			return fmt.Errorf("failed to insert historical data: %v", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// getActiveSP500Tickers returns a list of active S&P 500 tickers from the database
func getActiveSP500Tickers(db *sql.DB) ([]string, error) {
	rows, err := BacktestDB.Query(`
		SELECT DISTINCT symbol 
		FROM stock_historical_data
		ORDER BY symbol
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickers: %v", err)
	}
	defer rows.Close()

	var tickers []string
	for rows.Next() {
		var symbol string
		if err := rows.Scan(&symbol); err != nil {
			return nil, fmt.Errorf("failed to scan ticker: %v", err)
		}
		tickers = append(tickers, symbol)
	}

	return tickers, nil
}

// convertMarkdownToHTML converts markdown content to HTML
func convertMarkdownToHTML(content string) string {
	return string(markdown.ToHTML([]byte(content), nil, nil))
}
