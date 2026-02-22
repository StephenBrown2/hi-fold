package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/jszwec/csvutil"
)

// CSVFormat identifies the format of the CSV file
type CSVFormat int

const (
	CSVFormatUnknown     CSVFormat = iota
	CSVFormatFold                  // Fold format (current default)
	CSVFormatRiver                 // River format
	CSVFormatKoinly                // Koinly format
	CSVFormatCoinTracker           // CoinTracker format
	CSVFormatCoinLedger            // CoinLedger format
	CSVFormatStrike                // Strike format
)

func (f CSVFormat) String() string {
	switch f {
	case CSVFormatFold:
		return "Fold"
	case CSVFormatRiver:
		return "River"
	case CSVFormatKoinly:
		return "Koinly"
	case CSVFormatCoinTracker:
		return "CoinTracker"
	case CSVFormatCoinLedger:
		return "CoinLedger"
	case CSVFormatStrike:
		return "Strike"
	default:
		return "Unknown"
	}
}

func parseCSV(filename string) ([]Transaction, error) {
	format, transactions, err := detectCSVFormat(filename)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Detected CSV format: %s\n", format)

	return transactions, nil
}

func detectCSVFormat(filename string) (CSVFormat, []Transaction, error) {
	type parserAttempt struct {
		format          CSVFormat
		requiredHeaders []string
		parse           func(string, []string) ([]Transaction, error)
	}

	attempts := []parserAttempt{
		{
			format:          CSVFormatFold,
			requiredHeaders: []string{"referenceid", "dateutc", "transactiontype", "amountbtc"},
			parse: func(path string, headers []string) ([]Transaction, error) {
				return parseModelCSV[Fold](path, headers)
			},
		},
		{
			format:          CSVFormatStrike,
			requiredHeaders: []string{"transactionid", "timeutc", "status", "transactiontype", "amountbtc", "amountusd"},
			parse: func(path string, headers []string) ([]Transaction, error) {
				return parseModelCSV[Strike](path, headers)
			},
		},
		{
			format:          CSVFormatRiver,
			requiredHeaders: []string{"date", "sentamount", "sentcurrency", "receivedamount", "receivedcurrency", "feeamount", "feecurrency", "tag"},
			parse: func(path string, headers []string) ([]Transaction, error) {
				return parseModelCSV[River](path, headers)
			},
		},
		{
			format:          CSVFormatKoinly,
			requiredHeaders: []string{"date", "sentamount", "sentcurrency", "receivedamount", "receivedcurrency", "feeamount", "feecurrency", "label"},
			parse: func(path string, headers []string) ([]Transaction, error) {
				return parseModelCSV[Koinly](path, headers)
			},
		},
		{
			format:          CSVFormatCoinTracker,
			requiredHeaders: []string{"date", "receivedquantity", "receivedcurrency", "sentquantity", "sentcurrency", "feeamount", "feecurrency", "tag"},
			parse: func(path string, headers []string) ([]Transaction, error) {
				return parseModelCSV[CoinTracker](path, headers)
			},
		},
		{
			format:          CSVFormatCoinLedger,
			requiredHeaders: []string{"dateutc", "assetsent", "amountsent", "assetreceived", "amountreceived", "type"},
			parse: func(path string, headers []string) ([]Transaction, error) {
				return parseModelCSV[CoinLedger](path, headers)
			},
		},
	}

	for _, attempt := range attempts {
		transactions, err := attempt.parse(filename, attempt.requiredHeaders)
		if err == nil {
			return attempt.format, transactions, nil
		}
	}

	return CSVFormatUnknown, nil, fmt.Errorf("unable to detect CSV format; supported formats: Fold, Strike, River, Koinly, CoinTracker, CoinLedger")
}

type transactionModel interface {
	ToTransaction() (Transaction, error)
}

func parseModelCSV[T transactionModel](filename string, requiredHeaders []string) ([]Transaction, error) {
	headerRowIndex, err := findHeaderRowIndexInFile(filename, requiredHeaders)
	if err != nil {
		return nil, err
	}
	if headerRowIndex == -1 {
		return nil, fmt.Errorf("required headers not found")
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	for i := 0; i < headerRowIndex; i++ {
		if _, err := reader.Read(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("required headers not found")
			}
			return nil, err
		}
	}

	decoder, err := csvutil.NewDecoder(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to build decoder: %w", err)
	}

	transactions := make([]Transaction, 0)
	rowNumber := headerRowIndex + 2
	for {
		var record T
		if err := decoder.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			fmt.Printf("Warning: skipping invalid CSV row %d: %v\n", rowNumber, err)
			rowNumber++
			continue
		}

		tx, err := record.ToTransaction()
		if err != nil {
			if errors.Is(err, errSkipTransaction) {
				rowNumber++
				continue
			}
			fmt.Printf("Warning: skipping invalid transaction at row %d: %v\n", rowNumber, err)
			rowNumber++
			continue
		}
		transactions = append(transactions, tx)
		rowNumber++
	}

	return transactions, nil
}

