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
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gomarkdown/markdown"
	mdhtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
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
	serverPort  = ":8080"
)

// result represents the result of processing an email
type result struct {
	msg *gmail.Message
	err error
}

// StockData represents stock information for a ticker
type StockData struct {
	Symbol           string    `json:"symbol"`
	CompanyName      string    `json:"companyName"`
	Price            float64   `json:"price"`
	Change           float64   `json:"change"`
	ChangePercent    float64   `json:"changePercent"`
	Volume           int64     `json:"volume"`
	MarketCap        int64     `json:"marketCap"`
	PreviousClose    float64   `json:"previousClose"`
	Open             float64   `json:"open"`
	High             float64   `json:"high"`
	Low              float64   `json:"low"`
	FiftyTwoWeekHigh float64   `json:"fiftyTwoWeekHigh"`
	FiftyTwoWeekLow  float64   `json:"fiftyTwoWeekLow"`
	LastUpdated      time.Time `json:"lastUpdated"`
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

type HandlerResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
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

func main() {
	// Initialize the database
	db, err := initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	dbWrapper := NewDB(db)

	// Set up routes
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/login", handleGoogleLogin)
	http.HandleFunc("/callback", handleGoogleCallback)
	http.HandleFunc("/readme", readmeHandler)
	http.HandleFunc("/batchget", func(w http.ResponseWriter, r *http.Request) {
		batchGetHandler(w, r, dbWrapper)
	})
	http.HandleFunc("/fixdate", func(w http.ResponseWriter, r *http.Request) {
		fixDateHandler(w, r, dbWrapper)
	})
	http.HandleFunc("/api/sp500/update", func(w http.ResponseWriter, r *http.Request) {
		updateSP500Handler(w, r, dbWrapper)
	})
	http.HandleFunc("/api/sp500/list", func(w http.ResponseWriter, r *http.Request) {
		listSP500Handler(w, r, dbWrapper)
	})

	// Start the server
	fmt.Printf("Server is running on port %s\n", serverPort)
	log.Fatal(http.ListenAndServe(serverPort, nil))
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
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	if state != oauthStateString {
		fmt.Printf("Invalid oauth state, expected '%s', got '%s'\n", oauthStateString, state)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	code := r.FormValue("code")
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		fmt.Printf("Code exchange failed with '%s'\n", err)
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	// Save the token
	if err := saveToken(tokenFile, token); err != nil {
		log.Printf("Unable to save token: %v", err)
		http.Error(w, "Failed to save authentication token", http.StatusInternalServerError)
		return
	}

	// Redirect to home page with success message
	http.Redirect(w, r, "/?success=true", http.StatusTemporaryRedirect)
}

// Update getGmailClient to use the stored token
func getGmailClient(ctx context.Context) (*http.Client, error) {
	token, err := tokenFromFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %v", err)
	}

	return config.Client(ctx, token), nil
}

func batchGetHandler(w http.ResponseWriter, r *http.Request, db *DB) {
	// Get Gmail service
	ctx := context.Background()
	client, err := getGmailClient(ctx)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get Gmail client: %v", err),
		})
		return
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create Gmail service: %v", err),
		})
		return
	}

	// Get label ID
	labelID, err := getLabelID(srv, targetLabel)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get label ID: %v", err),
		})
		return
	}

	// Fetch emails
	if err := fetchEmailsWithLabel(srv, labelID, db); err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to fetch emails: %v", err),
		})
		return
	}

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: "Successfully fetched and stored emails",
	})
}

func fixDateHandler(w http.ResponseWriter, r *http.Request, db *DB) {
	// Get Gmail service
	ctx := context.Background()
	client, err := getGmailClient(ctx)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to get Gmail client: %v", err),
		})
		return
	}

	srv, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to create Gmail service: %v", err),
		})
		return
	}

	// Update dates
	if err := updateEmailDates(srv, db); err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to update email dates: %v", err),
		})
		return
	}

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: "Successfully updated email dates",
	})
}

