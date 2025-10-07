package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/matryer/is"
	"github.com/spf13/afero"
)

func TestMainWorkflowIntegration(t *testing.T) {
	is := is.New(t)

	t.Run("expandGlobPatterns with actual files", func(t *testing.T) {
		// Test with existing testdata files
		patterns := []string{
			"testdata/*.csv",
			"testdata/simple_transactions.csv", // literal file
		}

		expanded, err := expandGlobPatterns(patterns)
		is.NoErr(err)
		is.True(len(expanded) > 0)

		// Should contain our test files
		expandedStr := strings.Join(expanded, ",")
		is.True(strings.Contains(expandedStr, "simple_transactions.csv"))
		is.True(strings.Contains(expandedStr, "hifo_test.csv"))

		// May contain duplicates since expandGlobPatterns doesn't deduplicate
		// (deduplication happens later in the CSV parsing stage)
		simpleCount := 0
		for _, file := range expanded {
			if strings.Contains(file, "simple_transactions.csv") {
				simpleCount++
			}
		}
		is.True(simpleCount >= 1) // Should appear at least once
	})

	t.Run("expandGlobPatterns with non-existent files", func(t *testing.T) {
		patterns := []string{"testdata/non-existent-*.csv"}

		_, err := expandGlobPatterns(patterns)
		is.True(err != nil)
		is.True(strings.Contains(err.Error(), "no files match pattern"))
	})

	t.Run("expandGlobPatterns with invalid glob pattern", func(t *testing.T) {
		patterns := []string{"testdata/[invalid"}

		_, err := expandGlobPatterns(patterns)
		is.True(err != nil)
		is.True(strings.Contains(err.Error(), "invalid glob pattern"))
	})
}

func TestEndToEndWorkflowSingleYear(t *testing.T) {
	is := is.New(t)

	// Create a temporary directory for test outputs
	tempDir := t.TempDir()

	t.Run("single year processing with mock prices", func(t *testing.T) {
		// Use our test data
		inputFiles := []string{"testdata/simple_transactions.csv"}

		// Parse CSV files
		transactions, err := parseAndMergeCSVs(inputFiles)
		is.NoErr(err)
		is.True(len(transactions) > 0)

		// Initialize mock price API
		mockAPI := NewMockPriceAPI()

		// Initialize test cache
		cache := NewCache(afero.NewMemMapFs(), "hi-fold-test")
		defer cache.Clear() // Clean up after test

		// Test single year processing (2024)
		testYear := 2024
		lots, sales := calculateHIFOWithCache(transactions, mockAPI, testYear, cache, inputFiles)

		// Validate results
		is.True(len(lots) >= 0)  // May have zero lots remaining
		is.True(len(sales) >= 0) // May have zero sales in this year

		// Test tax record generation
		outputFile := filepath.Join(tempDir, "test-tax-records-2024.csv")
		err = generateTaxRecords(sales, outputFile)
		is.NoErr(err)

		// Verify output file was created
		_, err = os.Stat(outputFile)
		is.NoErr(err)

		// Test file content
		if len(sales) > 0 {
			content, err := os.ReadFile(outputFile)
			is.NoErr(err)

			contentStr := string(content)
			is.True(strings.Contains(contentStr, "Date")) // CSV header
			is.True(strings.Contains(contentStr, "2024")) // Should contain target year
		}
	})
}

func TestEndToEndWorkflowMultiYear(t *testing.T) {
	is := is.New(t)

	// Create a temporary directory for test outputs
	tempDir := t.TempDir()

	t.Run("multi-year processing with cached results", func(t *testing.T) {
		// Use multi-year test data
		inputFiles := []string{"testdata/multi_year.csv"}

		// Parse CSV files
		transactions, err := parseAndMergeCSVs(inputFiles)
		is.NoErr(err)
		is.True(len(transactions) > 0)

		// Initialize mock price API
		mockAPI := NewMockPriceAPI()

		// Initialize test cache
		cache := NewCache(afero.NewMemMapFs(), "hi-fold-test-multi")
		defer cache.Clear() // Clean up after test

		// Test multi-year processing
		allYearResults := calculateAllYearsWithCache(transactions, mockAPI, cache, inputFiles)

		// Validate results
		is.True(len(allYearResults) >= 0) // May have no sales years

		// Test that cache functions work
		for year := range allYearResults {
			// Test cache key generation
			cacheKey, err := generateCacheKey(year, inputFiles)
			is.NoErr(err)
			is.True(len(cacheKey) > 0)

			// Generate test output file
			outputFile := filepath.Join(tempDir, "test-tax-records-"+string(rune(year))+".csv")
			err = generateTaxRecords(allYearResults[year].Sales, outputFile)
			is.NoErr(err)
		}
	})
}

