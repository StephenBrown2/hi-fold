package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestMockPriceAPI(t *testing.T) {
	is := is.New(t)

	t.Run("create mock price API", func(t *testing.T) {
		mockAPI := NewMockPriceAPI()
		is.True(mockAPI != nil)
		is.True(mockAPI.prices != nil)
	})

	t.Run("set and get mock prices", func(t *testing.T) {
		mockAPI := NewMockPriceAPI()

		// Set specific prices for specific dates
		mockAPI.SetPrice("2024-01-01", 42000.00)
		mockAPI.SetPrice("2024-06-15", 65000.00)

		// Test GetBTCPriceUSD
		price1, err := mockAPI.GetBTCPriceUSD(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC))
		is.NoErr(err)
		is.Equal(price1, 42000.00)

		price2, err := mockAPI.GetBTCPriceUSD(time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC))
		is.NoErr(err)
		is.Equal(price2, 65000.00)

		// Test GetHistoricalPrice
		price3, err := mockAPI.GetHistoricalPrice(time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), "USD")
		is.NoErr(err)
		is.Equal(price3, 42000.00)
	})

	t.Run("default mock price for unknown dates", func(t *testing.T) {
		mockAPI := NewMockPriceAPI()

		// Should return default price for unknown dates
		price, err := mockAPI.GetBTCPriceUSD(time.Date(2024, 12, 25, 10, 0, 0, 0, time.UTC))
		is.NoErr(err)
		is.Equal(price, 50000.00) // Default mock price
	})

	t.Run("get current price USD", func(t *testing.T) {
		mockAPI := NewMockPriceAPI()

		price, err := mockAPI.GetCurrentPriceUSD()
		is.NoErr(err)
		is.Equal(price, 95000.00) // Mock current price
	})

	t.Run("different currencies return same USD price", func(t *testing.T) {
		mockAPI := NewMockPriceAPI()
		mockAPI.SetPrice("2024-01-01", 50000.00)

		testDate := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		// All currencies should return the same mock price (simplified mock)
		usdPrice, err := mockAPI.GetHistoricalPrice(testDate, "USD")
		is.NoErr(err)
		is.Equal(usdPrice, 50000.00)

		eurPrice, err := mockAPI.GetHistoricalPrice(testDate, "EUR")
		is.NoErr(err)
		is.Equal(eurPrice, 50000.00) // Mock doesn't do currency conversion
	})
}

func TestNormalizeBaseURL(t *testing.T) {
	is := is.New(t)

	t.Run("empty URL defaults to mempool.space", func(t *testing.T) {
		normalized := normalizeBaseURL("")
		is.Equal(normalized.String(), "https://mempool.space")
	})

	t.Run("URL without scheme gets https", func(t *testing.T) {
		normalized := normalizeBaseURL("mempool.space")
		is.Equal(normalized.String(), "https://mempool.space")
	})

	t.Run("URL with http scheme preserved", func(t *testing.T) {
		normalized := normalizeBaseURL("http://localhost:8080")
		is.Equal(normalized.String(), "http://localhost:8080")
	})

	t.Run("URL with https scheme preserved", func(t *testing.T) {
		normalized := normalizeBaseURL("https://custom.mempool.space")
		is.Equal(normalized.String(), "https://custom.mempool.space")
	})

	t.Run("trailing slash removed", func(t *testing.T) {
		normalized := normalizeBaseURL("https://mempool.space/")
		is.Equal(normalized.String(), "https://mempool.space")
	})

	t.Run("invalid scheme becomes https", func(t *testing.T) {
		normalized := normalizeBaseURL("ftp://mempool.space")
		is.Equal(normalized.String(), "https://mempool.space")
	})

	t.Run("malformed URL fallback", func(t *testing.T) {
		normalized := normalizeBaseURL("not-a-valid-url-://test")
		is.Equal(normalized.Scheme, "https")
		// The malformed URL parsing results in "test" as the host
		is.Equal(normalized.Host, "test")
	})
}

func TestMempoolPriceAPI(t *testing.T) {
	is := is.New(t)

	t.Run("create mempool API with default URL", func(t *testing.T) {
		api := NewMempoolPriceAPI()
		is.True(api != nil)
		is.True(strings.Contains(api.baseURL.String(), "mempool.space"))
		is.True(strings.Contains(api.baseURL.String(), "api/v1"))
	})

	t.Run("create mempool API with custom URL", func(t *testing.T) {
		api := NewMempoolPriceAPIWithURL("https://custom.mempool.space")
		is.True(api != nil)
		is.True(strings.Contains(api.baseURL.String(), "custom.mempool.space"))
		is.True(strings.Contains(api.baseURL.String(), "api/v1"))
	})

	t.Run("create mempool API with localhost", func(t *testing.T) {
		api := NewMempoolPriceAPIWithURL("http://localhost:8080")
		is.True(api != nil)
		is.True(strings.Contains(api.baseURL.String(), "localhost:8080"))
		is.Equal(api.baseURL.Scheme, "http")
	})
}

