package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Rhymond/go-money"
)

func parseCSV(filename string) ([]Transaction, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	var transactions []Transaction

	// Skip header rows and account info (first 4 rows)
	// Find the transaction header row
	headerRowIndex := -1
	for i, record := range records {
		if len(record) > 0 && record[0] == "Reference ID" {
			headerRowIndex = i
			break
		}
	}

	if headerRowIndex == -1 {
		return nil, fmt.Errorf("could not find transaction header row")
	}

	// Parse transaction rows
	for i := headerRowIndex + 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 11 || record[0] == "" || len(record) == 1 {
			continue // Skip empty rows, footer, or incomplete rows
		}

		transaction, err := parseTransaction(record)
		if err != nil {
			fmt.Printf("Warning: skipping invalid transaction at row %d: %v\n", i+1, err)
			continue
		}

		transactions = append(transactions, transaction)
	}

	return transactions, nil
}

// parseAndMergeCSVs parses multiple CSV files and merges them, deduplicating by Reference ID
func parseAndMergeCSVs(filenames []string) ([]Transaction, error) {
	transactionMap := make(map[string]Transaction) // Use Reference ID as key for deduplication

	for i, filename := range filenames {
		fmt.Printf("Processing file %d/%d: %s\n", i+1, len(filenames), filename)

		transactions, err := parseCSV(filename)
		if err != nil {
			return nil, fmt.Errorf("error parsing file %s: %w", filename, err)
		}

		// Add transactions to map, deduplicating by Reference ID
		duplicateCount := 0
		for _, tx := range transactions {
			if _, exists := transactionMap[tx.ReferenceID]; exists {
				duplicateCount++
				fmt.Printf("  Duplicate transaction found (Reference ID: %s), keeping first occurrence\n", tx.ReferenceID)
			} else {
				transactionMap[tx.ReferenceID] = tx
			}
		}

		fmt.Printf("  Loaded %d transactions (%d duplicates skipped)\n", len(transactions)-duplicateCount, duplicateCount)
	}

	// Convert map back to slice
	var allTransactions []Transaction
	for _, tx := range transactionMap {
		allTransactions = append(allTransactions, tx)
	}

	// Sort chronologically by date, with ReferenceID as tie-breaker for deterministic ordering
	slices.SortStableFunc(allTransactions, func(a, b Transaction) int {
		if cmp := a.Date.Compare(b.Date); cmp != 0 {
			return cmp
		}
		// Use ReferenceID as tie-breaker for consistent ordering
		return strings.Compare(a.ReferenceID, b.ReferenceID)
	})

	return allTransactions, nil
}

func parseTransaction(record []string) (Transaction, error) {
	// Parse date
	dateStr := record[1]
	date, err := time.Parse("2006-01-02 15:04:05.999999-07:00", dateStr)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid date format: %s", dateStr)
	}

	// Parse BTC amount
	amountBTC, err := newBTCFromString(record[5])
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid BTC amount: %s", record[5])
	}

	// Parse price per coin (may be empty for deposits)
	pricePerCoin := money.New(0, money.USD)
	if record[6] != "" {
		pricePerCoinFloat, err := strconv.ParseFloat(record[6], 64)
		if err != nil {
			return Transaction{}, fmt.Errorf("invalid price per coin: %s", record[6])
		}
		pricePerCoin = money.NewFromFloat(pricePerCoinFloat, money.USD)
	}

	// Parse subtotal USD
	subtotalUSD := money.New(0, money.USD)
	if record[7] != "" {
		subtotalUSDFloat, err := strconv.ParseFloat(record[7], 64)
		if err != nil {
			return Transaction{}, fmt.Errorf("invalid subtotal USD: %s", record[7])
		}
		subtotalUSD = money.NewFromFloat(subtotalUSDFloat, money.USD)
	}

	// Parse fee USD
	feeUSD := money.New(0, money.USD)
	if record[8] != "" {
		feeUSDFloat, err := strconv.ParseFloat(record[8], 64)
		if err != nil {
			return Transaction{}, fmt.Errorf("invalid fee USD: %s", record[8])
		}
		feeUSD = money.NewFromFloat(feeUSDFloat, money.USD)
	}

	// Parse total USD
	totalUSD := money.New(0, money.USD)
	if record[9] != "" {
		totalUSDFloat, err := strconv.ParseFloat(record[9], 64)
		if err != nil {
			return Transaction{}, fmt.Errorf("invalid total USD: %s", record[9])
		}
		totalUSD = money.NewFromFloat(totalUSDFloat, money.USD)
	}

	return Transaction{
		ReferenceID:     record[0],
		Date:            date,
		TransactionType: record[2],
		Description:     record[3],
		Asset:           record[4],
		AmountBTC:       amountBTC,
		PricePerCoin:    pricePerCoin,
		SubtotalUSD:     subtotalUSD,
		FeeUSD:          feeUSD,
		TotalUSD:        totalUSD,
		TransactionID:   record[10],
	}, nil
}

