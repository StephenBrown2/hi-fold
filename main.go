package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

var (
	targetYear     int
	inputFiles     []string
	outputFile     string
	useMockPrices  bool
	mempoolBaseURL string
)

var rootCmd = &cobra.Command{
	Use:   "hi-fold",
	Short: "Calculate HIFO cost basis for Bitcoin transactions",
	Long:  "Process Fold Bitcoin transaction CSV and calculate Optimized HIFO cost basis for tax reporting",
	Run:   runHIFO,
}

func init() {
	// Set up BTC currency for go-money
	money.AddCurrency("BTC", "₿", "1$", ".", ",", 8)

	currentYear := time.Now().Year()
	rootCmd.Flags().IntVarP(&targetYear, "year", "y", currentYear-1, "Tax year to calculate (default: previous year)")
	rootCmd.Flags().StringSliceVarP(&inputFiles, "input", "i", []string{}, "Input CSV files (can specify multiple)")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output CSV file (default: tax-records-{year}.csv)")
	rootCmd.Flags().BoolVarP(&useMockPrices, "mock-prices", "m", false, "Use mock prices instead of API for testing")
	rootCmd.Flags().StringVar(&mempoolBaseURL, "mempool-url", "", "Custom mempool API base URL (e.g., mempool.space, https://mempool.space, http://192.168.1.100:8080)")
}

// expandGlobPatterns expands glob patterns in file paths and returns actual file paths
func expandGlobPatterns(patterns []string) ([]string, error) {
	var expandedFiles []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern '%s': %v", pattern, err)
		}

		if len(matches) == 0 {
			// If no matches found, check if it's a literal file that doesn't exist
			if _, err := os.Stat(pattern); os.IsNotExist(err) {
				return nil, fmt.Errorf("no files match pattern or file does not exist: %s", pattern)
			}
			// If it's a literal file that exists, add it
			expandedFiles = append(expandedFiles, pattern)
		} else {
			expandedFiles = append(expandedFiles, matches...)
		}
	}

	return expandedFiles, nil
}

func main() {
	if err := fang.Execute(context.Background(), rootCmd, fang.WithoutVersion()); err != nil {
		log.Fatal(err)
	}
}

func runHIFO(cmd *cobra.Command, args []string) {
	if len(inputFiles) == 0 {
		log.Fatal("Error: At least one input CSV file must be specified with --input/-i")
	}

	if outputFile == "" {
		outputFile = fmt.Sprintf("tax-records-%d.csv", targetYear)
	}

	// Expand glob patterns to actual file paths
	expandedFiles, err := expandGlobPatterns(inputFiles)
	if err != nil {
		log.Fatalf("Error expanding file patterns: %v", err)
	}

	if len(expandedFiles) == 0 {
		log.Fatal("Error: No files found matching the specified patterns")
	}

	// Check that all expanded files exist and are readable
	for _, file := range expandedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			log.Fatalf("Error: Input file does not exist: %s", file)
		}
	}

	// Initialize price API
	var priceAPI PriceAPI
	if useMockPrices {
		priceAPI = NewMockPriceAPI()
		fmt.Println("Using mock prices for testing")
	} else {
		priceAPI = NewMempoolPriceAPIWithURL(mempoolBaseURL)
		if mempoolBaseURL != "" {
			fmt.Printf("Using custom mempool API: %s\n", mempoolBaseURL)
		} else {
			fmt.Println("Using mempool.space API for historical prices")
		}
	}

	// Parse and merge CSV files
	transactions, err := parseAndMergeCSVs(expandedFiles)
	if err != nil {
		log.Fatalf("Error parsing CSV files: %v", err)
	}

	fmt.Printf("Loaded %d unique transactions from %d file(s)\n", len(transactions), len(expandedFiles))

	// Calculate HIFO cost basis with all transactions (for proper remaining holdings calculation)
	lots, sales := calculateHIFO(transactions, priceAPI, targetYear)

	// Display results
	displayResults(lots, sales, transactions, targetYear, priceAPI)

	// Generate tax records CSV
	if err := generateTaxRecords(sales, outputFile); err != nil {
		log.Fatalf("Error generating tax records: %v", err)
	}

	fmt.Printf("\nTax records saved to: %s\n", outputFile)
}
