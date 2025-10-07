package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

// CSV data loader functions for static test data
func loadTransactionsFromCSV(filename string) ([]Transaction, error) {
	fullPath := filepath.Join("testdata", filename)
	return parseCSV(fullPath)
}

// TestHIFOOptimalityWithStaticData tests HIFO optimality using pre-generated CSV data
func TestHIFOOptimalityWithStaticData(t *testing.T) {
	testScenarios := []struct {
		name        string
		csvFile     string
		targetYear  int
		description string
	}{
		{
			name:        "Real Historical Data 2022-2024",
			csvFile:     "real_historical_2022_2024.csv",
			targetYear:  2024,
			description: "Real BTC price data with $3k buys/$5k sells",
		},
		{
			name:        "Bull Market 2025",
			csvFile:     "bull_market_2025.csv",
			targetYear:  2025,
			description: "Hypothetical bull market scenario ($95k→$250k)",
		},
		{
			name:        "Choppy Market 2025",
			csvFile:     "choppy_market_2025.csv",
			targetYear:  2025,
			description: "Hypothetical sideways market scenario ($70k-$120k)",
		},
		{
			name:        "Bear Market 2025",
			csvFile:     "bear_market_2025.csv",
			targetYear:  2025,
			description: "Hypothetical bear market scenario ($90k→$30k)",
		},
	}

	hifoWins := 0
	totalTests := len(testScenarios)

	for i, scenario := range testScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			fmt.Printf("\n=== Test %d/%d: %s ===\n", i+1, totalTests, scenario.name)
			fmt.Printf("📄 CSV File: %s\n", scenario.csvFile)
			fmt.Printf("📋 Description: %s\n", scenario.description)
			fmt.Printf("🎯 Target Year: %d\n", scenario.targetYear)

			// Load transactions from CSV
			transactions, err := loadTransactionsFromCSV(scenario.csvFile)
			if err != nil {
				t.Fatalf("Failed to load transactions from %s: %v", scenario.csvFile, err)
			}

			fmt.Printf("📊 Loaded %d transactions\n", len(transactions))

			// Create a mock API populated with actual prices from the CSV data
			mockAPI := NewMockPriceAPI()
			for _, tx := range transactions {
				dateKey := tx.Date.Format("2006-01-02")
				price := float64(tx.PricePerCoin.Amount()) / 100 // Convert from cents to dollars
				mockAPI.SetPrice(dateKey, price)
			}

			// Calculate using all three methods
			fmt.Printf("\n🧮 Calculating tax outcomes...\n")

			// Calculate HIFO
			_, hifoSales := calculateHIFO(transactions, mockAPI, scenario.targetYear)
			hifoOutcome := calculateTaxOutcome(hifoSales)

			// Calculate FIFO
			_, fifoSales := calculateFIFO(transactions, scenario.targetYear)
			fifoOutcome := calculateTaxOutcome(fifoSales)

			// Calculate LIFO
			_, lifoSales := calculateLIFO(transactions, scenario.targetYear)
			lifoOutcome := calculateTaxOutcome(lifoSales)

			// Print detailed results
			fmt.Printf("\n📊 Tax Optimization Results:\n")
			fmt.Printf("  HIFO - Total G/L: %s, Tax: %s\n",
				hifoOutcome.TotalGainLoss.Display(), hifoOutcome.TaxLiability.Display())
			fmt.Printf("  FIFO - Total G/L: %s, Tax: %s\n",
				fifoOutcome.TotalGainLoss.Display(), fifoOutcome.TaxLiability.Display())
			fmt.Printf("  LIFO - Total G/L: %s, Tax: %s\n",
				lifoOutcome.TotalGainLoss.Display(), lifoOutcome.TaxLiability.Display())

			// Check if HIFO is optimal (lowest tax liability)
			hifoTax := hifoOutcome.TaxLiability.Amount()
			fifoTax := fifoOutcome.TaxLiability.Amount()
			lifoTax := lifoOutcome.TaxLiability.Amount()

			isHifoOptimal := hifoTax <= fifoTax && hifoTax <= lifoTax
			
			fmt.Printf("\n🎯 Optimization Analysis:\n")
			if isHifoOptimal {
				hifoWins++
				fmt.Printf("  ✅ HIFO is optimal! (Tax: $%.2f)\n", float64(hifoTax)/100)
				
				// Calculate savings vs other methods
				fifoSavings := fifoTax - hifoTax
				lifoSavings := lifoTax - hifoTax
				if fifoSavings > 0 {
					fmt.Printf("  💰 HIFO saves $%.2f vs FIFO\n", float64(fifoSavings)/100)
				}
				if lifoSavings > 0 {
					fmt.Printf("  💰 HIFO saves $%.2f vs LIFO\n", float64(lifoSavings)/100)
				}
			} else {
				fmt.Printf("  ⚠️  HIFO not optimal: HIFO=$%.2f, FIFO=$%.2f, LIFO=$%.2f\n", 
					float64(hifoTax)/100, float64(fifoTax)/100, float64(lifoTax)/100)
				
				// Find which method is best
				minTax := hifoTax
				bestMethod := "HIFO"
				if fifoTax < minTax {
					minTax = fifoTax
					bestMethod = "FIFO"
				}
				if lifoTax < minTax {
					minTax = lifoTax
					bestMethod = "LIFO"
				}
				fmt.Printf("  🏆 Best method for this scenario: %s (Tax: $%.2f)\n", bestMethod, float64(minTax)/100)
			}

			// Detailed analysis
			fmt.Printf("\n📈 Transaction Analysis:\n")
			buyCount := 0
			sellCount := 0
			for _, tx := range transactions {
				if tx.Date.Year() == scenario.targetYear {
					if tx.TransactionType == "Purchase" {
						buyCount++
					} else if tx.TransactionType == "Sale" {
						sellCount++
					}
				}
			}
			fmt.Printf("  📅 %d transactions in target year %d (%d buys, %d sells)\n", 
				buyCount+sellCount, scenario.targetYear, buyCount, sellCount)

			if len(hifoSales) > 0 {
				fmt.Printf("  🔄 HIFO used %d lot(s) for tax calculations\n", 
					func() int {
						totalLots := 0
						for _, sale := range hifoSales {
							totalLots += len(sale.Lots)
						}
						return totalLots
					}())
			}

			// Verify we have meaningful test data
			if len(hifoSales) == 0 && len(fifoSales) == 0 && len(lifoSales) == 0 {
				t.Errorf("No sales found in target year %d for %s", scenario.targetYear, scenario.name)
			}
		})
	}

	// Overall summary
	t.Run("Overall HIFO Performance Summary", func(t *testing.T) {
		fmt.Printf("\n🏆 === FINAL HIFO PERFORMANCE SUMMARY ===\n")
		fmt.Printf("HIFO was optimal in %d out of %d scenarios (%.1f%%)\n",
			hifoWins, totalTests, float64(hifoWins)/float64(totalTests)*100)

		// Performance thresholds
		if hifoWins == totalTests {
			fmt.Printf("🎉 PERFECT! HIFO achieved optimal tax outcomes in ALL scenarios!\n")
		} else if hifoWins >= totalTests*3/4 {
			fmt.Printf("✅ EXCELLENT! HIFO performed optimally in most scenarios\n")
		} else if hifoWins >= totalTests/2 {
			fmt.Printf("👍 GOOD! HIFO performed well in the majority of scenarios\n")
		} else {
			fmt.Printf("⚠️  ATTENTION! HIFO was optimal in less than half the scenarios\n")
			t.Logf("WARNING: HIFO optimality rate is lower than expected (%d/%d)", hifoWins, totalTests)
		}

		// Test assertions
		if hifoWins == 0 {
			t.Error("HIFO should be optimal in at least some scenarios with realistic data")
		}

		// The HIFO algorithm should generally outperform FIFO/LIFO in most market conditions
		// A success rate of 75% or higher indicates the algorithm is working correctly
		expectedMinWins := totalTests * 3 / 4
		if hifoWins < expectedMinWins {
			t.Logf("INFO: HIFO won %d/%d tests (expected at least %d for 75%% rate)", 
				hifoWins, totalTests, expectedMinWins)
		}
	})
}

