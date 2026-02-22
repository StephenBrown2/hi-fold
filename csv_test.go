package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/matryer/is"
)

func TestParseCSV(t *testing.T) {
	is := is.New(t)

	t.Run("valid CSV with transactions", func(t *testing.T) {
		transactions, err := parseCSV("testdata/simple_transactions.csv")
		is.NoErr(err)
		is.Equal(len(transactions), 3)

		// Test first transaction (purchase)
		tx1 := transactions[0]
		is.Equal(tx1.ReferenceID, "txn-001")
		is.Equal(tx1.TransactionType, "Purchase")
		is.Equal(tx1.Description, "Test Purchase")
		is.Equal(tx1.Asset, "BTC")
		is.Equal(tx1.AmountBTC.Amount(), int64(100000000))  // 1.0 BTC in satoshis
		is.Equal(tx1.PricePerCoin.Amount(), int64(4000000)) // $40000.00 in cents
		is.Equal(tx1.TotalUSD.Amount(), int64(-4000000))    // -$40000.00 in cents

		// Test sale transaction (negative BTC amount)
		tx3 := transactions[2]
		is.Equal(tx3.TransactionType, "Sale")
		is.Equal(tx3.AmountBTC.Amount(), int64(-25000000)) // -0.25 BTC in satoshis
		is.Equal(tx3.TotalUSD.Amount(), int64(1500000))    // $15000.00 in cents
	})

	t.Run("empty CSV file", func(t *testing.T) {
		transactions, err := parseCSV("testdata/empty_transactions.csv")
		is.NoErr(err)
		is.Equal(len(transactions), 0)
	})

	t.Run("malformed CSV file", func(t *testing.T) {
		transactions, err := parseCSV("testdata/malformed.csv")
		// Should not return error but should skip malformed rows
		is.NoErr(err)
		// Should have skipped all malformed transactions
		is.Equal(len(transactions), 0)
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := parseCSV("testdata/nonexistent.csv")
		is.True(err != nil)
	})

	t.Run("CSV without header", func(t *testing.T) {
		// Create a CSV file without proper header
		tempFile := filepath.Join(t.TempDir(), "no_header.csv")
		content := `"txn-001","2024-01-01 10:00:00.000000+00:00","Purchase","Test","BTC","1.0","40000.00","-40000.00","0.00","-40000.00",""`
		err := os.WriteFile(tempFile, []byte(content), 0o644)
		is.NoErr(err)

		_, err = parseCSV(tempFile)
		is.True(err != nil) // Should error when header not found
	})
}

