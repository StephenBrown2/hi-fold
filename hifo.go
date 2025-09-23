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
		if tx.Date.Year() <= year {
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
			sale := processSale(tx.AmountBTC.Absolute(), tx.TotalUSD, tx.Date, &lots)
			// Only include sales from the target year in the returned sales slice for display
			if tx.Date.Year() == targetYear {
				sales = append(sales, sale)
			}

		case "Withdrawal":
			// Process withdrawals to reduce remaining holdings (but don't create taxable sale records)
			processWithdrawal(tx.AmountBTC.Absolute(), tx.Date, &lots)
		}
	}

	return lots, sales
}

func processSale(amountBTC, proceedsUSD *money.Money, saleDate time.Time, lots *[]Lot) Sale {
	sale := Sale{
		Date:         saleDate,
		AmountBTC:    amountBTC,
		ProceedsUSD:  proceedsUSD,
		CostBasisUSD: money.New(0, money.USD),
	}

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

		// Calculate price per coin for HIFO (highest cost first)
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

		lotSale := LotSale{
			LotDate:      lot.Date,
			AmountBTC:    sellAmount,
			CostBasisUSD: costBasisForPortion,
			PricePerCoin: money.New(int64(costBasisFloat/amountFloat), money.USD),
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

func processWithdrawal(amountBTC *money.Money, withdrawalDate time.Time, lots *[]Lot) {
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