func updateEmailDates(srv *gmail.Service, db *DB) error {
	// Get all message IDs from the database
	rows, err := db.Query("SELECT id FROM emails")
	if err != nil {
		return fmt.Errorf("failed to query emails: %v", err)
	}
	defer rows.Close()

	var messageIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("failed to scan message ID: %v", err)
		}
		messageIDs = append(messageIDs, id)
	}

	// Create a worker pool to process messages
	numWorkers := 10
	jobs := make(chan string, len(messageIDs))
	results := make(chan error, len(messageIDs))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for messageID := range jobs {
				msg, err := srv.Users.Messages.Get("me", messageID).Do()
				if err != nil {
					results <- fmt.Errorf("failed to get message %s: %v", messageID, err)
					continue
				}

				// Update the date in the database using internalDate
				_, err = db.Exec("UPDATE emails SET date = ? WHERE id = ?", msg.InternalDate, messageID)
				if err != nil {
					results <- fmt.Errorf("failed to update date for message %s: %v", messageID, err)
					continue
				}

				results <- nil
			}
		}()
	}

	// Send jobs to workers
	for _, messageID := range messageIDs {
		jobs <- messageID
	}
	close(jobs)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	var errors []error
	var processedCount int
	for err := range results {
		if err != nil {
			errors = append(errors, err)
		} else {
			processedCount++
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors while updating dates: %v", len(errors), errors[0])
	}

	log.Printf("Successfully updated dates for %d messages", processedCount)
	return nil
}

func sendJSONResponse(w http.ResponseWriter, response HandlerResponse) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// extractPortFromRedirectURI extracts the port number from a redirect URI
func extractPortFromRedirectURI(uri string) (int, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return 0, fmt.Errorf("failed to parse URI: %v", err)
	}

	if u.Port() == "" {
		return 0, fmt.Errorf("no port specified in URI")
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return 0, fmt.Errorf("invalid port number: %v", err)
	}

	return port, nil
}

// getTokenFromWeb requests a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%v\n", authURL)

	fmt.Print("Enter the authorization code: ")
	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.Background(), authCode)
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve token from web: %v", err)
	}
	return tok, nil
}

// tokenFromFile retrieves a token from a local file.
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

// saveToken saves a token to a file path.
func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("unable to cache oauth token: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// sanitizeFilename makes a string safe to use as a filename
func sanitizeFilename(name string) string {
	// Replace invalid characters with underscores
	invalid := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	name = invalid.ReplaceAllString(name, "_")

	// Limit length
	if len(name) > 200 {
		name = name[:200]
	}

	return name
}

// fetchEmailsWithLabel retrieves all emails with the specified label
func fetchEmailsWithLabel(srv *gmail.Service, labelID string, db *DB) error {
	messages, err := srv.Users.Messages.List("me").LabelIds(labelID).Do()
	if err != nil {
		return fmt.Errorf("unable to retrieve messages: %v", err)
	}

	if len(messages.Messages) == 0 {
		return fmt.Errorf("no messages found with label")
	}

	// Create channels for concurrent processing
	numWorkers := 10
	jobs := make(chan *gmail.Message, len(messages.Messages))
	results := make(chan result, len(messages.Messages))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msg := range jobs {
				message, err := srv.Users.Messages.Get("me", msg.Id).Do()
				if err != nil {
					results <- result{err: fmt.Errorf("failed to get message %s: %v", msg.Id, err)}
					continue
				}

				// Save directly to database using our DB type
				if err := db.saveEmailToDB(message); err != nil {
					results <- result{err: fmt.Errorf("failed to save message %s: %v", msg.Id, err)}
					continue
				}

				results <- result{msg: message}
			}
		}()
	}

	// Send jobs to workers
	for _, msg := range messages.Messages {
		jobs <- msg
	}
	close(jobs)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	var errors []error
	var processedCount int
	for result := range results {
		if result.err != nil {
			errors = append(errors, result.err)
		} else {
			processedCount++
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered %d errors while processing messages: %v", len(errors), errors[0])
	}

	log.Printf("Successfully processed %d messages", processedCount)
	return nil
}