func TestFoldToTransaction(t *testing.T) {
	is := is.New(t)

	t.Run("valid purchase transaction", func(t *testing.T) {
		parsedDate, err := time.Parse(foldDateTimeLayout, "2024-01-01 10:00:00.000000+00:00")
		is.NoErr(err)

		fold := Fold{
			ReferenceID:     "txn-001",
			Date:            foldDate{Time: parsedDate},
			TransactionType: "Purchase",
			Description:     "Test Purchase",
			Asset:           "BTC",
			AmountBTC:       "1.00000000",
			PricePerCoin:    "40000.00",
			SubtotalUSD:     "-40000.00",
			FeeUSD:          "0.00",
			TotalUSD:        "-40000.00",
			TransactionID:   "",
		}

		tx, err := fold.ToTransaction()
		is.NoErr(err)
		is.Equal(tx.ReferenceID, "txn-001")
		is.Equal(tx.TransactionType, "Purchase")
		is.Equal(tx.Description, "Test Purchase")
		is.Equal(tx.Asset, "BTC")
		is.Equal(tx.AmountBTC.Amount(), int64(100000000))  // 1.0 BTC in satoshis
		is.Equal(tx.PricePerCoin.Amount(), int64(4000000)) // $40000.00 in cents
		is.Equal(tx.TotalUSD.Amount(), int64(-4000000))    // -$40000.00 in cents
	})

	t.Run("deposit transaction with empty price fields", func(t *testing.T) {
		parsedDate, err := time.Parse(foldDateTimeLayout, "2024-01-01 10:00:00.000000+00:00")
		is.NoErr(err)

		fold := Fold{
			ReferenceID:     "txn-deposit",
			Date:            foldDate{Time: parsedDate},
			TransactionType: "Deposit",
			Description:     "External Deposit",
			Asset:           "BTC",
			AmountBTC:       "0.50000000",
			PricePerCoin:    "",
			SubtotalUSD:     "",
			FeeUSD:          "",
			TotalUSD:        "",
			TransactionID:   "abc123",
		}

		tx, err := fold.ToTransaction()
		is.NoErr(err)
		is.Equal(tx.TransactionType, "Deposit")
		is.Equal(tx.AmountBTC.Amount(), int64(50000000)) // 0.5 BTC in satoshis
		is.Equal(tx.PricePerCoin.Amount(), int64(0))     // Should be 0 when empty
		is.Equal(tx.TotalUSD.Amount(), int64(0))         // Should be 0 when empty
		is.Equal(tx.TransactionID, "abc123")
	})

	t.Run("invalid date format", func(t *testing.T) {
		_, err := time.Parse(foldDateTimeLayout, "invalid-date")
		is.True(err != nil)
	})

	t.Run("invalid BTC amount", func(t *testing.T) {
		parsedDate, err := time.Parse(foldDateTimeLayout, "2024-01-01 10:00:00.000000+00:00")
		is.NoErr(err)

		fold := Fold{
			ReferenceID:     "txn-bad",
			Date:            foldDate{Time: parsedDate},
			TransactionType: "Purchase",
			Description:     "Test",
			Asset:           "BTC",
			AmountBTC:       "not-a-number",
			PricePerCoin:    "40000.00",
			SubtotalUSD:     "-40000.00",
			FeeUSD:          "0.00",
			TotalUSD:        "-40000.00",
			TransactionID:   "",
		}

		_, err = fold.ToTransaction()
		is.True(err != nil)
	})

	t.Run("invalid price per coin", func(t *testing.T) {
		parsedDate, err := time.Parse(foldDateTimeLayout, "2024-01-01 10:00:00.000000+00:00")
		is.NoErr(err)

		fold := Fold{
			ReferenceID:     "txn-bad",
			Date:            foldDate{Time: parsedDate},
			TransactionType: "Purchase",
			Description:     "Test",
			Asset:           "BTC",
			AmountBTC:       "1.0",
			PricePerCoin:    "not-a-number",
			SubtotalUSD:     "-40000.00",
			FeeUSD:          "0.00",
			TotalUSD:        "-40000.00",
			TransactionID:   "",
		}

		_, err = fold.ToTransaction()
		is.True(err != nil)
	})
}

func TestParseAndMergeCSVs(t *testing.T) {
	is := is.New(t)

	t.Run("merge multiple CSV files", func(t *testing.T) {
		files := []string{
			"testdata/simple_transactions.csv",
			"testdata/deposits_withdrawals.csv",
		}

		transactions, err := parseAndMergeCSVs(files)
		is.NoErr(err)
		is.Equal(len(transactions), 5) // Exactly 3 from simple_transactions + 2 from deposits_withdrawals

		// Expected transactions in chronological order (both files have same Jan 1 date, then June 1)
		expectedRefs := []string{
			"txn-001",            // simple: 2024-01-01 Purchase (randomized time)
			"txn-deposit-001",    // deposits: 2024-01-01 Deposit (randomized time, same date as txn-001)
			"txn-002",            // simple: 2024-01-15 Purchase (randomized time)
			"txn-003",            // simple: 2024-06-01 Sale (randomized time)
			"txn-withdrawal-001", // deposits: 2024-06-01 Withdrawal (randomized time, same date as txn-003)
		}

		expectedTypes := []string{"Purchase", "Deposit", "Purchase", "Sale", "Withdrawal"}
		expectedAmounts := []int64{100000000, 100000000, 50000000, -25000000, -25000000} // BTC amounts in satoshis

		// Verify transactions are sorted by date and contain expected content
		for i := 0; i < len(transactions); i++ {
			if i > 0 {
				is.True(!transactions[i].Date.Before(transactions[i-1].Date))
			}
			is.Equal(transactions[i].ReferenceID, expectedRefs[i])
			is.Equal(transactions[i].TransactionType, expectedTypes[i])
			is.Equal(transactions[i].AmountBTC.Amount(), expectedAmounts[i])
		}
	})

	t.Run("handle duplicates in merged files", func(t *testing.T) {
		// Create duplicate file
		tempFile := filepath.Join(t.TempDir(), "duplicate.csv")

		// Copy content from simple_transactions.csv
		originalContent, err := os.ReadFile("testdata/simple_transactions.csv")
		is.NoErr(err)

		err = os.WriteFile(tempFile, originalContent, 0o644)
		is.NoErr(err)

		files := []string{
			"testdata/simple_transactions.csv",
			tempFile,
		}

		transactions, err := parseAndMergeCSVs(files)
		is.NoErr(err)

		// Should deduplicate - expect same number as in original file
		originalTransactions, err := parseCSV("testdata/simple_transactions.csv")
		is.NoErr(err)
		is.Equal(len(transactions), len(originalTransactions))
	})

	t.Run("nonexistent file in list", func(t *testing.T) {
		files := []string{
			"testdata/simple_transactions.csv",
			"testdata/nonexistent.csv",
		}

		_, err := parseAndMergeCSVs(files)
		is.True(err != nil)
	})

	t.Run("empty file list", func(t *testing.T) {
		transactions, err := parseAndMergeCSVs([]string{})
		is.NoErr(err)
		is.Equal(len(transactions), 0)
	})
}

