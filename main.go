package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/tls"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/html"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	credentialsFile = "/Users/darianhickman/Documents/Github/backteststoxx/client_secret_2_914016029840-24qpupahd54i01jt8kfvalmj2114kbh9.apps.googleusercontent.com.json"
	// credentialsFile = "client_secret_914016029840-24qpupahd54i01jt8kfvalmj2114kbh9.apps.googleusercontent.com.json"
	tokenDir    = ".credentials"
	tokenFile   = ".credentials/token.json"
	dbFile      = "backtest_sell_limits.db"
	targetLabel = "backteststoxx"
	serverPort  = "8080"
)

// result represents the result of processing an email
type result struct {
	msg *gmail.Message
	err error
}

// StockData represents stock information from the database
type StockData struct {
	Symbol           string    `json:"symbol"`
	CompanyName      string    `json:"company_name"`
	Price            float64   `json:"price"`
	ChangeAmount     float64   `json:"change_amount"`
	ChangePercent    float64   `json:"change_percent"`
	Volume           int64     `json:"volume"`
	MarketCap        int64     `json:"market_cap"`
	PreviousClose    float64   `json:"previous_close"`
	OpenPrice        float64   `json:"open_price"`
	High             float64   `json:"high"`
	Low              float64   `json:"low"`
	FiftyTwoWeekHigh float64   `json:"fifty_two_week_high"`
	FiftyTwoWeekLow  float64   `json:"fifty_two_week_low"`
	LastUpdated      int64     `json:"last_updated"`
	Date             time.Time `json:"date"`
	Open             float64   `json:"open"`
	Close            float64   `json:"close"`
	AdjClose         float64   `json:"adj_close"`
}

// StockResult represents the result of fetching stock data
type StockResult struct {
	Data *StockData
	Err  error
}

// HistoricalData represents a single day of stock data
type HistoricalData struct {
	Symbol   string    `json:"symbol"`
	Date     time.Time `json:"date"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	AdjClose float64   `json:"adjClose"`
	Volume   int64     `json:"volume"`
}

// HistoricalResult represents the result of fetching historical data for a ticker
type HistoricalResult struct {
	Ticker string
	Data   []HistoricalData
	Err    error
}

// CredentialInfo stores the raw credential file information
type CredentialInfo struct {
	Web struct {
		ClientID                string   `json:"client_id"`
		ProjectID               string   `json:"project_id"`
		AuthURI                 string   `json:"auth_uri"`
		TokenURI                string   `json:"token_uri"`
		AuthProviderX509CertURL string   `json:"auth_provider_x509_cert_url"`
		ClientSecret            string   `json:"client_secret"`
		RedirectURIs            []string `json:"redirect_uris"`
		JavascriptOrigins       []string `json:"javascript_origins,omitempty"`
	} `json:"web"`
}

// OAuthClientInfo stores detailed information about the OAuth client and token
type OAuthClientInfo struct {
	ClientID        string    `json:"client_id"`
	ClientSecret    string    `json:"client_secret"`
	RedirectURI     string    `json:"redirect_uri"`
	AuthURL         string    `json:"auth_url"`
	TokenURL        string    `json:"token_url"`
	Scopes          []string  `json:"scopes"`
	TokenType       string    `json:"token_type"`
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token"`
	Expiry          time.Time `json:"expiry"`
	UserEmail       string    `json:"user_email"`
	UserID          string    `json:"user_id"`
	VerifiedEmail   bool      `json:"verified_email"`
	Picture         string    `json:"picture"`
	Locale          string    `json:"locale"`
	LastRefreshTime time.Time `json:"last_refresh_time"`
	TokenSource     string    `json:"token_source"`
	ApplicationName string    `json:"application_name"`
	ProjectID       string    `json:"project_id"`
}

// DB represents our database connection
type DB struct {
	*sql.DB
}

func NewDB(db *sql.DB) *DB {
	return &DB{DB: db}
}

// printCredentialInfo reads and prints all available information from the credentials file
func printCredentialInfo(credBytes []byte) (*CredentialInfo, error) {
	var credInfo CredentialInfo
	if err := json.Unmarshal(credBytes, &credInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %v", err)
	}

	fmt.Printf("\n=== Credential Information ===\n")
	fmt.Printf("Project ID: %s\n", credInfo.Web.ProjectID)
	fmt.Printf("Client ID: %s\n", credInfo.Web.ClientID)
	fmt.Printf("Auth URI: %s\n", credInfo.Web.AuthURI)
	fmt.Printf("Token URI: %s\n", credInfo.Web.TokenURI)
	fmt.Printf("Redirect URIs: %v\n", credInfo.Web.RedirectURIs)

	return &credInfo, nil
}