// parseDate attempts to parse a date string using multiple formats
func parseDate(dateStr string) (time.Time, error) {
	// Common email date formats
	formats := []string{
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 MST",
		time.RFC1123Z,
		time.RFC822Z,
		time.RFC3339,
		"Mon, 2 Jan 2006 15:04:05 GMT",
		"Mon, 02 Jan 2006 15:04:05 GMT",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("could not parse date: %s", dateStr)
}

// extractDateFromHTML attempts to find a date in the HTML content
func extractDateFromHTML(htmlContent string) (time.Time, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return time.Time{}, err
	}

	// Common date patterns in HTML
	datePatterns := []string{
		`\d{1,2}\s+(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\s+\d{4}(?:\s+\d{1,2}:\d{2}(?::\d{2})?)?(?:\s+[+-]\d{4})?`,
		`\d{4}-\d{2}-\d{2}(?:T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?)?`,
		`\d{1,2}/\d{1,2}/\d{4}`,
	}

	var dates []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			for _, pattern := range datePatterns {
				re := regexp.MustCompile(pattern)
				if match := re.FindString(text); match != "" {
					dates = append(dates, match)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)

	// Try to parse each found date
	for _, dateStr := range dates {
		if t, err := parseDate(dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("no valid date found in HTML")
}

// Process headers and extract date in fetchEmailsWithLabel function
func processEmailDate(email *gmail.Message, headers []*gmail.MessagePartHeader, htmlContent string) {
	var dateStr string
	for _, header := range headers {
		if header.Name == "Date" {
			dateStr = header.Value
			break
		}
	}

	// Try to parse the date from header
	if dateStr != "" {
		if parsedDate, err := parseDate(dateStr); err == nil {
			email.InternalDate = parsedDate.Unix()
			return
		}
	}

	// If header date parsing failed, try HTML content
	if htmlContent != "" {
		if parsedDate, err := extractDateFromHTML(htmlContent); err == nil {
			email.InternalDate = parsedDate.Unix()
			return
		}
	}

	// If all else fails, use current time and log warning
	email.InternalDate = time.Now().Unix()
	log.Printf("Warning: Could not parse date for message %s, using current time", email.Id)
}

// Modify the email processing part in fetchEmailsWithLabel
func extractMessageContent(message *gmail.Message) error {
	// Process the date with both header and HTML content
	var htmlContent string
	if message.Payload != nil && message.Payload.Body != nil {
		htmlContent = message.Payload.Body.Data
	}
	processEmailDate(message, message.Payload.Headers, htmlContent)
	return nil
}

func getLabelID(srv *gmail.Service, labelName string) (string, error) {
	labels, err := srv.Users.Labels.List("me").Do()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve labels: %v", err)
	}

	for _, label := range labels.Labels {
		if label.Name == labelName {
			return label.Id, nil
		}
	}

	return "", fmt.Errorf("label '%s' not found", labelName)
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
		return fmt.Errorf("failed to create table: %v", err)
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

	// Create indexes for emails
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_thread_id ON emails(thread_id);
		CREATE INDEX IF NOT EXISTS idx_date ON emails(date);
		CREATE INDEX IF NOT EXISTS idx_subject ON emails(subject);
	`)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %v", err)
	}

	// Create indexes for stock_data
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_stock_symbol ON stock_data(symbol);
		CREATE INDEX IF NOT EXISTS idx_stock_updated ON stock_data(last_updated);
		CREATE INDEX IF NOT EXISTS idx_stock_change_percent ON stock_data(change_percent);
	`)
	if err != nil {
		return fmt.Errorf("failed to create stock indexes: %v", err)
	}

	// Create indexes for stock_historical_data
	_, err = db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_historical_symbol_date ON stock_historical_data(symbol, date);
	`)
	if err != nil {
		return fmt.Errorf("failed to create historical stock indexes: %v", err)
	}

	return nil
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

	client := &http.Client{Timeout: 10 * time.Second}

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
		Change:           change,
		ChangePercent:    changePercent,
		Volume:           meta.RegularMarketVolume,
		MarketCap:        meta.MarketCap,
		PreviousClose:    previousClose,
		Open:             meta.RegularMarketOpen,
		High:             meta.RegularMarketDayHigh,
		Low:              meta.RegularMarketDayLow,
		FiftyTwoWeekHigh: meta.FiftyTwoWeekHigh,
		FiftyTwoWeekLow:  meta.FiftyTwoWeekLow,
		LastUpdated:      time.Now(),
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
		stock.Change,
		stock.ChangePercent,
		stock.Volume,
		stock.MarketCap,
		stock.PreviousClose,
		stock.Open,
		stock.High,
		stock.Low,
		stock.FiftyTwoWeekHigh,
		stock.FiftyTwoWeekLow,
		stock.LastUpdated.Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to insert stock data: %v", err)
	}

	return nil
}

// fetchAllSP500Data fetches data for all S&P 500 tickers using goroutines
func fetchAllSP500Data(db *DB) error {
	// Get ticker list from database
	tickers, err := db.getActiveSP500Tickers()
	if err != nil {
		return fmt.Errorf("failed to get ticker list: %v", err)
	}

	if len(tickers) == 0 {
		return fmt.Errorf("no active tickers found in database")
	}

	numWorkers := 20 // Concurrent goroutines
	jobs := make(chan string, len(tickers))
	results := make(chan StockResult, len(tickers))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ticker := range jobs {
				stockData, err := fetchStockData(ticker)
				if err != nil {
					results <- StockResult{Err: fmt.Errorf("failed to fetch %s: %v", ticker, err)}
					continue
				}

				// Save to database
				if err := db.saveStockData(stockData); err != nil {
					results <- StockResult{Err: fmt.Errorf("failed to save %s: %v", ticker, err)}
					continue
				}

				results <- StockResult{Data: stockData}
			}
		}()
	}

	// Send jobs to workers
	for _, ticker := range tickers {
		jobs <- ticker
	}
	close(jobs)

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Process results
	var errors []error
	var successCount int
	for result := range results {
		if result.Err != nil {
			errors = append(errors, result.Err)
		} else {
			successCount++
		}
	}

	log.Printf("Successfully fetched and saved data for %d out of %d S&P 500 tickers", successCount, len(tickers))

	if len(errors) > 0 {
		log.Printf("Encountered %d errors while fetching stock data", len(errors))
		// Log first few errors
		for i, err := range errors {
			if i >= 5 { // Only log first 5 errors
				break
			}
			log.Printf("Error %d: %v", i+1, err)
		}
	}

	return nil
}

// sp500Handler handles the S&P 500 data fetching endpoint
func sp500Handler(w http.ResponseWriter, r *http.Request, db *DB) {
	log.Printf("Starting S&P 500 data fetch...")

	start := time.Now()
	if err := fetchAllSP500Data(db); err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to fetch S&P 500 data: %v", err),
		})
		return
	}

	duration := time.Since(start)
	message := fmt.Sprintf("Successfully fetched S&P 500 data in %v", duration)
	log.Printf(message)

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: message,
	})
}

// historicalDataHandler handles the historical data fetching endpoint
func historicalDataHandler(w http.ResponseWriter, r *http.Request, db *DB) {
	// 1. Parse query parameters
	tableName := r.URL.Query().Get("table")
	startDateStr := r.URL.Query().Get("start_date")
	endDateStr := r.URL.Query().Get("end_date")
	symbol := r.URL.Query().Get("symbol")

	if tableName == "" || startDateStr == "" || endDateStr == "" {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   "Missing required query parameters: table, start_date, end_date",
		})
		return
	}

	// 2. Parse dates
	layout := "2006-01-02"
	startDate, err := time.Parse(layout, startDateStr)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid start_date format: %v", err),
		})
		return
	}
	endDate, err := time.Parse(layout, endDateStr)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid end_date format: %v", err),
		})
		return
	}

	log.Printf("Starting historical data fetch for table '%s' from %s to %s...", tableName, startDateStr, endDateStr)
	start := time.Now()

	// 3. Fetch historical data
	var fetchErr error
	if symbol != "" {
		// Fetch data for a single symbol
		log.Printf("Fetching historical data for single symbol: %s", symbol)
		data, err := fetchHistoricalTickerData(symbol, startDate, endDate)
		if err != nil {
			sendJSONResponse(w, HandlerResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to fetch historical data for %s: %v", symbol, err),
			})
			return
		}
		fetchErr = db.saveHistoricalData(data)
	} else {
		// Fetch data for all symbols in the table
		fetchErr = fetchAllHistoricalData(db, tableName, startDate, endDate)
	}

	if fetchErr != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to fetch historical data: %v", fetchErr),
		})
		return
	}

	duration := time.Since(start)
	message := fmt.Sprintf("Successfully fetched historical data in %v", duration)
	log.Printf(message)

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: message,
	})
}

// getTickersFromTable fetches ticker symbols from a specified table
func (db *DB) getTickersFromTable(tableName string) ([]string, error) {
	// Sanitize table name to prevent SQL injection
	// A simple approach is to allow only alphanumeric characters and underscores
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, tableName); !matched {
		return nil, fmt.Errorf("invalid table name: %s", tableName)
	}

	query := fmt.Sprintf("SELECT ticker FROM %s WHERE is_active = 1 ORDER BY ticker", tableName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickers from %s: %v", tableName, err)
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

// fetchAllHistoricalData fetches historical data for all tickers from a given table
func fetchAllHistoricalData(db *DB, tableName string, startDate, endDate time.Time) error {
	log.Printf("Starting historical data fetch for table %s from %s to %s", tableName, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

	tickers, err := db.getTickersFromTable(tableName)
	if err != nil {
		return fmt.Errorf("failed to get ticker list: %v", err)
	}

	log.Printf("Found %d tickers in table %s", len(tickers), tableName)
	if len(tickers) == 0 {
		return fmt.Errorf("no active tickers found in table %s", tableName)
	}

	numWorkers := 20
	jobs := make(chan string, len(tickers))
	results := make(chan HistoricalResult, len(tickers))

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ticker := range jobs {
				log.Printf("Fetching historical data for %s", ticker)
				historicalData, err := fetchHistoricalTickerData(ticker, startDate, endDate)
				if err != nil {
					log.Printf("Error fetching historical data for %s: %v", ticker, err)
					results <- HistoricalResult{Ticker: ticker, Err: err}
					continue
				}

				log.Printf("Saving %d data points for %s", len(historicalData), ticker)
				if err := db.saveHistoricalData(historicalData); err != nil {
					log.Printf("Error saving historical data for %s: %v", ticker, err)
					results <- HistoricalResult{Ticker: ticker, Err: err}
					continue
				}

				results <- HistoricalResult{Ticker: ticker, Data: historicalData}
			}
		}()
	}

	for _, ticker := range tickers {
		jobs <- ticker
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	var errors []error
	var successCount int
	for result := range results {
		if result.Err != nil {
			errors = append(errors, result.Err)
		} else {
			successCount++
		}
	}

	log.Printf("Successfully fetched and saved historical data for %d out of %d tickers", successCount, len(tickers))

	if len(errors) > 0 {
		log.Printf("Encountered %d errors while fetching historical data", len(errors))
		for i, err := range errors {
			if i >= 5 {
				break
			}
			log.Printf("Error %d: %v", i+1, err)
		}
	}

	return nil
}

// fetchHistoricalTickerData fetches historical data for a single ticker from Yahoo Finance
func fetchHistoricalTickerData(ticker string, startDate, endDate time.Time) ([]HistoricalData, error) {
	// Yahoo Finance uses Unix timestamps for period1 and period2
	p1 := startDate.Unix()
	p2 := endDate.Unix()

	url := fmt.Sprintf("https://query2.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d&includeAdjustedClose=true", ticker, p1, p2)

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %v", ticker, err)
	}

	// Add required headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Origin", "https://finance.yahoo.com")
	req.Header.Set("Referer", fmt.Sprintf("https://finance.yahoo.com/quote/%s", ticker))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data for %s: %v", ticker, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed for %s: status %d", ticker, resp.StatusCode)
	}

	// Handle gzip compression
	var reader io.Reader
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader for %s: %v", ticker, err)
		}
		defer gzReader.Close()
		reader = gzReader
	default:
		reader = resp.Body
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response for %s: %v", ticker, err)
	}

	// Parse JSON response
	var response struct {
		Chart struct {
			Result []struct {
				Meta struct {
					Symbol string `json:"symbol"`
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
			Error interface{} `json:"error"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse JSON for %s: %v", ticker, err)
	}

	if response.Chart.Error != nil {
		return nil, fmt.Errorf("API error for %s: %v", ticker, response.Chart.Error)
	}

	if len(response.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data returned for %s", ticker)
	}

	result := response.Chart.Result[0]
	if len(result.Timestamp) == 0 {
		return nil, fmt.Errorf("no timestamps returned for %s", ticker)
	}

	if len(result.Indicators.Quote) == 0 || len(result.Indicators.Adjclose) == 0 {
		return nil, fmt.Errorf("no price data returned for %s", ticker)
	}

	quote := result.Indicators.Quote[0]
	adjclose := result.Indicators.Adjclose[0]

	var historicalData []HistoricalData
	for i, ts := range result.Timestamp {
		if i >= len(quote.Open) || i >= len(quote.High) || i >= len(quote.Low) ||
			i >= len(quote.Close) || i >= len(quote.Volume) || i >= len(adjclose.Adjclose) {
			continue
		}

		date := time.Unix(ts, 0)
		historicalData = append(historicalData, HistoricalData{
			Symbol:   ticker,
			Date:     date,
			Open:     quote.Open[i],
			High:     quote.High[i],
			Low:      quote.Low[i],
			Close:    quote.Close[i],
			AdjClose: adjclose.Adjclose[i],
			Volume:   quote.Volume[i],
		})
	}

	return historicalData, nil
}