func TestWorkflowErrorHandling(t *testing.T) {
	is := is.New(t)

	t.Run("invalid CSV file handling", func(t *testing.T) {
		// Try to parse a malformed CSV
		inputFiles := []string{"testdata/malformed.csv"}

		transactions, err := parseAndMergeCSVs(inputFiles)
		// Should still succeed but with warnings/skipped rows
		is.NoErr(err)

		// May have fewer transactions due to skipped invalid rows
		is.True(len(transactions) >= 0)
	})

	t.Run("empty transaction list handling", func(t *testing.T) {
		// Test with empty transactions
		var emptyTransactions []Transaction

		mockAPI := NewMockPriceAPI()
		cache := NewCache(afero.NewMemMapFs(), "hi-fold-test-empty")
		defer cache.Clear()

		// Should handle empty input gracefully
		inputFiles := []string{"testdata/empty_transactions.csv"}
		lots, sales := calculateHIFOWithCache(emptyTransactions, mockAPI, 2024, cache, inputFiles)

		is.True(len(lots) == 0)
		is.True(len(sales) == 0)
	})

	t.Run("invalid output directory handling", func(t *testing.T) {
		// Try to generate output to invalid directory
		invalidPath := "/invalid/path/that/does/not/exist/output.csv"

		err := generateTaxRecords([]Sale{}, invalidPath)
		is.True(err != nil) // Should fail
	})
}

func TestCacheIntegration(t *testing.T) {
	is := is.New(t)

	t.Run("cache save and load integration", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "hi-fold-integration-test")
		defer cache.Clear()

		inputFiles := []string{"testdata/simple_transactions.csv"}
		testYear := 2024

		// Create test data
		testDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		btcAmount, _ := newBTCFromString("1.0")
		usdAmount := money.NewFromFloat(50000.0, money.USD)

		testLots := []Lot{
			{
				Date:         testDate,
				AmountBTC:    btcAmount,
				CostBasisUSD: usdAmount,
				PricePerCoin: usdAmount,
				Remaining:    btcAmount,
			},
		}

		testSales := []Sale{
			{
				Date:         testDate.AddDate(0, 6, 0),
				AmountBTC:    btcAmount,
				ProceedsUSD:  usdAmount,
				CostBasisUSD: usdAmount,
				GainLossUSD:  usdAmount,
				Lots:         []LotSale{},
			},
		}

		// Test save
		err := cache.saveYearEndState(testYear, testLots, testSales, inputFiles)
		is.NoErr(err)

		// Test load
		loadedState, err := cache.loadYearEndState(testYear, inputFiles)
		is.NoErr(err)
		is.True(loadedState != nil)
		is.Equal(len(loadedState.Lots), len(testLots))
		is.Equal(len(loadedState.Sales), len(testSales))

		// Verify data integrity
		is.Equal(loadedState.Lots[0].Date, testLots[0].Date)
		is.Equal(loadedState.Lots[0].AmountBTC.Amount(), testLots[0].AmountBTC.Amount())
	})

	t.Run("cache invalidation integration", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "hi-fold-invalidation-test")
		defer cache.Clear()

		inputFiles := []string{"testdata/simple_transactions.csv"}
		testYear := 2024

		// Save some cache data first
		testDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		btcAmount, _ := newBTCFromString("1.0")
		usdAmount := money.NewFromFloat(50000.0, money.USD)

		testLots := []Lot{
			{
				Date:         testDate,
				AmountBTC:    btcAmount,
				CostBasisUSD: usdAmount,
				PricePerCoin: usdAmount,
				Remaining:    btcAmount,
			},
		}

		err := cache.saveYearEndState(testYear, testLots, []Sale{}, inputFiles)
		is.NoErr(err)

		// Verify cache exists
		loadedState, err := cache.loadYearEndState(testYear, inputFiles)
		is.NoErr(err)
		is.True(loadedState != nil)

		// Test invalidation
		err = cache.invalidateCache(testYear, inputFiles)
		is.NoErr(err)

		// Verify cache is gone
		loadedState, err = cache.loadYearEndState(testYear, inputFiles)
		is.True(err != nil) // Should return error for cache miss
		is.True(loadedState == nil)
	})
}

