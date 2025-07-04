package types

import (
	"database/sql"
	"net/http"
	"sync"
	"time"
)

type Config struct {
	// need to migrate most of this to Credential struct
	ENV                string // DEV, Prod, Local, Hosted
	TopLevelDir        string // Top level directory of the application.
	BacktestDB         string // Application Support App settings like store of credentials, known connections.
	SPXLBacktestDB     string // New field for SPXL specific database
	ServiceAccountJson string // Need to move ServiceAccountJson to credential struct.
	Port               string // Config.Port is the port that Mavgo Flight service binds to.  Do not confuse with port of a request.
	TopCacheDir        string // Remote files and local files cached as sqlite land in this folder
	DefaultFormat      string // I have no idea.  Need to track where this is used.
	ServeFolder        string // I supersetted/wrapped/inherited http.FileServer as starting point of FlightHandler. ServeFolder is the folder it starts for serving.
	PrivateKeyPath     string // Need to move PrivateKeyPath to Credential struct.
	ProjectID          string // Until I create a better solution assuming that Mavgo Flight is serving data from services tied to one single Google Cloud project 	// I created this variable to enable NewClient for bigquery July 27 2024.
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

// DB represents our database connection
type DB struct {
	*sql.DB
}

// HandlerResponse represents a standardized API response
type HandlerResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

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