func findHeaderRowIndexInFile(filename string, requiredHeaders []string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return -1, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1

	for i := 0; ; i++ {
		row, err := reader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return -1, err
		}

		headerSet := make(map[string]struct{}, len(row))
		for _, col := range row {
			headerSet[canonicalHeader(col)] = struct{}{}
		}

		allPresent := true
		for _, required := range requiredHeaders {
			if _, ok := headerSet[required]; !ok {
				allPresent = false
				break
			}
		}

		if allPresent {
			return i, nil
		}
	}

	return -1, nil
}

func canonicalHeader(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	replacer := strings.NewReplacer("\"", "", " ", "", "_", "", "-", "", "(", "", ")", "", ".", "", "/", "")
	return replacer.Replace(normalized)
}

// parseAndMergeCSVs parses multiple CSV files and merges them, deduplicating by Reference ID
func parseAndMergeCSVs(filenames []string) ([]Transaction, error) {
	groupedTransactions, _, err := parseAndMergeCSVsByFormat(filenames)
	if err != nil {
		return nil, err
	}

	// Convert map back to slice
	var allTransactions []Transaction
	for _, format := range slices.Sorted(maps.Keys(groupedTransactions)) {
		allTransactions = append(allTransactions, groupedTransactions[format]...)
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

// parseAndMergeCSVsByFormat parses CSV files, groups by detected format, and deduplicates by Reference ID within each format.
func parseAndMergeCSVsByFormat(filenames []string) (map[CSVFormat][]Transaction, map[CSVFormat][]string, error) {
	groupedTransactionMap := make(map[CSVFormat]map[string]Transaction)
	groupedFiles := make(map[CSVFormat][]string)

	for i, filename := range filenames {
		fmt.Printf("Processing file %d/%d: %s\n", i+1, len(filenames), filename)

		format, transactions, err := detectCSVFormat(filename)
		if err != nil {
			return nil, nil, fmt.Errorf("error parsing file %s: %w", filename, err)
		}

		fmt.Printf("Detected CSV format: %s\n", format)

		if _, exists := groupedTransactionMap[format]; !exists {
			groupedTransactionMap[format] = make(map[string]Transaction)
		}

		groupedFiles[format] = append(groupedFiles[format], filename)

		duplicateCount := 0
		for _, tx := range transactions {
			if _, exists := groupedTransactionMap[format][tx.ReferenceID]; exists {
				duplicateCount++
				fmt.Printf("  Duplicate %s transaction found (Reference ID: %s), keeping first occurrence\n", format, tx.ReferenceID)
			} else {
				groupedTransactionMap[format][tx.ReferenceID] = tx
			}
		}

		fmt.Printf("  Loaded %d %s transactions (%d duplicates skipped)\n", len(transactions)-duplicateCount, format, duplicateCount)
	}

	groupedTransactions := make(map[CSVFormat][]Transaction, len(groupedTransactionMap))
	for format, txMap := range groupedTransactionMap {
		transactions := make([]Transaction, 0, len(txMap))
		for _, tx := range txMap {
			transactions = append(transactions, tx)
		}

		slices.SortStableFunc(transactions, func(a, b Transaction) int {
			if cmp := a.Date.Compare(b.Date); cmp != 0 {
				return cmp
			}
			return strings.Compare(a.ReferenceID, b.ReferenceID)
		})

		groupedTransactions[format] = transactions
	}

	return groupedTransactions, groupedFiles, nil
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
	boxHeaders := []string{
		"Box 1(a)",
		"Box 1(b)",
		"Box 1(c)",
		"Box 1(d)",
		"Box 1(e)",
		"Box 1(h)",
	}

	headers := []string{
		"Description",
		"Date Acquired",
		"Date Sold",
		"Proceeds",
		"Cost Basis",
		"Gain/Loss",
	}

	boxTotals := []string{
		"",
		"",
		"",
		"Box 2(d)",
		"Box 2(e)",
		"Box 2(h)",
	}
	totalsHeaders := []string{
		"",
		"",
		"",
		"Total Proceeds",
		"Total Cost Basis",
		"Total Gain/Loss",
	}

	// Write Short-Term Capital Gains section
	if len(shortTermRecords) > 0 {
		if err := writer.Write([]string{"SHORT-TERM CAPITAL GAINS AND LOSSES (Form 8949 Part I)"}); err != nil {
			return err
		}
		if err := writer.Write([]string{}); err != nil { // Empty line
			return err
		}
		if err := writer.Write(boxHeaders); err != nil {
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

		// Add totals box row
		if err := writer.Write(boxTotals); err != nil {
			return err
		}
		// Add totals box headers
		if err := writer.Write(totalsHeaders); err != nil {
			return err
		}

		// Add totals row
		totalsRow := []string{
			"",
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
		if err := writer.Write(boxHeaders); err != nil {
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

		// Add totals box row
		if err := writer.Write(boxTotals); err != nil {
			return err
		}
		// Add totals box headers
		if err := writer.Write(totalsHeaders); err != nil {
			return err
		}

		// Add totals row
		totalsRow := []string{
			"",
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
