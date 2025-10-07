package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/Rhymond/go-money"
)

func filterTransactionsByYear(transactions []Transaction, year int) []Transaction {
	var filtered []Transaction
	for _, tx := range transactions {
		if tx.Date.Year() == year {
			filtered = append(filtered, tx)
		}
	}
	return filtered
}

func calculateHIFO(transactions []Transaction, priceAPI PriceAPI, targetYear int) ([]Lot, []Sale) {
	var lots []Lot
	var sales []Sale

	// Sort transactions by date
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Date.Before(transactions[j].Date)
	})

	for _, tx := range transactions {
		switch tx.TransactionType {
		case "Purchase", "Deposit":
			// Only add lots that were acquired by the target year or earlier
			if tx.Date.Year() <= targetYear {
				lot := Lot{
					Date:         tx.Date,
					AmountBTC:    tx.AmountBTC,
					CostBasisUSD: tx.TotalUSD.Absolute(), // Use absolute value to ensure positive cost basis
					PricePerCoin: tx.PricePerCoin,
					Remaining:    tx.AmountBTC,
				}

				// Skip price fetching if we already have a price
				if !tx.PricePerCoin.IsZero() {
					lots = append(lots, lot)
					continue
				}

				// Fetch historical price for transactions without price
				price, err := priceAPI.GetBTCPriceUSD(tx.Date)
				if err != nil {
					// Handle API failure case
					fmt.Printf("Warning: Could not fetch price for %s, using zero cost basis: %v\n",
						tx.Date.Format("2006-01-02"), err)
					lot.CostBasisUSD = money.New(0, money.USD)
					lot.PricePerCoin = money.New(0, money.USD)
					lots = append(lots, lot)
					continue
				}

				// Handle API success case - set the fetched price
				lot.PricePerCoin = money.NewFromFloat(price, money.USD)

				// Calculate cost basis from market price if needed
				if tx.TotalUSD.IsZero() {
					// For deposits or purchases without price data, use the market price as cost basis
					// Get BTC amount as int64 to avoid float precision issues
					btcAmount := tx.AmountBTC.Amount()
					// Calculate cost basis: price * BTC amount (converting units appropriately)
					costBasisCents := int64(price*100) * btcAmount / 100000000 // Convert from satoshis to USD cents
					lot.CostBasisUSD = money.New(costBasisCents, money.USD)
				}

				fmt.Printf("Fetched historical price for %s on %s: $%.2f\n",
					tx.TransactionType, tx.Date.Local().Format(time.RFC1123), price)

				lots = append(lots, lot)
			}

		case "Sale":
			// Process all sales to correctly track remaining holdings
			sale := processSale(tx, &lots)
			// Only include sales from the target year in the returned sales slice for display
			if tx.Date.Year() == targetYear {
				sales = append(sales, sale)
			}

		case "Withdrawal":
			// Process withdrawals to reduce remaining holdings (but don't create taxable sale records)
			processWithdrawal(tx, &lots)
		}
	}

	return lots, sales
}

