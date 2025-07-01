package main

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/darianmavgo/backtest-sell-limit/pkg/types"
)

func TestFetchSP500List(t *testing.T) {
	// Skip this test for now - it requires sp500.html file
	t.Skip("Skipping SP500 list test - requires sp500.html file")
}

func TestBacktestTSLA(t *testing.T) {
	// Skip this test for now - it requires database setup and Python environment
	t.Skip("Skipping TSLA backtest test - requires full environment setup")
}

func TestConvertMarkdownToHTML(t *testing.T) {
	// Test basic markdown conversion
	input := "# Hello World\n\nThis is a **test**."
	output := convertMarkdownToHTML(input)
	
	if output == "" {
		t.Error("Expected non-empty HTML output")
	}
	
	// Check that it contains basic HTML elements
	if !strings.Contains(output, "<h1>") {
		t.Error("Expected HTML to contain h1 tag")
	}
	
	if !strings.Contains(output, "<strong>") {
		t.Error("Expected HTML to contain strong tag")
	}
	
	t.Logf("Converted markdown to HTML: %s", output)
}

func TestTypesStructs(t *testing.T) {
	// Test that our types can be created
	stock := types.StockData{
		Symbol:      "TEST",
		CompanyName: "Test Company",
		Price:       100.0,
	}
	
	if stock.Symbol != "TEST" {
		t.Errorf("Expected symbol 'TEST', got %s", stock.Symbol)
	}
	
	if stock.Price != 100.0 {
		t.Errorf("Expected price 100.0, got %f", stock.Price)
	}
	
	// Test SP500Stock struct
	sp500Stock := types.SP500Stock{
		Symbol:       "AAPL",
		SecurityName: "Apple Inc.",
	}
	
	if sp500Stock.Symbol != "AAPL" {
		t.Errorf("Expected symbol 'AAPL', got %s", sp500Stock.Symbol)
	}
}


func verifyBacktestData(t *testing.T, db *sql.DB) {
	// Check stock_historical_data table
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM stock_historical_data WHERE symbol = 'TSLA'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query stock_historical_data: %v", err)
	}
	if count == 0 {
		t.Error("Expected TSLA data in stock_historical_data table, but found none")
	} else {
		t.Logf("Found %d records for TSLA in stock_historical_data table", count)
	}

	// Check logs table (if exists)
	var logCount int
	err = db.QueryRow("SELECT COUNT(*) FROM logs").Scan(&logCount)
	if err != nil {
		// Table might not exist, which is okay
		t.Logf("No logs table found or error querying: %v", err)
	} else {
		t.Logf("Found %d log entries", logCount)
	}

	// Check backtest_strategies table (if exists)
	var strategyCount int
	err = db.QueryRow("SELECT COUNT(*) FROM backtest_strategies").Scan(&strategyCount)
	if err != nil {
		// Table might not exist, which is okay
		t.Logf("No backtest_strategies table found or error querying: %v", err)
	} else {
		t.Logf("Found %d strategy entries", strategyCount)
	}

	// Check backtest_daily_values table (if exists)
	var dailyCount int
	err = db.QueryRow("SELECT COUNT(*) FROM backtest_daily_values").Scan(&dailyCount)
	if err != nil {
		// Table might not exist, which is okay
		t.Logf("No backtest_daily_values table found or error querying: %v", err)
	} else {
		t.Logf("Found %d daily value entries", dailyCount)
	}
}
