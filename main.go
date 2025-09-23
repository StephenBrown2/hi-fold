package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

var (
	targetYear    int
	inputFiles    []string
	outputFile    string
	useMockPrices bool
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

	// Check that all input files exist
	for _, file := range inputFiles {
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
		priceAPI = NewMempoolPriceAPI()
		fmt.Println("Using mempool.space API for historical prices")
	}

	// Parse and merge CSV files
	transactions, err := parseAndMergeCSVs(inputFiles)
	if err != nil {
		log.Fatalf("Error parsing CSV files: %v", err)
	}

	fmt.Printf("Loaded %d unique transactions from %d file(s)\n", len(transactions), len(inputFiles))

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