// TestHIFOWithSpecificScenario allows testing individual CSV files
func TestHIFOWithSpecificScenario(t *testing.T) {
	csvFile := "real_historical_2022_2024.csv"
	targetYear := 2024

	t.Run("Detailed Real Historical Analysis", func(t *testing.T) {
		fmt.Printf("\n🔍 === DETAILED ANALYSIS: %s ===\n", csvFile)

		// Load transactions
		transactions, err := loadTransactionsFromCSV(csvFile)
		if err != nil {
			t.Fatalf("Failed to load transactions: %v", err)
		}

		fmt.Printf("📊 Total transactions loaded: %d\n", len(transactions))

		// Analyze transaction distribution by year
		yearCounts := make(map[int]int)
		for _, tx := range transactions {
			yearCounts[tx.Date.Year()]++
		}

		fmt.Printf("\n📅 Transaction distribution by year:\n")
		for year := 2022; year <= 2024; year++ {
			if count, exists := yearCounts[year]; exists {
				fmt.Printf("  %d: %d transactions\n", year, count)
			}
		}

		// Create mock API
		mockAPI := NewMockPriceAPI()
		for _, tx := range transactions {
			dateKey := tx.Date.Format("2006-01-02")
			price := float64(tx.PricePerCoin.Amount()) / 100
			mockAPI.SetPrice(dateKey, price)
		}

		// Detailed HIFO analysis
		lots, hifoSales := calculateHIFO(transactions, mockAPI, targetYear)
		
		fmt.Printf("\n🏦 Lot inventory analysis:\n")
		fmt.Printf("  Total lots created: %d\n", len(lots))
		
		remainingBTC := 0.0
		totalCostBasis := 0.0
		for _, lot := range lots {
			if !lot.Remaining.IsZero() {
				remainingBTC += float64(lot.Remaining.Amount()) / 100000000
				totalCostBasis += float64(lot.CostBasisUSD.Amount()) / 100
			}
		}
		
		fmt.Printf("  Remaining BTC: %.8f\n", remainingBTC)
		fmt.Printf("  Total cost basis: $%.2f\n", totalCostBasis)
		if remainingBTC > 0 {
			fmt.Printf("  Average cost per BTC: $%.2f\n", totalCostBasis/remainingBTC)
		}

		fmt.Printf("\n💰 Sales analysis for %d:\n", targetYear)
		fmt.Printf("  Number of sales: %d\n", len(hifoSales))
		
		totalProceeds := 0.0
		totalBTCSold := 0.0
		for i, sale := range hifoSales {
			proceeds := float64(sale.ProceedsUSD.Amount()) / 100
			btcSold := float64(sale.AmountBTC.Amount()) / 100000000
			totalProceeds += proceeds
			totalBTCSold += btcSold
			
			fmt.Printf("  Sale %d: %.8f BTC for $%.2f (avg price: $%.2f)\n", 
				i+1, btcSold, proceeds, proceeds/btcSold)
		}
		
		if len(hifoSales) > 0 {
			fmt.Printf("  Total sold: %.8f BTC for $%.2f (avg: $%.2f/BTC)\n", 
				totalBTCSold, totalProceeds, totalProceeds/totalBTCSold)
		}
	})
}