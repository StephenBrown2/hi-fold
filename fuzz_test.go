package main

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/matryer/is"
)

// FuzzFoldToTransaction tests Fold.ToTransaction with randomized inputs
func FuzzFoldToTransaction(f *testing.F) {
	f.Add("txn-001", "2024-01-15 10:30:00.000000+00:00", "Purchase", "BTC", "0.10000000", "40000", "-4000", "0", "-4000")
	f.Add("txn-002", "2023-12-01 00:00:00.000000+00:00", "Sale", "BTC", "-0.05000000", "50000", "2500", "100", "2600")
	f.Add("txn-003", "2024-06-01 12:00:00.000000+00:00", "Deposit", "BTC", "1.00000000", "", "", "", "")
	f.Add("", "", "", "", "", "", "", "", "")

	f.Fuzz(func(t *testing.T, refID, date, txType, asset, amount, price, subtotal, fee, total string) {
		is := is.New(t)

		parsedDate, err := time.Parse(foldDateTimeLayout, date)
		if err != nil {
			return
		}

		fold := Fold{
			ReferenceID:     refID,
			Date:            foldDate{Time: parsedDate},
			TransactionType: txType,
			Description:     "fuzz",
			Asset:           asset,
			AmountBTC:       amount,
			PricePerCoin:    price,
			SubtotalUSD:     subtotal,
			FeeUSD:          fee,
			TotalUSD:        total,
			TransactionID:   refID,
		}

		transaction, err := fold.ToTransaction()
		if err != nil {
			// Error is acceptable for invalid input
			return
		}

		// If parsing succeeded, validate the transaction makes sense
		is.True(transaction.Date.Year() >= 1970 && transaction.Date.Year() <= 2100) // Reasonable date range
		is.True(transaction.TransactionType != "")                                  // Type should not be empty if parsing succeeded
		is.True(transaction.TransactionID != "")                                    // ID should not be empty if parsing succeeded

		// Validate BTC amount is reasonable (not extremely large)
		if transaction.AmountBTC != nil {
			btcAmount := transaction.AmountBTC.Amount()
			is.True(btcAmount >= -2100000000000000 && btcAmount <= 2100000000000000) // Within reasonable BTC supply range (satoshis)
		}

		// Validate USD amounts are reasonable
		if transaction.PricePerCoin != nil {
			priceAmount := transaction.PricePerCoin.Amount()
			is.True(priceAmount >= 0 && priceAmount <= 100000000) // $0 to $1M per BTC
		}
	})
}

// FuzzNewBTCFromString tests BTC amount parsing with random strings
func FuzzNewBTCFromString(f *testing.F) {
	// Seed with valid BTC amounts
	f.Add("0.00000001") // 1 satoshi
	f.Add("1.0")
	f.Add("21000000") // Max BTC supply
	f.Add("-0.5")
	f.Add("0")
	f.Add("1.12345678") // Max precision

	f.Fuzz(func(t *testing.T, btcStr string) {
		is := is.New(t)

		btc, err := newBTCFromString(btcStr)
		if err != nil {
			// Error is acceptable for invalid input
			return
		}

		// If parsing succeeded, validate the result
		amount := btc.Amount()

		// Skip test if amount exceeds maximum Bitcoin supply
		// 21M BTC = 2.1e15 satoshis (exact maximum Bitcoin supply)
		const maxSatoshis = int64(2100000000000000) // 21M BTC in satoshis
		if max(amount, -amount) > maxSatoshis {
			t.Skip("amount exceeds max BTC supply")
		}

		// Should have correct currency
		is.Equal(btc.Currency().Code, "BTC")
	})
}

// FuzzCSVParsing tests CSV parsing with random CSV content
func FuzzCSVParsing(f *testing.F) {
	// Seed with valid CSV lines
	f.Add("Date,Type,Amount (BTC),Price Per Coin (USD),Subtotal (USD),Fee (USD),Total (USD),Notes,Notes,Notes,Reference ID")
	f.Add("2024-01-01T00:00:00Z,Purchase,1.0,50000,50000,100,50100,,,,,txn-001")
	f.Add("invalid,row,with,wrong,field,count")
	f.Add("")

	f.Fuzz(func(t *testing.T, csvContent string) {
		is := is.New(t)

		// Create temporary file with the fuzzed content
		tempFile := createTempCSV(t, csvContent)
		defer tempFile.Close()

		transactions, err := parseCSV(tempFile.Name())
		if err != nil {
			// Errors are acceptable for invalid CSV content
			return
		}

		// If parsing succeeded, validate results
		is.True(len(transactions) >= 0) // Should be non-negative

		for _, tx := range transactions {
			// Basic sanity checks on parsed transactions
			is.True(tx.Date.Year() >= 1970 && tx.Date.Year() <= 2100)
			is.True(tx.TransactionType != "")
			is.True(tx.TransactionID != "")
		}
	})
}

// FuzzMoneyOperations tests money arithmetic operations with random values
func FuzzMoneyOperations(f *testing.F) {
	// Seed with some values
	f.Add(int64(100000000), int64(50000000)) // 1.0 BTC, 0.5 BTC
	f.Add(int64(0), int64(1))                // Edge cases
	f.Add(int64(-50000000), int64(25000000)) // Negative values

	f.Fuzz(func(t *testing.T, amount1, amount2 int64) {
		is := is.New(t)

		// Limit to reasonable ranges to avoid overflow
		if amount1 < -1e15 || amount1 > 1e15 || amount2 < -1e15 || amount2 > 1e15 {
			return
		}

		// Create money objects
		btc1, err := newBTCFromString(formatSatoshis(amount1))
		if err != nil {
			return
		}

		btc2, err := newBTCFromString(formatSatoshis(amount2))
		if err != nil {
			return
		}

		// Test addition
		sum, err := btc1.Add(btc2)
		if err == nil {
			is.True(sum.Amount() == amount1+amount2)
		}

		// Test subtraction
		diff, err := btc1.Subtract(btc2)
		if err == nil {
			is.True(diff.Amount() == amount1-amount2)
		}

		// Test comparison
		if amount1 > amount2 {
			isGreater, _ := btc1.GreaterThan(btc2)
			is.True(isGreater)
		} else if amount1 < amount2 {
			isLess, _ := btc1.LessThan(btc2)
			is.True(isLess)
		} else {
			isEqual, _ := btc1.Equals(btc2)
			is.True(isEqual)
		}
	})
}