// Test helper functions for CSV parsing edge cases
func TestCSVParsingEdgeCases(t *testing.T) {
	is := is.New(t)

	t.Run("transaction date parsing edge cases", func(t *testing.T) {
		// Test various timezone formats that might appear in Fold CSVs
		testDates := []string{
			"2024-01-01 10:00:00.000000+00:00",
			"2024-12-31 23:59:59.999999-08:00",
			"2024-06-15 12:30:45.123456+05:30",
		}

		for _, dateStr := range testDates {
			parsedDate, err := time.Parse(foldDateTimeLayout, dateStr)
			is.NoErr(err)

			fold := Fold{
				ReferenceID:     "test-id",
				Date:            foldDate{Time: parsedDate},
				TransactionType: "Purchase",
				Description:     "Test",
				Asset:           "BTC",
				AmountBTC:       "1.0",
				PricePerCoin:    "40000.00",
				SubtotalUSD:     "-40000.00",
				FeeUSD:          "0.00",
				TotalUSD:        "-40000.00",
				TransactionID:   "",
			}

			tx, err := fold.ToTransaction()
			is.NoErr(err)
			is.True(!tx.Date.IsZero())
		}
	})

	t.Run("precision in BTC amounts", func(t *testing.T) {
		// Test various BTC precision levels (up to 8 decimal places)
		testAmounts := []string{
			"1.00000000",        // 8 decimals
			"0.12345678",        // 8 decimals, fractional
			"100.1",             // 1 decimal
			"0.00000001",        // 1 satoshi
			"21000000.00000000", // Max BTC supply
		}

		for _, amountStr := range testAmounts {
			parsedDate, err := time.Parse(foldDateTimeLayout, "2024-01-01 10:00:00.000000+00:00")
			is.NoErr(err)

			fold := Fold{
				ReferenceID:     "test-id",
				Date:            foldDate{Time: parsedDate},
				TransactionType: "Purchase",
				Description:     "Test",
				Asset:           "BTC",
				AmountBTC:       amountStr,
				PricePerCoin:    "40000.00",
				SubtotalUSD:     "-40000.00",
				FeeUSD:          "0.00",
				TotalUSD:        "-40000.00",
				TransactionID:   "",
			}

			tx, err := fold.ToTransaction()
			is.NoErr(err)
			is.True(tx.AmountBTC.Amount() > 0)
		}
	})

	t.Run("large USD amounts", func(t *testing.T) {
		parsedDate, err := time.Parse(foldDateTimeLayout, "2024-01-01 10:00:00.000000+00:00")
		is.NoErr(err)

		fold := Fold{
			ReferenceID:     "test-id",
			Date:            foldDate{Time: parsedDate},
			TransactionType: "Purchase",
			Description:     "Test",
			Asset:           "BTC",
			AmountBTC:       "1.0",
			PricePerCoin:    "100000.00",
			SubtotalUSD:     "-100000.00",
			FeeUSD:          "100.00",
			TotalUSD:        "-100100.00",
			TransactionID:   "",
		}

		tx, err := fold.ToTransaction()
		is.NoErr(err)
		is.Equal(tx.PricePerCoin.Amount(), int64(10000000)) // $100k in cents
		is.Equal(tx.FeeUSD.Amount(), int64(10000))          // $100 in cents
		is.Equal(tx.TotalUSD.Amount(), int64(-10010000))    // -$100,100 in cents
	})
}

// Testify enhancement opportunities:
/*
The following functions could benefit from testify/suite for setup/teardown:
- TestParseCSV: Could use a test suite to create/cleanup temp files
- TestParseAndMergeCSVs: Could use suite for managing multiple test files

The following could benefit from testify/mock:
- File system operations could be mocked for error condition testing
- CSV reader could be mocked to test various failure scenarios

The following could benefit from testify/assert for more expressive assertions:
- Complex struct comparisons could use assert.Equal with better diff output
- Time comparisons could use assert.WithinDuration for more precise testing
- Money amount comparisons could use custom assertion helpers
*/