// ensureTokenDir creates the token directory if it doesn't exist
func ensureTokenDir() error {
	tokenDir := "token"
	if _, err := os.Stat(tokenDir); os.IsNotExist(err) {
		return os.MkdirAll(tokenDir, 0700)
	}
	return nil
}

// initDB initializes the SQLite database with WAL mode and creates the schema
func initDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbFile+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create the emails table with the new schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS emails (
			id TEXT PRIMARY KEY,
			thread_id TEXT,
			subject TEXT,
			from_address TEXT,
			to_address TEXT,
			date INTEGER,
			plain_text TEXT,
			html TEXT,
			label_ids TEXT,
			UNIQUE(id)
		)
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create table: %v", err)
	}

	// Create indexes
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_thread_id ON emails(thread_id);
		CREATE INDEX IF NOT EXISTS idx_date ON emails(date);
		CREATE INDEX IF NOT EXISTS idx_subject ON emails(subject);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %v", err)
	}

	// Create the stock_data table for S&P 500 tickers
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS stock_data (
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
		return nil, fmt.Errorf("failed to create stock_data table: %v", err)
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
		return nil, fmt.Errorf("failed to create stock_historical_data table: %v", err)
	}

	// Create indexes for emails
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_thread_id ON emails(thread_id);
		CREATE INDEX IF NOT EXISTS idx_date ON emails(date);
		CREATE INDEX IF NOT EXISTS idx_subject ON emails(subject);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create indexes: %v", err)
	}

	// Create indexes for stock_data
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_stock_symbol ON stock_data(symbol);
		CREATE INDEX IF NOT EXISTS idx_stock_updated ON stock_data(last_updated);
		CREATE INDEX IF NOT EXISTS idx_stock_change_percent ON stock_data(change_percent);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create stock indexes: %v", err)
	}

	// Create indexes for stock_historical_data
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_historical_symbol_date ON stock_historical_data(symbol, date);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create historical stock indexes: %v", err)
	}

	return db, nil
}