// saveHistoricalData saves a slice of historical data points to the database
func (db *DB) saveHistoricalData(data []HistoricalData) error {
	if len(data) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}

	stmt, err := tx.Prepare(`
		INSERT OR REPLACE INTO stock_historical_data (
			symbol, date, open, high, low, close, adj_close, volume
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to prepare statement: %v", err)
	}
	defer stmt.Close()

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
			tx.Rollback()
			log.Printf("Error inserting data for %s on %s: %v", d.Symbol, d.Date.Format("2006-01-02"), err)
			return fmt.Errorf("failed to insert historical data for %s on %s: %v", d.Symbol, d.Date.String(), err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

// updateSP500Handler fetches the current S&P 500 list and updates the database
func updateSP500Handler(w http.ResponseWriter, r *http.Request, db *DB) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   "Method not allowed",
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
			Error:   fmt.Sprintf("Failed to create table: %v", err),
		})
		return
	}

	// Fetch S&P 500 constituents from local file
	stocks, err := fetchSP500List()
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to fetch S&P 500 list: %v", err),
		})
		return
	}

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to begin transaction: %v", err),
		})
		return
	}
	defer tx.Rollback()

	// Clear existing data
	_, err = tx.Exec("DELETE FROM sp500_list_2025_jun")
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to clear existing data: %v", err),
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
			Error:   fmt.Sprintf("Failed to prepare statement: %v", err),
		})
		return
	}
	defer stmt.Close()

	for _, stock := range stocks {
		_, err = stmt.Exec(stock.Symbol, stock.SecurityName)
		if err != nil {
			sendJSONResponse(w, HandlerResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to insert stock %s: %v", stock.Symbol, err),
			})
			return
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to commit transaction: %v", err),
		})
		return
	}

	sendJSONResponse(w, HandlerResponse{
		Success: true,
		Message: fmt.Sprintf("Successfully updated %d S&P 500 stocks", len(stocks)),
	})
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
func listSP500Handler(w http.ResponseWriter, r *http.Request, db *DB) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	rows, err := db.Query(`
		SELECT ticker
		FROM sp500_list_2025_jun
		ORDER BY ticker
	`)
	if err != nil {
		sendJSONResponse(w, HandlerResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to query database: %v", err),
		})
		return
	}
	defer rows.Close()

	var stocks []SP500Stock
	for rows.Next() {
		var stock SP500Stock
		err := rows.Scan(&stock.Symbol)
		if err != nil {
			sendJSONResponse(w, HandlerResponse{
				Success: false,
				Error:   fmt.Sprintf("Failed to scan row: %v", err),
			})
			return
		}
		stocks = append(stocks, stock)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stocks)
}

func readmeHandler(w http.ResponseWriter, r *http.Request) {
	// Read the README.md file
	content, err := os.ReadFile("README.md")
	if err != nil {
		http.Error(w, "Failed to read README.md", http.StatusInternalServerError)
		return
	}

	// Convert markdown to HTML
	html := markdownToHTML(content)

	// Add CSS styling
	styledHTML := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<meta charset="UTF-8">
			<meta name="viewport" content="width=device-width, initial-scale=1.0">
			<title>API Documentation</title>
			<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/github-markdown-css/5.2.0/github-markdown.min.css">
			<style>
				.markdown-body {
					box-sizing: border-box;
					min-width: 200px;
					max-width: 980px;
					margin: 0 auto;
					padding: 45px;
				}
				@media (max-width: 767px) {
					.markdown-body {
						padding: 15px;
					}
				}
				body {
					background-color: #f6f8fa;
				}
				pre {
					background-color: #f6f8fa;
					border-radius: 6px;
					padding: 16px;
					overflow: auto;
				}
				code {
					background-color: rgba(175,184,193,0.2);
					border-radius: 6px;
					padding: 0.2em 0.4em;
					font-size: 85%%;
				}
			</style>
		</head>
		<body>
			<article class="markdown-body">
				%s
			</article>
		</body>
		</html>
	`, html)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(styledHTML))
}

func markdownToHTML(md []byte) string {
	// Create a markdown parser with extensions
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)

	// Parse the markdown content
	parsedContent := p.Parse(md)

	// Create HTML renderer with default options
	htmlFlags := mdhtml.CommonFlags | mdhtml.HrefTargetBlank
	opts := mdhtml.RendererOptions{Flags: htmlFlags}
	renderer := mdhtml.NewRenderer(opts)

	// Convert to HTML
	html := markdown.Render(parsedContent, renderer)
	return string(html)
}
