package main

import (
	"fmt"
	"testing"
	"time"
)

func TestMempoolPriceAPI(t *testing.T) {
	api := NewMempoolPriceAPI()

	// Test with a known date (January 1, 2025)
	testDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	expectedPrice := 93318.00

	price, err := api.GetBTCPriceUSD(testDate)
	if err != nil {
		t.Errorf("Failed to get price: %v", err)
		return
	}

	if price <= 0 {
		t.Errorf("Invalid price returned: %.2f", price)
	}

	if price != expectedPrice {
		t.Errorf("Expected price %.2f, got %.2f", expectedPrice, price)
	}

	fmt.Printf("Bitcoin price on %s: $%.2f\n", testDate.Format("2006-01-02"), price)
}

func TestMockPriceAPI(t *testing.T) {
	api := NewMockPriceAPI()

	// Set a test price
	testDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	expectedPrice := 50000.0
	api.SetPrice("2025-01-01", expectedPrice)

	price, err := api.GetBTCPriceUSD(testDate)
	if err != nil {
		t.Errorf("Failed to get mock price: %v", err)
	}

	if price != expectedPrice {
		t.Errorf("Expected price %.2f, got %.2f", expectedPrice, price)
	}

	fmt.Printf("Mock Bitcoin price on %s: $%.2f\n", testDate.Format("2006-01-02"), price)
}

func TestPriceAPIIntegration(t *testing.T) {
	// Test that both APIs implement the interface correctly
	var apis []PriceAPI
	apis = append(apis, NewMempoolPriceAPI())
	apis = append(apis, NewMockPriceAPI())

	testDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for i, api := range apis {
		_, err := api.GetHistoricalPrice(testDate, "USD")
		if err != nil && i == 0 { // Only fail for mempool API if there's an actual error
			t.Errorf("API %d failed: %v", i, err)
		}

		_, err = api.GetBTCPriceUSD(testDate)
		if err != nil && i == 0 { // Only fail for mempool API if there's an actual error
			t.Errorf("API %d BTC price failed: %v", i, err)
		}
	}
}