func processSale(tx Transaction, lots *[]Lot) Sale {
	amountBTC := tx.AmountBTC.Absolute()
	proceedsUSD := tx.TotalUSD
	saleDate := tx.Date

	sale := Sale{
		Date:         saleDate,
		AmountBTC:    amountBTC,
		ProceedsUSD:  proceedsUSD,
		CostBasisUSD: money.New(0, money.USD),
	}

	remaining := amountBTC

	// Sort lots for optimal tax outcome: Long-term HIFO first, then Short-term HIFO
	sortedIndices := make([]int, len(*lots))
	for i := range sortedIndices {
		sortedIndices[i] = i
	}

	sort.Slice(sortedIndices, func(i, j int) bool {
		lotI := (*lots)[sortedIndices[i]]
		lotJ := (*lots)[sortedIndices[j]]

		if lotI.Remaining.IsZero() {
			return false
		}
		if lotJ.Remaining.IsZero() {
			return true
		}

		// Calculate holding periods
		isLongTermI := saleDate.Sub(lotI.Date) > 365*24*time.Hour
		isLongTermJ := saleDate.Sub(lotJ.Date) > 365*24*time.Hour

		// Use transaction's PricePerCoin if available, otherwise calculate from proceeds
		var salePricePerCoin int64
		if !tx.PricePerCoin.IsZero() {
			salePricePerCoin = tx.PricePerCoin.Amount()
		} else {
			// Fallback: calculate from proceeds
			// proceedsUSD.Amount() is in cents, amountBTC.Amount() is in satoshis
			// We need cents per BTC, so: (cents * 100000000 satoshis/BTC) / satoshis
			salePricePerCoin = (proceedsUSD.Amount() * 100000000) / amountBTC.Amount()
		}

		// Determine gain/loss for each lot
		costPerCoinI := lotI.PricePerCoin.Amount()
		costPerCoinJ := lotJ.PricePerCoin.Amount()

		isLossI := salePricePerCoin < costPerCoinI
		isLossJ := salePricePerCoin < costPerCoinJ

		// Calculate priority scores (lower score = higher priority)
		// Priority order for tax optimization:
		// Long-term loss(0) > Short-term loss(1) > Short-term gain(2) > Long-term gain(3)
		// Rationale: Losses offset gains. For gains, minimize high-tax short-term first, defer low-tax long-term.
		getPriorityScore := func(isLongTerm, isLoss bool) int {
			if isLongTerm && isLoss {
				return 0 // Long-term loss - highest priority (offset gains + favorable tax treatment)
			} else if !isLongTerm && isLoss {
				return 1 // Short-term loss - second priority (offset gains)
			} else if !isLongTerm && !isLoss {
				return 2 // Short-term gain - third priority (high tax rate, minimize first)
			} else {
				return 3 // Long-term gain - lowest priority (low tax rate, defer when possible)
			}
		}

		priorityI := getPriorityScore(isLongTermI, isLossI)
		priorityJ := getPriorityScore(isLongTermJ, isLossJ)

		// First, sort by priority
		if priorityI != priorityJ {
			return priorityI < priorityJ
		}

		// Within the same priority category, optimize the selection:
		// For losses: prefer higher diff (bigger loss for tax purposes)
		// For gains: prefer lower diff (smaller gain for tax purposes)
		if isLossI == isLossJ {
			diffPerCoinI := salePricePerCoin - costPerCoinI
			diffPerCoinJ := salePricePerCoin - costPerCoinJ
			if isLossI { // Both are losses
				return diffPerCoinI < diffPerCoinJ // Bigger loss first
			} else { // Both are gains
				return diffPerCoinJ > diffPerCoinI // Smaller gain first
			}
		}

		return false
	})

	for _, idx := range sortedIndices {
		if remaining.IsZero() {
			break
		}

		lot := &(*lots)[idx]
		if lot.Remaining.IsZero() {
			continue
		}

		// Only consider lots that were purchased before or at the same time as the sale
		if lot.Date.After(saleDate) {
			continue
		}

		// Determine how much to sell from this lot
		sellAmount := remaining
		isRemainingGreater, _ := remaining.GreaterThan(lot.Remaining)
		if isRemainingGreater {
			sellAmount = lot.Remaining
		}

		// Calculate cost basis for this portion
		// Note: go-money doesn't have division, so we need float64 for: (cost/amount) * sellAmount
		costBasisFloat := float64(lot.CostBasisUSD.Amount())
		amountFloat := float64(lot.AmountBTC.Amount())
		sellAmountFloat := float64(sellAmount.Amount())
		costBasisForPortionFloat := (costBasisFloat / amountFloat) * sellAmountFloat
		costBasisForPortion := money.New(int64(costBasisForPortionFloat), money.USD)

		// Calculate holding period
		isLongTerm := saleDate.Sub(lot.Date) > 365*24*time.Hour

		lotSale := LotSale{
			LotDate:      lot.Date,
			AmountBTC:    sellAmount,
			CostBasisUSD: costBasisForPortion,
			PricePerCoin: money.New(int64(costBasisFloat/amountFloat), money.USD),
			IsLongTerm:   isLongTerm,
		}

		sale.Lots = append(sale.Lots, lotSale)
		sale.CostBasisUSD, _ = sale.CostBasisUSD.Add(costBasisForPortion)

		// Update lot
		lot.Remaining, _ = lot.Remaining.Subtract(sellAmount)
		remaining, _ = remaining.Subtract(sellAmount)
	}

	sale.GainLossUSD, _ = sale.ProceedsUSD.Subtract(sale.CostBasisUSD)
	return sale
}

