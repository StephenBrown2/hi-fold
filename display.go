package main

import (
	"fmt"

	"github.com/Rhymond/go-money"
)

// createSaleFromLots creates a Sale record from a subset of lots, calculating proportional proceeds
func createSaleFromLots(originalSale Sale, lots []LotSale) Sale {
	sale := Sale{
		Date: originalSale.Date,
		Lots: lots,
	}

	// Calculate totals for this subset of lots
	sale.AmountBTC = money.New(0, "BTC")
	sale.CostBasisUSD = money.New(0, money.USD)

	for _, lot := range lots {
		sale.AmountBTC, _ = sale.AmountBTC.Add(lot.AmountBTC)
		sale.CostBasisUSD, _ = sale.CostBasisUSD.Add(lot.CostBasisUSD)
	}

	// Calculate proportional proceeds
	if !originalSale.AmountBTC.IsZero() {
		proceedsFloat := float64(originalSale.ProceedsUSD.Amount()) / 100
		saleAmountFloat := float64(originalSale.AmountBTC.Amount()) / 100000000
		subsetAmountFloat := float64(sale.AmountBTC.Amount()) / 100000000
		subsetProceedsFloat := (proceedsFloat / saleAmountFloat) * subsetAmountFloat
		sale.ProceedsUSD = money.NewFromFloat(subsetProceedsFloat, money.USD)
	} else {
		sale.ProceedsUSD = money.New(0, money.USD)
	}

	sale.GainLossUSD, _ = sale.ProceedsUSD.Subtract(sale.CostBasisUSD)
	return sale
}

func displayResults(lots []Lot, sales []Sale, transactions []Transaction, year int, priceAPI PriceAPI) {
	fmt.Println(titleStyle.Render(fmt.Sprintf("Bitcoin HIFO Cost Basis Report - %d", year)))

	// Separate sales into short-term and long-term
	var shortTermSales []Sale
	var longTermSales []Sale

	for _, sale := range sales {
		// Create separate sales for short-term and long-term portions
		var shortTermLots []LotSale
		var longTermLots []LotSale

		for _, lot := range sale.Lots {
			if lot.IsLongTerm {
				longTermLots = append(longTermLots, lot)
			} else {
				shortTermLots = append(shortTermLots, lot)
			}
		}

		// Create separate sale records if there are both types
		if len(shortTermLots) > 0 {
			shortTermSale := createSaleFromLots(sale, shortTermLots)
			shortTermSales = append(shortTermSales, shortTermSale)
		}

		if len(longTermLots) > 0 {
			longTermSale := createSaleFromLots(sale, longTermLots)
			longTermSales = append(longTermSales, longTermSale)
		}
	}

	// Overall summary table
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

	// Short-Term Sales Detail Table
	if len(shortTermSales) > 0 {
		displaySalesTable("Short-Term Sales Details (≤1 Year Holding Period)", shortTermSales)
	}

	// Long-Term Sales Detail Table
	if len(longTermSales) > 0 {
		displaySalesTable("Long-Term Sales Details (>1 Year Holding Period)", longTermSales)
	}

	displayHoldings(lots, priceAPI)
}

// displaySalesTable shows a table of sales with summary totals
func displaySalesTable(title string, sales []Sale) {
	if len(sales) == 0 {
		return
	}

	salesTable := newTable().
		StyleFunc(monetaryTableStyleFunc()).
		Headers("Date Sold", "Amount (BTC)", "Proceeds ($)", "Cost Basis ($)", "Price/BTC", "Gain/Loss ($)")

	totalProceeds := money.New(0, money.USD)
	totalCostBasis := money.New(0, money.USD)
	totalGainLoss := money.New(0, money.USD)

	for _, sale := range sales {
		// Calculate average price per BTC: proceeds ÷ amount
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

		totalProceeds, _ = totalProceeds.Add(sale.ProceedsUSD)
		totalCostBasis, _ = totalCostBasis.Add(sale.CostBasisUSD)
		totalGainLoss, _ = totalGainLoss.Add(sale.GainLossUSD)
	}

	// Add totals row
	salesTable.Row(
		"TOTALS:",
		"",
		totalProceeds.Display(),
		totalCostBasis.Display(),
		"",
		displayRedGreen(totalGainLoss),
	)

	fmt.Println(titleStyle.Render(title))
	fmt.Println(salesTable.Render())
	fmt.Println()
}

// displayHoldings shows the remaining BTC holdings
func displayHoldings(lots []Lot, priceAPI PriceAPI) {
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