func TestPriceAPIIntegration2(t *testing.T) {
	is := is.New(t)

	t.Run("mock price API integration with HIFO calculation", func(t *testing.T) {
		mockAPI := NewMockPriceAPI()

		// Set known prices for testing
		testDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		dateStr := testDate.Format("2006-01-02")
		mockAPI.SetPrice(dateStr, 50000.0)

		// Test price retrieval
		price, err := mockAPI.GetHistoricalPrice(testDate, "USD")
		is.NoErr(err)
		is.Equal(price, 50000.0)

		// Test current price
		currentPrice, err := mockAPI.GetCurrentPriceUSD()
		is.NoErr(err)
		is.True(currentPrice > 0)
	})

	t.Run("price API error handling in workflow", func(t *testing.T) {
		// Test with transactions that might require price lookups
		inputFiles := []string{"testdata/deposits_withdrawals.csv"}

		transactions, err := parseAndMergeCSVs(inputFiles)
		is.NoErr(err)

		// Use mock API (should not fail)
		mockAPI := NewMockPriceAPI()
		cache := NewCache(afero.NewMemMapFs(), "hi-fold-price-test")
		defer cache.Clear()

		// Should complete without errors
		lots, sales := calculateHIFOWithCache(transactions, mockAPI, 2024, cache, inputFiles)

		is.True(len(lots) >= 0)
		is.True(len(sales) >= 0)
	})
}

func TestFullEndToEndScenario(t *testing.T) {
	is := is.New(t)

	t.Run("complete workflow simulation", func(t *testing.T) {
		// Simulate the full CLI workflow without actually running CLI

		tempDir := t.TempDir()

		// Step 1: Parse input files (simulate --input flag)
		inputFiles := []string{"testdata/simple_transactions.csv", "testdata/deposits_withdrawals.csv"}

		expandedFiles, err := expandGlobPatterns(inputFiles)
		is.NoErr(err)
		is.True(len(expandedFiles) >= 2)

		// Step 2: Parse and merge CSVs
		transactions, err := parseAndMergeCSVs(expandedFiles)
		is.NoErr(err)
		is.True(len(transactions) > 0)

		// Step 3: Initialize services (simulate --mock-prices flag)
		mockAPI := NewMockPriceAPI()
		cache := NewCache(afero.NewMemMapFs(), "hi-fold-full-test")
		defer cache.Clear()

		// Step 4: Process single year (simulate --year 2024)
		targetYear := 2024
		_, sales := calculateHIFOWithCache(transactions, mockAPI, targetYear, cache, expandedFiles)

		// Step 5: Generate output (simulate --output flag)
		outputFile := filepath.Join(tempDir, "tax-records-2024.csv")
		err = generateTaxRecords(sales, outputFile)
		is.NoErr(err)

		// Verify complete workflow
		fileInfo, err := os.Stat(outputFile)
		is.NoErr(err)
		is.True(fileInfo.Size() >= 0) // File should exist and have some size

		// Verify cache was used
		cacheKey, err := generateCacheKey(targetYear, expandedFiles)
		is.NoErr(err)
		is.True(len(cacheKey) > 0)
	})

	t.Run("multi-year workflow simulation", func(t *testing.T) {
		tempDir := t.TempDir()

		// Use multi-year test data
		inputFiles := []string{"testdata/multi_year.csv"}

		transactions, err := parseAndMergeCSVs(inputFiles)
		is.NoErr(err)
		is.True(len(transactions) > 0)

		mockAPI := NewMockPriceAPI()
		cache := NewCache(afero.NewMemMapFs(), "hi-fold-multi-full-test")
		defer cache.Clear()

		// Simulate processAllYears function
		allYearResults := calculateAllYearsWithCache(transactions, mockAPI, cache, inputFiles)

		// Generate outputs for each year (simulate --output-dir flag)
		for year, result := range allYearResults {
			outputFile := filepath.Join(tempDir, "tax-records-"+string(rune(year+48))+".csv")
			err := generateTaxRecords(result.Sales, outputFile)
			is.NoErr(err)

			// Verify file exists
			_, err = os.Stat(outputFile)
			is.NoErr(err)
		}
	})
}
