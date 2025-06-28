package main

import (
	"testing"
)

func TestFetchSP500List(t *testing.T) {
	// Fetch the S&P 500 list
	stocks, err := fetchSP500List()
	if err != nil {
		t.Fatalf("Failed to fetch S&P 500 list: %v", err)
	}

	// Verify we got some stocks
	if len(stocks) == 0 {
		t.Error("Expected non-empty stock list, got empty list")
		return
	}

	t.Logf("Found %d stocks", len(stocks))

	// Print first few stocks for debugging
	t.Log("First 10 stocks found:")
	for i := 0; i < len(stocks) && i < 10; i++ {
		t.Logf("%d. Symbol: %s, Name: %s", i+1, stocks[i].Symbol, stocks[i].SecurityName)
	}

	// Look for TSLA in the list
	var found bool
	for _, stock := range stocks {
		if stock.Symbol == "TSLA" {
			found = true
			t.Logf("Found TSLA with security name: %s", stock.SecurityName)
			break
		}
	}

	if !found {
		t.Error("Expected to find TSLA in S&P 500 list, but it was not found")
	}
}