func processWithdrawal(tx Transaction, lots *[]Lot) {
	amountBTC := tx.AmountBTC.Absolute()
	withdrawalDate := tx.Date
	remaining := amountBTC

	// Sort lots by cost basis per coin (highest first for HIFO)
	sortedIndices := make([]int, len(*lots))
	for i := range sortedIndices {
		sortedIndices[i] = i
	}

	sort.Slice(sortedIndices, func(i, j int) bool {
		lotI := (*lots)[sortedIndices[i]]
		lotJ := (*lots)[sortedIndices[j]]

		if lotI.Remaining.IsZero() {
			return false
		}
		if lotJ.Remaining.IsZero() {
			return true
		}

		// Highest price first
		gt, err := lotI.PricePerCoin.GreaterThan(lotJ.PricePerCoin)
		if err != nil {
			return false
		}
		return gt
	})

	for _, idx := range sortedIndices {
		if remaining.IsZero() {
			break
		}

		lot := &(*lots)[idx]
		if lot.Remaining.IsZero() {
			continue
		}

		// Only consider lots that were purchased before or at the same time as the withdrawal
		if lot.Date.After(withdrawalDate) {
			continue
		}

		// Determine how much to withdraw from this lot
		withdrawAmount := remaining
		isRemainingGreater, _ := remaining.GreaterThan(lot.Remaining)
		if isRemainingGreater {
			withdrawAmount = lot.Remaining
		}

		// Update lot (reduce remaining quantity)
		lot.Remaining, _ = lot.Remaining.Subtract(withdrawAmount)
		remaining, _ = remaining.Subtract(withdrawAmount)
	}
}

// YearResult holds the lots and sales for a specific year
type YearResult struct {
	Year  int
	Lots  []Lot
	Sales []Sale
}

// calculateHIFOWithCache calculates HIFO for a single year using cache when available
func calculateHIFOWithCache(transactions []Transaction, priceAPI PriceAPI, targetYear int, cache *Cache, inputFiles []string) ([]Lot, []Sale) {
	// Try to load cached state for the previous year
	var startingLots []Lot

	if targetYear > 1 {
		if cachedState, err := cache.loadYearEndState(targetYear-1, inputFiles); err == nil {
			fmt.Printf("Using cached lot state from end of %d\n", targetYear-1)
			startingLots = cachedState.Lots
		} else {
			fmt.Printf("Cache miss for year %d, calculating from scratch: %v\n", targetYear-1, err)
			// Calculate from beginning up to previous year
			startingLots, prevYearSales := calculateHIFO(transactions, priceAPI, targetYear-1)
			// Cache the previous year's ending state
			if err := cache.saveYearEndState(targetYear-1, startingLots, prevYearSales, inputFiles); err != nil {
				fmt.Printf("Warning: Failed to cache year-end state for %d: %v\n", targetYear-1, err)
			}
		}
	}

	// Process only the target year's transactions starting from cached lots
	yearTransactions := filterTransactionsByYear(transactions, targetYear)
	lots, sales := calculateHIFOFromState(yearTransactions, priceAPI, targetYear, startingLots)

	// Cache this year's ending state
	if err := cache.saveYearEndState(targetYear, lots, sales, inputFiles); err != nil {
		fmt.Printf("Warning: Failed to cache year-end state for %d: %v\n", targetYear, err)
	}

	return lots, sales
}