// generateTaxRecords creates a CSV file with tax records for IRS Form 8949
// Separates short-term and long-term gains as required by tax regulations
func generateTaxRecords(sales []Sale, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Separate transactions into short-term and long-term
	var shortTermRecords [][]string
	var longTermRecords [][]string

	var shortTermProceeds, shortTermCostBasis, shortTermGainLoss float64
	var longTermProceeds, longTermCostBasis, longTermGainLoss float64

	for _, sale := range sales {
		for _, lotSale := range sale.Lots {
			// Calculate proceeds and gain/loss for this lot
			proceedsFloat := float64(sale.ProceedsUSD.Amount()) / 100         // USD in smallest unit
			saleAmountFloat := float64(sale.AmountBTC.Amount()) / 100000000   // BTC in smallest unit
			lotAmountFloat := float64(lotSale.AmountBTC.Amount()) / 100000000 // BTC in smallest unit
			costBasisFloat := float64(lotSale.CostBasisUSD.Amount()) / 100    // USD in smallest unit

			pricePerBTC := proceedsFloat / saleAmountFloat
			lotProceeds := pricePerBTC * lotAmountFloat
			lotGainLoss := lotProceeds - costBasisFloat

			record := []string{
				fmt.Sprintf("%.8f BTC", lotAmountFloat),
				lotSale.LotDate.Format("01/02/2006"),
				sale.Date.Format("01/02/2006"),
				fmt.Sprintf("%.2f", lotProceeds),
				fmt.Sprintf("%.2f", costBasisFloat),
				fmt.Sprintf("%.2f", lotGainLoss),
			}

			if lotSale.IsLongTerm {
				longTermRecords = append(longTermRecords, record)
				longTermProceeds += lotProceeds
				longTermCostBasis += costBasisFloat
				longTermGainLoss += lotGainLoss
			} else {
				shortTermRecords = append(shortTermRecords, record)
				shortTermProceeds += lotProceeds
				shortTermCostBasis += costBasisFloat
				shortTermGainLoss += lotGainLoss
			}
		}
	}

	// Write headers
	headers := []string{
		"Description",
		"Date Acquired",
		"Date Sold",
		"Proceeds",
		"Cost Basis",
		"Gain/Loss",
	}

	// Write Short-Term Capital Gains section
	if len(shortTermRecords) > 0 {
		if err := writer.Write([]string{"SHORT-TERM CAPITAL GAINS AND LOSSES (Form 8949 Part I)"}); err != nil {
			return err
		}
		if err := writer.Write([]string{}); err != nil { // Empty line
			return err
		}
		if err := writer.Write(headers); err != nil {
			return err
		}

		for _, record := range shortTermRecords {
			if err := writer.Write(record); err != nil {
				return err
			}
		}

		// Add totals row
		totalsRow := []string{
			"TOTALS:",
			"",
			"",
			fmt.Sprintf("%.2f", shortTermProceeds),
			fmt.Sprintf("%.2f", shortTermCostBasis),
			fmt.Sprintf("%.2f", shortTermGainLoss),
		}
		if err := writer.Write(totalsRow); err != nil {
			return err
		}

		// Add separator
		if err := writer.Write([]string{}); err != nil {
			return err
		}
		if err := writer.Write([]string{}); err != nil {
			return err
		}
	}

	// Write Long-Term Capital Gains section
	if len(longTermRecords) > 0 {
		if err := writer.Write([]string{"LONG-TERM CAPITAL GAINS AND LOSSES (Form 8949 Part II)"}); err != nil {
			return err
		}
		if err := writer.Write([]string{}); err != nil { // Empty line
			return err
		}
		if err := writer.Write(headers); err != nil {
			return err
		}

		for _, record := range longTermRecords {
			if err := writer.Write(record); err != nil {
				return err
			}
		}

		// Add totals row
		totalsRow := []string{
			"TOTALS:",
			"",
			"",
			fmt.Sprintf("%.2f", longTermProceeds),
			fmt.Sprintf("%.2f", longTermCostBasis),
			fmt.Sprintf("%.2f", longTermGainLoss),
		}
		if err := writer.Write(totalsRow); err != nil {
			return err
		}
	}

	return nil
}