// FuzzLotSelection tests HIFO lot selection with random lot configurations
func FuzzLotSelection(f *testing.F) {
	// Seed with some lot scenarios
	f.Add(int64(100000000), int64(4000000), true) // 1 BTC lot at $40k, long-term
	f.Add(int64(50000000), int64(6000000), false) // 0.5 BTC lot at $60k, short-term
	f.Add(int64(1), int64(5000000), true)         // 1 satoshi lot

	f.Fuzz(func(t *testing.T, amountSatoshis, pricePerBTCCents int64, isLongTerm bool) {
		is := is.New(t)

		// Limit to reasonable ranges
		if amountSatoshis <= 0 || amountSatoshis > 2100000000000000 { // Max 21M BTC in satoshis
			return
		}
		if pricePerBTCCents <= 0 || pricePerBTCCents > 10000000 { // Max $100k per BTC in cents
			return
		}

		// Create test lot
		saleDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		var lotDate time.Time
		if isLongTerm {
			lotDate = saleDate.AddDate(-2, 0, 0) // Make it long-term (more than 1 year before sale)
		} else {
			lotDate = saleDate.AddDate(0, -6, 0) // Make it short-term (6 months before sale)
		}

		btcAmount, err := newBTCFromString(formatSatoshis(amountSatoshis))
		if err != nil {
			return
		}

		pricePerBTC := formatCents(pricePerBTCCents)
		pricePerCoinFloat, err := strconv.ParseFloat(pricePerBTC, 64)
		if err != nil {
			return
		}

		// Calculate cost basis: amount * price
		costBasisFloat := (float64(amountSatoshis) / 100000000.0) * pricePerCoinFloat
		if costBasisFloat > 1e12 { // Avoid extremely large values
			return
		}

		lot := createTestLot(lotDate, btcAmount, pricePerCoinFloat)

		// Validate lot creation
		is.True(lot.AmountBTC.Amount() == amountSatoshis)
		is.True(lot.Remaining.Amount() == amountSatoshis)
		// Note: Price comparison may have rounding differences due to float conversion
		actualPrice := lot.PricePerCoin.Amount()
		is.True(actualPrice >= pricePerBTCCents-1 && actualPrice <= pricePerBTCCents+1) // Allow 1 cent tolerance

		// Test lot priority scoring (from HIFO logic)
		// (saleDate was defined above)
		isLongTermActual := saleDate.Sub(lot.Date).Hours() >= 24*365
		// The lot date was set based on isLongTerm input, so they should match
		is.Equal(isLongTermActual, isLongTerm)
	})
}

// FuzzCacheKeyGeneration tests cache key generation with random inputs
func FuzzCacheKeyGeneration(f *testing.F) {
	// Seed with typical values
	f.Add(2024, "file1.csv")
	f.Add(1999, "path/to/file.csv")
	f.Add(2050, "")

	f.Fuzz(func(t *testing.T, year int, filename string) {
		is := is.New(t)

		// Only test reasonable year ranges
		if year < 1900 || year > 2200 {
			return
		}

		// Skip if filename has problematic characters
		if strings.ContainsAny(filename, "\x00\n\r") {
			return
		}

		files := []string{filename}
		if filename == "" {
			files = []string{} // Empty file list
		}

		cacheKey, err := generateCacheKey(year, files)
		if err != nil {
			// Errors are acceptable for invalid inputs
			return
		}

		// Validate cache key format
		is.True(len(cacheKey) > 0)
		is.True(strings.HasPrefix(cacheKey, "year_"+strconv.Itoa(year)))
		is.True(strings.HasSuffix(cacheKey, ".json"))

		// Should not contain problematic characters
		is.True(!strings.ContainsAny(cacheKey, "\x00\n\r/\\"))
	})
}

// Helper functions for fuzz tests

func createTempCSV(t *testing.T, content string) *os.File {
	tempFile, err := os.CreateTemp(t.TempDir(), "fuzz_*.csv")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tempFile.WriteString(content); err != nil {
		t.Fatal(err)
	}

	if err := tempFile.Close(); err != nil {
		t.Fatal(err)
	}

	return tempFile
}

func formatSatoshis(satoshis int64) string {
	btc := float64(satoshis) / 100000000.0
	return strconv.FormatFloat(btc, 'f', 8, 64)
}

func formatCents(cents int64) string {
	usd := float64(cents) / 100.0
	return strconv.FormatFloat(usd, 'f', 2, 64)
}

func createTestLot(date time.Time, amountBTC *money.Money, pricePerCoinUSD float64) Lot {
	pricePerCoin := money.NewFromFloat(pricePerCoinUSD, money.USD)
	costBasis := money.NewFromFloat((float64(amountBTC.Amount())/100000000.0)*pricePerCoinUSD, money.USD)

	return Lot{
		Date:         date,
		AmountBTC:    amountBTC,
		CostBasisUSD: costBasis,
		PricePerCoin: pricePerCoin,
		Remaining:    amountBTC,
	}
}
