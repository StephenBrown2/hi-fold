package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MempoolPriceResponse represents the response from mempool.space API
type MempoolPriceResponse struct {
	Prices []PriceData `json:"prices"`
}

// PriceData represents a single price data point
type PriceData struct {
	Time int64   `json:"time"`
	USD  float64 `json:"USD"`
	EUR  float64 `json:"EUR"`
	GBP  float64 `json:"GBP"`
	CAD  float64 `json:"CAD"`
	CHF  float64 `json:"CHF"`
	AUD  float64 `json:"AUD"`
	JPY  float64 `json:"JPY"`
}

// PriceAPI interface for getting historical Bitcoin prices
type PriceAPI interface {
	GetHistoricalPrice(timestamp time.Time, currency string) (float64, error)
	GetBTCPriceUSD(timestamp time.Time) (float64, error)
	GetCurrentPriceUSD() (float64, error)
}

// MempoolPriceAPI implements PriceAPI using mempool.space
type MempoolPriceAPI struct {
	baseURL string
	client  *http.Client
}

// NewMempoolPriceAPI creates a new mempool.space price API client
func NewMempoolPriceAPI() *MempoolPriceAPI {
	return &MempoolPriceAPI{
		baseURL: "https://mempool.space/api/v1",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetHistoricalPrice fetches the historical Bitcoin price for a given timestamp
func (m *MempoolPriceAPI) GetHistoricalPrice(timestamp time.Time, currency string) (float64, error) {
	// Convert timestamp to Unix timestamp
	unixTimestamp := timestamp.Unix()

	// Build the URL
	url := fmt.Sprintf("%s/historical-price?currency=%s&timestamp=%d",
		m.baseURL, currency, unixTimestamp)

	// Make the HTTP request
	resp, err := m.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch price data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse the JSON response
	var priceResp MempoolPriceResponse
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return 0, fmt.Errorf("failed to decode price response: %w", err)
	}

	// Extract the price based on currency
	if len(priceResp.Prices) == 0 {
		return 0, fmt.Errorf("no price data available for timestamp %d", unixTimestamp)
	}

	priceData := priceResp.Prices[0]
	switch currency {
	case "USD":
		return priceData.USD, nil
	case "EUR":
		return priceData.EUR, nil
	case "GBP":
		return priceData.GBP, nil
	case "CAD":
		return priceData.CAD, nil
	case "CHF":
		return priceData.CHF, nil
	case "AUD":
		return priceData.AUD, nil
	case "JPY":
		return priceData.JPY, nil
	default:
		return 0, fmt.Errorf("unsupported currency: %s", currency)
	}
}

// GetBTCPriceUSD is a convenience method to get USD price
func (m *MempoolPriceAPI) GetBTCPriceUSD(timestamp time.Time) (float64, error) {
	return m.GetHistoricalPrice(timestamp, "USD")
}

// GetCurrentPriceUSD fetches the current Bitcoin price in USD
func (m *MempoolPriceAPI) GetCurrentPriceUSD() (float64, error) {
	url := fmt.Sprintf("%s/prices", m.baseURL)

	// Make the HTTP request
	resp, err := m.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch current price: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse the JSON response
	var priceResp PriceData
	if err := json.NewDecoder(resp.Body).Decode(&priceResp); err != nil {
		return 0, fmt.Errorf("failed to decode current price response: %w", err)
	}

	return priceResp.USD, nil
}

// MockPriceAPI implements PriceAPI for testing purposes
type MockPriceAPI struct {
	prices map[string]float64
}

// NewMockPriceAPI creates a new mock price API for testing
func NewMockPriceAPI() *MockPriceAPI {
	return &MockPriceAPI{
		prices: make(map[string]float64),
	}
}

// SetPrice sets a mock price for a specific date
func (m *MockPriceAPI) SetPrice(date string, price float64) {
	m.prices[date] = price
}

// GetHistoricalPrice returns the mock price for testing
func (m *MockPriceAPI) GetHistoricalPrice(timestamp time.Time, currency string) (float64, error) {
	dateKey := timestamp.Format("2006-01-02")
	if price, exists := m.prices[dateKey]; exists {
		return price, nil
	}
	return 50000.0, nil // Default mock price
}

// GetBTCPriceUSD is a convenience method for mock API
func (m *MockPriceAPI) GetBTCPriceUSD(timestamp time.Time) (float64, error) {
	return m.GetHistoricalPrice(timestamp, "USD")
}

// GetCurrentPriceUSD returns a mock current price for testing
func (m *MockPriceAPI) GetCurrentPriceUSD() (float64, error) {
	return 95000.0, nil // Mock current price
}
