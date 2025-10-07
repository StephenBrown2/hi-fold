package main

import (
	"fmt"
	"time"

	"github.com/Rhymond/go-money"
)

// TaxOutcome represents the tax calculation results for a set of sales
type TaxOutcome struct {
	TotalGainLoss  *money.Money
	ShortTermGains *money.Money
	LongTermGains  *money.Money
	ShortTermTax   *money.Money
	LongTermTax    *money.Money
	TaxLiability   *money.Money
}

// calculateTaxOutcome calculates tax liability using simplified tax rates
func calculateTaxOutcome(sales []Sale) TaxOutcome {
	var shortTermGains int64
	var longTermGains int64
	var totalGainLoss int64

	for _, sale := range sales {
		totalGainLoss += sale.GainLossUSD.Amount()

		for _, lot := range sale.Lots {
			// Calculate gain/loss for this lot
			proceedsForLot := sale.ProceedsUSD.Amount() * lot.AmountBTC.Amount() / sale.AmountBTC.Amount()
			gainLoss := proceedsForLot - lot.CostBasisUSD.Amount()

			if lot.IsLongTerm {
				longTermGains += gainLoss
			} else {
				shortTermGains += gainLoss
			}
		}
	}

	// Only pay tax on gains, not losses
	var shortTermTax int64
	var longTermTax int64

	if shortTermGains > 0 {
		shortTermTax = shortTermGains * 32 / 100 // 32% for short-term gains
	}
	if longTermGains > 0 {
		longTermTax = longTermGains * 15 / 100 // 15% for long-term gains
	}

	totalTax := shortTermTax + longTermTax

	return TaxOutcome{
		TotalGainLoss:  money.New(totalGainLoss, money.USD),
		ShortTermGains: money.New(shortTermGains, money.USD),
		LongTermGains:  money.New(longTermGains, money.USD),
		ShortTermTax:   money.New(shortTermTax, money.USD),
		LongTermTax:    money.New(longTermTax, money.USD),
		TaxLiability:   money.New(totalTax, money.USD),
	}
}

// calculateFIFO implements First-In-First-Out cost basis calculation
func calculateFIFO(transactions []Transaction, targetYear int) ([]Lot, []Sale) {
	var lots []Lot
	var sales []Sale

	// Sort transactions by date
	sortedTxs := make([]Transaction, len(transactions))
	copy(sortedTxs, transactions)
	for i := 0; i < len(sortedTxs); i++ {
		for j := i + 1; j < len(sortedTxs); j++ {
			if sortedTxs[j].Date.Before(sortedTxs[i].Date) {
				sortedTxs[i], sortedTxs[j] = sortedTxs[j], sortedTxs[i]
			}
		}
	}

	for _, tx := range sortedTxs {
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
				lots = append(lots, lot)
			}

		case "Sale":
			sale := processSaleFIFO(tx, &lots)
			if tx.Date.Year() == targetYear {
				sales = append(sales, sale)
			}

		case "Withdrawal":
			processWithdrawalFIFO(tx, &lots)
		}
	}

	return lots, sales
}

// processSaleFIFO processes a sale using FIFO (First-In-First-Out) method
func processSaleFIFO(tx Transaction, lots *[]Lot) Sale {
	amountBTC := tx.AmountBTC.Absolute()
	proceedsUSD := tx.TotalUSD
	saleDate := tx.Date

	var usedLots []LotSale
	var totalCostBasis int64
	remainingToSell := amountBTC.Amount()

	// Use lots in chronological order (FIFO)
	for i := range *lots {
		if remainingToSell <= 0 {
			break
		}

		lot := &(*lots)[i]
		if lot.Remaining.Amount() <= 0 {
			continue
		}

		amountFromThisLot := lot.Remaining.Amount()
		if amountFromThisLot > remainingToSell {
			amountFromThisLot = remainingToSell
		}

		costBasisForAmount := lot.CostBasisUSD.Amount() * amountFromThisLot / lot.AmountBTC.Amount()

		isLongTerm := saleDate.Sub(lot.Date) > 365*24*time.Hour

		usedLots = append(usedLots, LotSale{
			LotDate:      lot.Date,
			AmountBTC:    money.New(amountFromThisLot, "BTC"),
			CostBasisUSD: money.New(costBasisForAmount, money.USD),
			PricePerCoin: lot.PricePerCoin,
			IsLongTerm:   isLongTerm,
		})

		totalCostBasis += costBasisForAmount
		lot.Remaining = money.New(lot.Remaining.Amount()-amountFromThisLot, "BTC")
		remainingToSell -= amountFromThisLot
	}

	gainLossUSD := money.New(proceedsUSD.Amount()-totalCostBasis, money.USD)

	return Sale{
		Date:         saleDate,
		AmountBTC:    amountBTC,
		ProceedsUSD:  proceedsUSD,
		CostBasisUSD: money.New(totalCostBasis, money.USD),
		GainLossUSD:  gainLossUSD,
		Lots:         usedLots,
	}
}

// processWithdrawalFIFO processes a withdrawal using FIFO method
func processWithdrawalFIFO(tx Transaction, lots *[]Lot) {
	amountBTC := tx.AmountBTC.Absolute()
	remainingToWithdraw := amountBTC.Amount()

	for i := range *lots {
		if remainingToWithdraw <= 0 {
			break
		}

		lot := &(*lots)[i]
		if lot.Remaining.Amount() <= 0 {
			continue
		}

		amountFromThisLot := lot.Remaining.Amount()
		if amountFromThisLot > remainingToWithdraw {
			amountFromThisLot = remainingToWithdraw
		}

		lot.Remaining = money.New(lot.Remaining.Amount()-amountFromThisLot, "BTC")
		remainingToWithdraw -= amountFromThisLot
	}
}