// saveEmailToDB saves an email to the SQLite database
func (db *DB) saveEmailToDB(msg *gmail.Message) error {
	// Extract headers
	var subject, from, to string
	for _, header := range msg.Payload.Headers {
		switch header.Name {
		case "Subject":
			subject = header.Value
		case "From":
			from = header.Value
		case "To":
			to = header.Value
		}
	}

	// Extract content
	var plainText, html string
	var processPayload func(*gmail.MessagePart) error
	processPayload = func(part *gmail.MessagePart) error {
		if part.MimeType == "text/plain" {
			if part.Body != nil && part.Body.Data != "" {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					return fmt.Errorf("failed to decode plain text: %v", err)
				}
				plainText = string(data)
			}
		} else if part.MimeType == "text/html" {
			if part.Body != nil && part.Body.Data != "" {
				data, err := base64.URLEncoding.DecodeString(part.Body.Data)
				if err != nil {
					return fmt.Errorf("failed to decode HTML: %v", err)
				}
				html = string(data)
			}
		}

		if part.Parts != nil {
			for _, subPart := range part.Parts {
				if err := processPayload(subPart); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := processPayload(msg.Payload); err != nil {
		return fmt.Errorf("failed to process payload: %v", err)
	}

	// Save to database
	stmt, err := db.Prepare(`
		INSERT INTO emails (id, thread_id, subject, from_address, to_address, date, plain_text, html, label_ids)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		msg.Id,
		msg.ThreadId,
		subject,
		from,
		to,
		msg.InternalDate,
		plainText,
		html,
		strings.Join(msg.LabelIds, ","),
	)
	if err != nil {
		return fmt.Errorf("failed to insert email: %v", err)
	}

	return nil
}

// HandlerResponse represents a standardized API response
type HandlerResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OAuth configuration
var (
	oauthStateString = "random-state-string" // In production, generate this randomly per session
	config           *oauth2.Config
)

// SP500Stock represents a stock in the S&P 500 index
type SP500Stock struct {
	Symbol       string `json:"symbol"`
	SecurityName string `json:"security_name"`
}

// StreamingLogWriter is a writer that streams logs to an HTTP response
type StreamingLogWriter struct {
	w  http.ResponseWriter
	f  http.Flusher
	mu sync.Mutex
}

func NewStreamingLogWriter(w http.ResponseWriter) *StreamingLogWriter {
	f, _ := w.(http.Flusher)
	return &StreamingLogWriter{
		w: w,
		f: f,
	}
}

func (s *StreamingLogWriter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Write the log line
	n, err = s.w.Write(p)
	if err != nil {
		return n, err
	}

	// Flush if we have a flusher
	if s.f != nil {
		s.f.Flush()
	}

	return n, nil
}

// portfolioBacktestHandler runs the portfolio backtest and streams the output
func portfolioBacktestHandler(w http.ResponseWriter, r *http.Request) {
	// Get list of active tickers
	tickers, err := getActiveSP500Tickers(db)
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
		if err := saveHistoricalData(db, symbol, data); err != nil {
			log.Printf("Failed to save historical data for %s: %v", symbol, err)
			continue
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "completed",
	})
}

// Global database connection
var db *sql.DB

func init() {
	// Initialize database
	var err error
	db, err = sql.Open("sqlite3", dbFile+"?_journal_mode=WAL")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Create tables
	if err := createTables(db); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}
}

func createTables(db *sql.DB) error {
	// Create the emails table with the new schema
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS emails (
			id TEXT PRIMARY KEY,
			thread_id TEXT,
			subject TEXT,
			from_address TEXT,
			to_address TEXT,
			date INTEGER,
			plain_text TEXT,
			html TEXT,
			label_ids TEXT,
			UNIQUE(id)
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create emails table: %v", err)
	}

	// Create the stock_data table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS stock_data (
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

	// Create indexes
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_thread_id ON emails(thread_id);
		CREATE INDEX IF NOT EXISTS idx_date ON emails(date);
		CREATE INDEX IF NOT EXISTS idx_subject ON emails(subject);
		CREATE INDEX IF NOT EXISTS idx_stock_symbol ON stock_data(symbol);
		CREATE INDEX IF NOT EXISTS idx_stock_updated ON stock_data(last_updated);
		CREATE INDEX IF NOT EXISTS idx_stock_change_percent ON stock_data(change_percent);
		CREATE INDEX IF NOT EXISTS idx_historical_symbol_date ON stock_historical_data(symbol, date);
	`)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %v", err)
	}

	return nil
}

func main() {
	// Create router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Routes
	r.Get("/api/stock/{symbol}", stockHandler)
	r.Get("/api/stock/historical/{symbol}", historicalDataHandler)
	r.Get("/api/stock/historical/fill", fillHistoricalDataHandler)
	r.Get("/api/portfolio/backtest", portfolioBacktestHandler)

	// Database browsing routes
	r.Get("/api/tables", func(w http.ResponseWriter, r *http.Request) {
		// Query to get all table names
		rows, err := db.Query(`
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
	})

	r.Get("/api/tables/{table}", func(w http.ResponseWriter, r *http.Request) {
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
		err := db.QueryRow(`
			SELECT 1 FROM sqlite_master 
			WHERE type='table' AND name=? AND name NOT LIKE 'sqlite_%'
		`, tableName).Scan(&exists)
		if err != nil || !exists {
			http.Error(w, "Table not found", http.StatusNotFound)
			return
		}

		// Execute the query
		rows, err := db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", tableName, pageSizeNum, offset))
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
	})

	// Start the server
	fmt.Printf("Server is running on port %s\n", serverPort)
	log.Fatal(http.ListenAndServe(":"+serverPort, r))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	var html = `
	<html>
		<head>
			<title>Gmail Backtest Email Processor</title>
			<style>
				body { 
					font-family: Arial, sans-serif; 
					margin: 40px auto;
					max-width: 800px;
					padding: 20px;
				}
				.button {
					display: inline-block;
					padding: 10px 20px;
					background-color: #4285f4;
					color: white;
					text-decoration: none;
					border-radius: 4px;
					margin: 10px 0;
				}
				.button:hover {
					background-color: #357abd;
				}
				.button.stock {
					background-color: #34a853;
				}
				.button.stock:hover {
					background-color: #2d8f45;
				}
			</style>
		</head>
		<body>
			<h1>Gmail Backtest Email Processor</h1>
			<p>Process your backtest emails from Gmail and fetch S&P 500 stock data.</p>
			<div>
				<a href="/login" class="button">Login with Google</a>
			</div>
			<div id="actions" style="display:none;">
				<a href="/batchget" class="button">Fetch Emails</a>
				<a href="/fixdate" class="button">Fix Dates</a>
			</div>
			<div>
				<h2>Stock Data</h2>
				<a href="/sp500" class="button stock">Fetch S&P 500 Data</a>
				<p><small>Fetches current stock data for all S&P 500 companies using concurrent goroutines.</small></p>
			</div>
			<div>
				<h2>Historical Stock Data</h2>
				<a href="/historical-data?table=sp500_list_2025_jun&start_date=2023-01-01&end_date=2023-12-31" class="button stock">Fetch Historical Data (Example)</a>
				<p><small>Fetches historical stock data for a given table, start date, and end date.</small></p>
			</div>
		</body>
	</html>
	`
	fmt.Fprint(w, html)
}

func handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	url := config.AuthCodeURL(oauthStateString)
	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: "Authorization URL generated",
		Data:    url,
	})
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	if state != oauthStateString {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: "Invalid OAuth state",
			Data:    nil,
		})
		return
	}

	code := r.URL.Query().Get("code")
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to exchange token: %v", err),
			Data:    nil,
		})
		return
	}

	client := config.Client(context.Background(), token)
	srv, err := gmail.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create Gmail client: %v", err),
			Data:    nil,
		})
		return
	}

	// Get user profile
	user, err := srv.Users.GetProfile("me").Do()
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get user profile: %v", err),
			Data:    nil,
		})
		return
	}

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: "Successfully authenticated",
		Data: map[string]interface{}{
			"email": user.EmailAddress,
			"token": token,
		},
	})
}

func batchGetHandler(w http.ResponseWriter, r *http.Request) {
	labelName := r.URL.Query().Get("label")
	if labelName == "" {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: "Label name is required",
			Data:    nil,
		})
		return
	}

	// Get Gmail service
	ctx := context.Background()
	client, err := getGmailClient(ctx)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get Gmail client: %v", err),
			Data:    nil,
		})
		return
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create Gmail service: %v", err),
			Data:    nil,
		})
		return
	}

	// Get label ID
	labelID, err := getLabelID(srv, labelName)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get label ID: %v", err),
			Data:    nil,
		})
		return
	}

	// Get messages
	messages, err := getMessagesWithLabel(srv, labelID)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to get messages: %v", err),
			Data:    nil,
		})
		return
	}

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully retrieved %d messages", len(messages)),
		Data:    messages,
	})
}

func fixDateHandler(w http.ResponseWriter, r *http.Request, db *DB) {
	rows, err := db.Query("SELECT id, date FROM emails")
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to query emails: %v", err),
			Data:    nil,
		})
		return
	}
	defer rows.Close()

	var updates int
	for rows.Next() {
		var id string
		var dateStr string
		if err := rows.Scan(&id, &dateStr); err != nil {
			sendJSONResponse(w, HandlerResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to scan row: %v", err),
				Data:    nil,
			})
			return
		}

		// Parse and fix date
		date, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			continue
		}

		// Update the record
		_, err = db.Exec("UPDATE emails SET date = ? WHERE id = ?", date.Unix(), id)
		if err != nil {
			continue
		}
		updates++
	}

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully updated %d dates", updates),
		Data:    updates,
	})
}

func sp500Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: "Method not allowed",
			Data:    nil,
		})
		return
	}

	stocks, err := fetchSP500List()
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
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

		if err := saveHistoricalData(db, stock.Symbol, data); err != nil {
			log.Printf("Failed to save historical data for %s: %v", stock.Symbol, err)
			continue
		}
	}

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully updated %d S&P 500 stocks", len(stocks)),
		Data:    stocks,
	})
}

// Update getGmailClient to use the stored token
func getGmailClient(ctx context.Context) (*http.Client, error) {
	token, err := tokenFromFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", err)
	}

	return config.Client(ctx, token), nil
}

func getLabelID(srv *gmail.Service, labelName string) (string, error) {
	labels, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return "", fmt.Errorf("failed to list labels: %v", err)
	}

	for _, label := range labels.Labels {
		if strings.EqualFold(label.Name, labelName) {
			return label.Id, nil
		}
	}

	return "", fmt.Errorf("label '%s' not found", labelName)
}

func getMessagesWithLabel(srv *gmail.Service, labelID string) ([]*gmail.Message, error) {
	var messages []*gmail.Message
	pageToken := ""
	for {
		req := srv.Users.Messages.List("me").LabelIds(labelID)
		if pageToken != "" {
			req.PageToken(pageToken)
		}
		r, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("failed to list messages: %v", err)
		}
		messages = append(messages, r.Messages...)
		if r.NextPageToken == "" {
			break
		}
		pageToken = r.NextPageToken
	}
	return messages, nil
}

func processEmail(message *gmail.Message) (*gmail.Message, error) {
	if err := extractMessageContent(message); err != nil {
		return nil, fmt.Errorf("failed to extract content: %v", err)
	}
	return message, nil
}

// getActiveSP500Tickers fetches active ticker symbols from the database
func (db *DB) getActiveSP500Tickers() ([]string, error) {
	query := "SELECT ticker FROM sp500_list_2025_jun WHERE is_active = 1 ORDER BY ticker"
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickers: %v", err)
	}
	defer rows.Close()

	var tickers []string
	for rows.Next() {
		var ticker string
		if err := rows.Scan(&ticker); err != nil {
			return nil, fmt.Errorf("failed to scan ticker: %v", err)
		}
		tickers = append(tickers, ticker)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %v", err)
	}

	return tickers, nil
}

// fetchStockData fetches stock data for a given ticker using a free API
func fetchStockData(ticker string) (*StockData, error) {
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

	return &StockData{
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
func (db *DB) saveStockData(stock *StockData) error {
	stmt, err := db.Prepare(`
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
	tickers, err := getActiveSP500Tickers(db)
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
		if err := saveHistoricalData(db, symbol, data); err != nil {
			log.Printf("Failed to save historical data for %s: %v", symbol, err)
			continue
		}
	}

	return nil
}

// historicalDataHandler handles requests for historical stock data
func historicalDataHandler(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		http.Error(w, "Symbol is required", http.StatusBadRequest)
		return
	}

	// Query the database for historical data
	rows, err := db.Query(`
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

	var data []StockData
	for rows.Next() {
		var d StockData
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

// fetchHistoricalTickerData fetches historical data for a single ticker from Yahoo Finance
func fetchHistoricalTickerData(ticker string, startDate, endDate time.Time) ([]HistoricalData, error) {
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

	var historicalData []HistoricalData
	for i, ts := range result.Timestamp {
		if i >= len(quote.Open) || i >= len(quote.High) || i >= len(quote.Low) ||
			i >= len(quote.Close) || i >= len(quote.Volume) || i >= len(adjclose.Adjclose) {
			continue
		}

		historicalData = append(historicalData, HistoricalData{
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

// saveHistoricalData saves a slice of historical data points to the database
func (db *DB) saveHistoricalData(data []HistoricalData) error {
	if len(data) == 0 {
		return nil
	}

	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback() // Will be ignored if transaction is committed

	// Prepare the statement
	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO stock_historical_data (
			symbol, date, open, high, low, close, adj_close, volume
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

	// Insert all data points in a single transaction
	for _, d := range data {
		_, err := stmt.Exec(
			d.Symbol,
			d.Date.Unix(),
			d.Open,
			d.High,
			d.Low,
			d.Close,
			d.AdjClose,
			d.Volume,
		)
		if err != nil {
			return fmt.Errorf("failed to insert historical data for %s on %s: %v", d.Symbol, d.Date.String(), err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// updateSP500Handler fetches the current S&P 500 list and updates the database
func updateSP500Handler(w http.ResponseWriter, r *http.Request, db *DB) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: "Method not allowed",
		})
		return
	}

	// Create the table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS sp500_list_2025_jun (
			ticker TEXT PRIMARY KEY,
			security_name TEXT
		)
	`)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to create table: %v", err),
		})
		return
	}

	// Fetch S&P 500 constituents from local file
	stocks, err := fetchSP500List()
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch S&P 500 list: %v", err),
		})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to begin transaction: %v", err),
		})
		return
	}
	defer tx.Rollback()

	// Clear existing data
	_, err = tx.Exec("DELETE FROM sp500_list_2025_jun")
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
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
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to prepare statement: %v", err),
		})
		return
	}
	defer stmt.Close()

	for _, stock := range stocks {
		_, err = stmt.Exec(stock.Symbol, stock.SecurityName)
		if err != nil {
			sendJSONResponse(w, HandlerResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to insert stock %s: %v", stock.Symbol, err),
			})
			return
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		sendJSONResponse(w, HandlerResponse{
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
	var stock StockData
	err := db.QueryRow(`
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
	// Get list of active tickers
	tickers, err := getActiveSP500Tickers(db)
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
		if err := saveHistoricalData(db, symbol, data); err != nil {
			log.Printf("Failed to save historical data for %s: %v", symbol, err)
			continue
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "completed",
	})
}

// fetchHistoricalData fetches historical data for a given symbol
func fetchHistoricalData(symbol string) ([]StockData, error) {
	// Implement the actual data fetching logic here
	// For now, return empty data
	return []StockData{}, nil
}

// saveHistoricalData saves historical stock data to the database
func saveHistoricalData(db *sql.DB, symbol string, data []StockData) error {
	// Begin transaction
	tx, err := db.Begin()
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
	rows, err := db.Query(`
		SELECT symbol 
		FROM stock_data 
		WHERE symbol IN (
			SELECT DISTINCT symbol 
			FROM stock_historical_data
		)
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

// sendJSONResponse sends a JSON response with the given HandlerResponse
func sendJSONResponse(w http.ResponseWriter, response HandlerResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// tokenFromFile retrieves a token from a local file
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// extractMessageContent extracts content from a Gmail message
func extractMessageContent(message *gmail.Message) error {
	if message.Payload == nil {
		return fmt.Errorf("message payload is nil")
	}

	// Process headers
	for _, header := range message.Payload.Headers {
		switch header.Name {
		case "Subject":
			message.Payload.Headers = append(message.Payload.Headers, &gmail.MessagePartHeader{
				Name:  "Subject",
				Value: header.Value,
			})
		case "From":
			message.Payload.Headers = append(message.Payload.Headers, &gmail.MessagePartHeader{
				Name:  "From",
				Value: header.Value,
			})
		}
	}

	return nil
}

// fetchSP500List fetches the current S&P 500 constituents from local HTML file
func fetchSP500List() ([]SP500Stock, error) {
	// Read the local HTML file
	content, err := os.ReadFile("sp500.html")
	if err != nil {
		return nil, fmt.Errorf("failed to read sp500.html: %v", err)
	}

	// Parse the HTML document
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	var stocks []SP500Stock
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			// Check if this is the S&P 500 table
			for _, a := range n.Attr {
				if a.Key == "id" && a.Val == "constituents" {
					// Found the right table, now parse rows
					var currentStock SP500Stock
					var inRow bool
					var colIndex int

					var parseRow func(*html.Node)
					parseRow = func(n *html.Node) {
						if n.Type == html.ElementNode {
							switch n.Data {
							case "tr":
								if n.Parent != nil && n.Parent.Data == "tbody" {
									inRow = true
									colIndex = 0
									currentStock = SP500Stock{}
								}
							case "td":
								if !inRow {
									return
								}
								switch colIndex {
								case 0: // Symbol column
									// Find the first anchor tag
									for c := n.FirstChild; c != nil; c = c.NextSibling {
										if c.Type == html.ElementNode && c.Data == "a" {
											if c.FirstChild != nil {
												currentStock.Symbol = strings.TrimSpace(c.FirstChild.Data)
											}
											break
										}
									}
								case 1: // Security Name column
									// Find the first anchor tag
									for c := n.FirstChild; c != nil; c = c.NextSibling {
										if c.Type == html.ElementNode && c.Data == "a" {
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

// listSP500Handler returns the current list of S&P 500 stocks
func listSP500Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: "Method not allowed",
			Data:    nil,
		})
		return
	}

	stocks, err := fetchSP500List()
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch S&P 500 list: %v", err),
			Data:    nil,
		})
		return
	}

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully retrieved %d S&P 500 stocks", len(stocks)),
		Data:    stocks,
	})
}
