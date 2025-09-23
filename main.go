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
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
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

func displayResults(lots []Lot, sales []Sale, transactions []Transaction, year int, priceAPI PriceAPI) {
	fmt.Println(titleStyle.Render(fmt.Sprintf("Bitcoin HIFO Cost Basis Report - %d", year)))

	// Summary table
	summaryTable := newTable().
		StyleFunc(summaryTableStyleFunc()).
		Headers("Metric", "Value")

	totalProceeds := money.New(0, money.USD)
	totalCostBasis := money.New(0, money.USD)
	totalGainLoss := money.New(0, money.USD)
	totalSales := len(sales)

	for _, sale := range sales {
		totalProceeds, _ = totalProceeds.Add(sale.ProceedsUSD)
		totalCostBasis, _ = totalCostBasis.Add(sale.CostBasisUSD)
		totalGainLoss, _ = totalGainLoss.Add(sale.GainLossUSD)
	}

	summaryTable.Row("Total Sales", fmt.Sprintf("%d", totalSales))
	summaryTable.Row("Total Proceeds", totalProceeds.Display())
	summaryTable.Row("Total Cost Basis", totalCostBasis.Display())
	summaryTable.Row("Total Gain/Loss", displayRedGreen(totalGainLoss))

	fmt.Println(summaryTable.Render())
	fmt.Println()

	// Sales detail table
	if len(sales) > 0 {
		salesTable := newTable().
			StyleFunc(monetaryTableStyleFunc()).
			Headers("Date Sold", "Amount (BTC)", "Proceeds ($)", "Cost Basis ($)", "Price/BTC", "Gain/Loss ($)")

		for _, sale := range sales {
			// Calculate average price per BTC: proceeds ÷ amount
			// Note: go-money doesn't have division, so we convert to float64 for price calculations
			proceedsFloat := float64(sale.ProceedsUSD.Amount()) / 100   // go-money uses smallest unit
			amountFloat := float64(sale.AmountBTC.Amount()) / 100000000 // 8 decimal places for BTC
			avgPriceFloat := proceedsFloat / amountFloat
			avgPrice := money.NewFromFloat(avgPriceFloat, money.USD)

			salesTable.Row(
				sale.Date.Format("2006-01-02"),
				sale.AmountBTC.Display(),
				sale.ProceedsUSD.Display(),
				sale.CostBasisUSD.Display(),
				avgPrice.Display(),
				displayRedGreen(sale.GainLossUSD),
			)
		}

		fmt.Println(titleStyle.Render("Sales Details"))
		fmt.Println(salesTable.Render())
	}

	// Remaining lots
	remainingLots := []Lot{}
	for _, lot := range lots {
		if !lot.Remaining.IsZero() {
			remainingLots = append(remainingLots, lot)
		}
	}

	if len(remainingLots) > 0 {
		fmt.Println()
		fmt.Println(titleStyle.Render("Remaining Holdings"))

		lotsTable := newTable().
			StyleFunc(monetaryTableStyleFunc()).
			Headers("Date Acquired", "Amount (BTC)", "Cost Basis ($)", "Price/BTC")

		for _, lot := range remainingLots {
			// Calculate price per BTC and cost basis for remaining amount
			// Note: go-money doesn't have division, so we convert to float64 for price calculations
			costBasisFloat := float64(lot.CostBasisUSD.Amount()) / 100    // USD in smallest unit
			amountFloat := float64(lot.AmountBTC.Amount()) / 100000000    // BTC in smallest unit (8 decimals)
			remainingFloat := float64(lot.Remaining.Amount()) / 100000000 // BTC in smallest unit

			pricePerBTCFloat := costBasisFloat / amountFloat
			costBasisForRemainingFloat := pricePerBTCFloat * remainingFloat

			pricePerBTC := money.NewFromFloat(pricePerBTCFloat, money.USD)
			costBasisForRemaining := money.NewFromFloat(costBasisForRemainingFloat, money.USD)

			lotsTable.Row(
				lot.Date.Format("2006-01-02"),
				lot.Remaining.Display(),
				costBasisForRemaining.Display(),
				pricePerBTC.Display(),
			)
		}

		fmt.Println(lotsTable.Render())

		// Calculate and display summary for remaining holdings
		totalRemainingBTC := money.New(0, "BTC")
		totalCostBasisRemaining := money.New(0, money.USD)
		for _, lot := range remainingLots {
			totalRemainingBTC, _ = totalRemainingBTC.Add(lot.Remaining)

			// Calculate cost basis for remaining amount: (cost/amount) * remaining
			// Note: go-money doesn't have division, so we convert to float64 for calculations
			costBasisFloat := float64(lot.CostBasisUSD.Amount()) / 100    // USD in smallest unit
			amountFloat := float64(lot.AmountBTC.Amount()) / 100000000    // BTC in smallest unit
			remainingFloat := float64(lot.Remaining.Amount()) / 100000000 // BTC in smallest unit
			costBasisForRemainingFloat := (costBasisFloat / amountFloat) * remainingFloat
			costBasisForRemaining := money.NewFromFloat(costBasisForRemainingFloat, money.USD)

			totalCostBasisRemaining, _ = totalCostBasisRemaining.Add(costBasisForRemaining)
		}

		// Always show holdings summary
		fmt.Println()

		// Holdings summary table
		summaryTable := newTable().
			StyleFunc(summaryTableStyleFunc()).
			Headers("Holdings Summary", "Value")

		summaryTable.Row("Net BTC Position", totalRemainingBTC.Display())

		if !totalRemainingBTC.IsZero() {
			// Calculate average cost basis: total cost ÷ total BTC
			// Note: go-money doesn't have division, so we convert to float64 for calculations
			totalCostFloat := float64(totalCostBasisRemaining.Amount()) / 100 // USD in smallest unit
			totalBTCFloat := float64(totalRemainingBTC.Amount()) / 100000000  // BTC in smallest unit
			avgCostBasisFloat := totalCostFloat / totalBTCFloat
			avgCostBasis := money.NewFromFloat(avgCostBasisFloat, money.USD)

			summaryTable.Row("Average BTC Price", avgCostBasis.Display())
			summaryTable.Row("Total Cost Basis", totalCostBasisRemaining.Display())

			// Get current price
			currentPrice, err := priceAPI.GetCurrentPriceUSD()
			if err != nil {
				fmt.Printf("Warning: Could not fetch current price: %v\n", err)
				currentPrice = 0
			}

			if currentPrice > 0 {
				// Calculate current value: BTC amount * current price
				// Note: go-money doesn't have methods for multiplying by float64, so we convert for calculations
				currentValueFloat := totalBTCFloat * currentPrice
				currentValue := money.NewFromFloat(currentValueFloat, money.USD)

				summaryTable.Row("Current BTC Price", money.NewFromFloat(currentPrice, money.USD).Display())
				summaryTable.Row("Current Value", currentValue.Display())

				unrealizedGainLoss, _ := currentValue.Subtract(totalCostBasisRemaining)
				summaryTable.Row("Unrealized Gain/Loss", displayRedGreen(unrealizedGainLoss))
			}
		}

		fmt.Println(summaryTable.Render())
	}
}