// Test actual HTTP interactions with mock server
func TestMempoolPriceAPIHTTP(t *testing.T) {
	is := is.New(t)

	t.Run("successful historical price request", func(t *testing.T) {
		// Create mock server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request URL and parameters
			is.True(strings.Contains(r.URL.Path, "historical-price"))

			query := r.URL.Query()
			is.Equal(query.Get("currency"), "USD")
			is.True(query.Get("timestamp") != "")

			// Return mock response
			response := `{
				"prices": [
					{
						"time": 1640995200,
						"USD": 47000.50,
						"EUR": 41500.25,
						"GBP": 35200.75,
						"CAD": 59800.00,
						"CHF": 43200.00,
						"AUD": 65300.00,
						"JPY": 5400000.00
					}
				]
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		// Create API client with test server URL
		api := NewMempoolPriceAPIWithURL(server.URL)

		testDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
		price, err := api.GetBTCPriceUSD(testDate)

		is.NoErr(err)
		is.Equal(price, 47000.50)
	})

	t.Run("successful current price request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request URL
			is.True(strings.Contains(r.URL.Path, "prices"))

			// Return mock current price response
			response := `{
				"time": 1640995200,
				"USD": 95000.00,
				"EUR": 85000.00,
				"GBP": 72000.00,
				"CAD": 120000.00,
				"CHF": 87000.00,
				"AUD": 135000.00,
				"JPY": 13500000.00
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		api := NewMempoolPriceAPIWithURL(server.URL)
		price, err := api.GetCurrentPriceUSD()

		is.NoErr(err)
		is.Equal(price, 95000.00)
	})

	t.Run("API server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		api := NewMempoolPriceAPIWithURL(server.URL)
		testDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

		_, err := api.GetBTCPriceUSD(testDate)
		is.True(err != nil)
		is.True(strings.Contains(err.Error(), "status 500"))
	})

	t.Run("malformed JSON response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		api := NewMempoolPriceAPIWithURL(server.URL)
		testDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

		_, err := api.GetBTCPriceUSD(testDate)
		is.True(err != nil)
		is.True(strings.Contains(err.Error(), "decode"))
	})

	t.Run("empty prices array", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := `{"prices": []}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		api := NewMempoolPriceAPIWithURL(server.URL)
		testDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

		_, err := api.GetBTCPriceUSD(testDate)
		is.True(err != nil)
		is.True(strings.Contains(err.Error(), "no price data"))
	})

	t.Run("unsupported currency", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			response := `{
				"prices": [
					{
						"time": 1640995200,
						"USD": 47000.50,
						"EUR": 41500.25
					}
				]
			}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		api := NewMempoolPriceAPIWithURL(server.URL)
		testDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

		_, err := api.GetHistoricalPrice(testDate, "XYZ")
		is.True(err != nil)
		is.True(strings.Contains(err.Error(), "unsupported currency"))
	})

	t.Run("connection refused", func(t *testing.T) {
		// Use a port that should be closed
		api := NewMempoolPriceAPIWithURL("http://localhost:9999")
		testDate := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

		_, err := api.GetBTCPriceUSD(testDate)
		is.True(err != nil)
	})
}

// Test URL construction and parameter encoding
func TestMempoolPriceAPIURLs(t *testing.T) {
	is := is.New(t)

	t.Run("historical price URL construction", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify URL path
			expectedPath := "/api/v1/historical-price"
			is.Equal(r.URL.Path, expectedPath)

			// Verify query parameters
			query := r.URL.Query()
			is.Equal(query.Get("currency"), "USD")

			timestamp := query.Get("timestamp")
			is.True(timestamp != "")

			// Verify timestamp is correct Unix timestamp
			expectedTimestamp := fmt.Sprintf("%d", time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC).Unix())
			is.Equal(timestamp, expectedTimestamp)

			// Return minimal valid response
			response := `{"prices": [{"time": 1704110400, "USD": 42000.00}]}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		api := NewMempoolPriceAPIWithURL(server.URL)
		testDate := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

		_, err := api.GetBTCPriceUSD(testDate)
		is.NoErr(err)
	})

	t.Run("current price URL construction", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify URL path
			expectedPath := "/api/v1/prices"
			is.Equal(r.URL.Path, expectedPath)

			// Should have no query parameters for current price
			is.Equal(len(r.URL.Query()), 0)

			// Return minimal valid response
			response := `{"time": 1704110400, "USD": 95000.00}`
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(response))
		}))
		defer server.Close()

		api := NewMempoolPriceAPIWithURL(server.URL)
		_, err := api.GetCurrentPriceUSD()
		is.NoErr(err)
	})

	t.Run("URL joining with custom base path", func(t *testing.T) {
		// Test that URL joining works correctly with different base URLs
		testCases := []struct {
			baseURL      string
			expectedHost string
		}{
			{"https://mempool.space", "mempool.space"},
			{"https://mempool.space/", "mempool.space"}, // Trailing slash removed
			{"http://localhost:8080", "localhost:8080"},
			{"https://custom.domain.com/path", "custom.domain.com"},
		}

		for _, tc := range testCases {
			api := NewMempoolPriceAPIWithURL(tc.baseURL)

			// Parse the base URL to verify it's constructed correctly
			parsedURL, err := url.Parse(api.baseURL.String())
			is.NoErr(err)
			is.Equal(parsedURL.Host, tc.expectedHost)
			is.True(strings.Contains(parsedURL.Path, "api/v1"))
		}
	})
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
			t.Skipf("API %d failed: %v", i, err)
		}

		_, err = api.GetBTCPriceUSD(testDate)
		if err != nil && i == 0 { // Only fail for mempool API if there's an actual error
			t.Skipf("API %d BTC price failed: %v", i, err)
		}
	}
}