// calculateLIFO implements Last-In-First-Out cost basis calculation
func calculateLIFO(transactions []Transaction, targetYear int) ([]Lot, []Sale) {
	var lots []Lot
	var sales []Sale

	// Sort transactions by date
	sortedTxs := make([]Transaction, len(transactions))
	copy(sortedTxs, transactions)
	for i := 0; i < len(sortedTxs); i++ {
		for j := i + 1; j < len(sortedTxs); j++ {
			if sortedTxs[j].Date.Before(sortedTxs[i].Date) {
				sortedTxs[i], sortedTxs[j] = sortedTxs[j], sortedTxs[i]
			}
		}
	}

	for _, tx := range sortedTxs {
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
				lots = append(lots, lot)
			}

		case "Sale":
			sale := processSaleLIFO(tx, &lots)
			if tx.Date.Year() == targetYear {
				sales = append(sales, sale)
			}

		case "Withdrawal":
			processWithdrawalLIFO(tx, &lots)
		}
	}

	return lots, sales
}

// processSaleLIFO processes a sale using LIFO (Last-In-First-Out) method
func processSaleLIFO(tx Transaction, lots *[]Lot) Sale {
	amountBTC := tx.AmountBTC.Absolute()
	proceedsUSD := tx.TotalUSD
	saleDate := tx.Date

	var usedLots []LotSale
	var totalCostBasis int64
	remainingToSell := amountBTC.Amount()

	// Use lots in reverse chronological order (LIFO)
	for i := len(*lots) - 1; i >= 0; i-- {
		if remainingToSell <= 0 {
			break
		}

		lot := &(*lots)[i]
		if lot.Remaining.Amount() <= 0 {
			continue
		}

		amountFromThisLot := lot.Remaining.Amount()
		if amountFromThisLot > remainingToSell {
			amountFromThisLot = remainingToSell
		}

		costBasisForAmount := lot.CostBasisUSD.Amount() * amountFromThisLot / lot.AmountBTC.Amount()

		isLongTerm := saleDate.Sub(lot.Date) > 365*24*time.Hour

		usedLots = append(usedLots, LotSale{
			LotDate:      lot.Date,
			AmountBTC:    money.New(amountFromThisLot, "BTC"),
			CostBasisUSD: money.New(costBasisForAmount, money.USD),
			PricePerCoin: lot.PricePerCoin,
			IsLongTerm:   isLongTerm,
		})

		totalCostBasis += costBasisForAmount
		lot.Remaining = money.New(lot.Remaining.Amount()-amountFromThisLot, "BTC")
		remainingToSell -= amountFromThisLot
	}

	gainLossUSD := money.New(proceedsUSD.Amount()-totalCostBasis, money.USD)

	return Sale{
		Date:         saleDate,
		AmountBTC:    amountBTC,
		ProceedsUSD:  proceedsUSD,
		CostBasisUSD: money.New(totalCostBasis, money.USD),
		GainLossUSD:  gainLossUSD,
		Lots:         usedLots,
	}
}

// processWithdrawalLIFO processes a withdrawal using LIFO method
func processWithdrawalLIFO(tx Transaction, lots *[]Lot) {
	amountBTC := tx.AmountBTC.Absolute()
	remainingToWithdraw := amountBTC.Amount()

	for i := len(*lots) - 1; i >= 0; i-- {
		if remainingToWithdraw <= 0 {
			break
		}

		lot := &(*lots)[i]
		if lot.Remaining.Amount() <= 0 {
			continue
		}

		amountFromThisLot := lot.Remaining.Amount()
		if amountFromThisLot > remainingToWithdraw {
			amountFromThisLot = remainingToWithdraw
		}

		lot.Remaining = money.New(lot.Remaining.Amount()-amountFromThisLot, "BTC")
		remainingToWithdraw -= amountFromThisLot
	}
}

// getLastWeekday returns the last weekday of the given month/year
func getLastWeekday(year int, month time.Month) time.Time {
	// Start with the last day of the month
	lastDay := time.Date(year, month+1, 0, 15, 0, 0, 0, time.UTC)

	// Move backwards to find the last weekday
	for lastDay.Weekday() == time.Saturday || lastDay.Weekday() == time.Sunday {
		lastDay = lastDay.AddDate(0, 0, -1)
	}

	return lastDay
}

// Helper function to create test transactions
func createTestTransaction(refID, txType string, date time.Time, amountBTC, priceUSD, totalUSD float64) Transaction {
	btcAmount, _ := newBTCFromString(formatFloat(amountBTC))
	return Transaction{
		ReferenceID:     refID,
		Date:            date,
		TransactionType: txType,
		Description:     txType + " transaction",
		Asset:           "BTC",
		AmountBTC:       btcAmount,
		PricePerCoin:    money.NewFromFloat(priceUSD, money.USD),
		SubtotalUSD:     money.NewFromFloat(totalUSD, money.USD),
		FeeUSD:          money.New(0, money.USD),
		TotalUSD:        money.NewFromFloat(totalUSD, money.USD),
		TransactionID:   "",
	}
}

// Helper to format float as string with appropriate precision
func formatFloat(f float64) string {
	if f < 0 {
		return fmt.Sprintf("%.8f", f)
	}
	return fmt.Sprintf("%.8f", f)
}

// formatBTCForDisplay formats BTC amount for readable display
func formatBTCForDisplay(btc *money.Money) string {
	satoshis := btc.Amount()
	btcFloat := float64(satoshis) / 100000000.0
	return fmt.Sprintf("%.8f", btcFloat)
}