// calculateAllYearsWithCache processes all years with sales using cache
func calculateAllYearsWithCache(transactions []Transaction, priceAPI PriceAPI, cache *Cache, inputFiles []string) map[int]YearResult {
	results := make(map[int]YearResult)

	// Find all years with sales
	salesYears := make(map[int]bool)
	for _, tx := range transactions {
		if tx.TransactionType == "Sale" {
			salesYears[tx.Date.Year()] = true
		}
	}

	// Sort years for sequential processing
	var years []int
	for year := range salesYears {
		years = append(years, year)
	}
	sort.Ints(years)

	var currentLots []Lot

	for _, year := range years {
		// Try to load cached state
		if cachedState, err := cache.loadYearEndState(year, inputFiles); err == nil {
			fmt.Printf("Using cached results for year %d\n", year)
			results[year] = YearResult{
				Year:  year,
				Lots:  cachedState.Lots,
				Sales: cachedState.Sales,
			}
			// For cached results, we still need to update currentLots to the ending state
			// This ensures the next year starts with the correct lot state
			currentLots = cachedState.Lots
		} else {
			fmt.Printf("Calculating year %d...\n", year)
			// Calculate this year starting from current lot state
			yearTransactions := filterTransactionsByYear(transactions, year)
			lots, sales := calculateHIFOFromState(yearTransactions, priceAPI, year, currentLots)

			results[year] = YearResult{
				Year:  year,
				Lots:  lots,
				Sales: sales,
			}
			currentLots = lots

			// Cache this year's ending state
			if err := cache.saveYearEndState(year, lots, sales, inputFiles); err != nil {
				fmt.Printf("Warning: Failed to cache year-end state for %d: %v\n", year, err)
			}
		}
	}

	return results
}

// calculateHIFOFromState calculates HIFO starting from a given lot state
func calculateHIFOFromState(transactions []Transaction, priceAPI PriceAPI, targetYear int, startingLots []Lot) ([]Lot, []Sale) {
	lots := make([]Lot, len(startingLots))
	copy(lots, startingLots)
	var sales []Sale

	// Sort transactions by date
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Date.Before(transactions[j].Date)
	})

	for _, tx := range transactions {
		// Only process transactions from target year and earlier
		if tx.Date.Year() > targetYear {
			continue
		}

		switch tx.TransactionType {
		case "Purchase", "Deposit":
			if tx.Date.Year() <= targetYear {
				lot := Lot{
					Date:         tx.Date,
					AmountBTC:    tx.AmountBTC,
					CostBasisUSD: tx.TotalUSD.Absolute(),
					PricePerCoin: tx.PricePerCoin,
					Remaining:    tx.AmountBTC,
				}

				// Handle price fetching similar to original calculateHIFO
				if !tx.PricePerCoin.IsZero() {
					lots = append(lots, lot)
					continue
				}

				price, err := priceAPI.GetBTCPriceUSD(tx.Date)
				if err != nil {
					fmt.Printf("Warning: Could not fetch price for %s, using zero cost basis: %v\n",
						tx.Date.Format("2006-01-02"), err)
					lot.CostBasisUSD = money.New(0, money.USD)
					lot.PricePerCoin = money.New(0, money.USD)
					lots = append(lots, lot)
					continue
				}

				lot.PricePerCoin = money.NewFromFloat(price, money.USD)
				if tx.TotalUSD.IsZero() {
					btcAmount := tx.AmountBTC.Amount()
					costBasisCents := int64(price*100) * btcAmount / 100000000
					lot.CostBasisUSD = money.New(costBasisCents, money.USD)
				}

				fmt.Printf("Fetched historical price for %s on %s: $%.2f\n",
					tx.TransactionType, tx.Date.Local().Format(time.RFC1123), price)

				lots = append(lots, lot)
			}

		case "Sale":
			sale := processSale(tx, &lots)
			if tx.Date.Year() == targetYear {
				sales = append(sales, sale)
			}

		case "Withdrawal":
			processWithdrawal(tx, &lots)
		}
	}

	return lots, sales
}
