package main

import (
	"strconv"
	"time"

	"github.com/Rhymond/go-money"
)

// Transaction represents a single transaction from the Fold CSV
type Transaction struct {
	ReferenceID     string
	Date            time.Time
	TransactionType string
	Description     string
	Asset           string
	AmountBTC       *money.Money
	PricePerCoin    *money.Money
	SubtotalUSD     *money.Money
	FeeUSD          *money.Money
	TotalUSD        *money.Money
	TransactionID   string
}

// Lot represents a tax lot for HIFO calculation
type Lot struct {
	Date         time.Time
	AmountBTC    *money.Money
	CostBasisUSD *money.Money
	PricePerCoin *money.Money
	Remaining    *money.Money
}

// Sale represents a sale transaction with cost basis calculation
type Sale struct {
	Date         time.Time
	AmountBTC    *money.Money
	ProceedsUSD  *money.Money
	CostBasisUSD *money.Money
	GainLossUSD  *money.Money
	Lots         []LotSale
}

// LotSale represents the portion of a sale matched to a specific lot
type LotSale struct {
	LotDate      time.Time
	AmountBTC    *money.Money
	CostBasisUSD *money.Money
	PricePerCoin *money.Money
	IsLongTerm   bool // True if held for more than 1 year
}

// TaxRecord represents a record for IRS Form 8949
type TaxRecord struct {
	DateAcquired string
	DateSold     string
	Description  string
	Proceeds     *money.Money
	CostBasis    *money.Money
	GainLoss     *money.Money
}

// Helper function to create precise BTC Money objects from string
func newBTCFromString(btcAmountStr string) (*money.Money, error) {
	// Use go-money's NewFromFloat but with decimal package for more precision
	// Alternative: directly parse string to avoid any float precision issues
	btcAmount, err := strconv.ParseFloat(btcAmountStr, 64)
	if err != nil {
		return nil, err
	}

	// Round to 8 decimal places (satoshi precision) to avoid floating point errors
	const satoshiPrecision = 100_000_000
	satoshis := int64(btcAmount*satoshiPrecision + 0.5)
	if btcAmount < 0 {
		satoshis = int64(btcAmount*satoshiPrecision - 0.5)
	}
	return money.New(satoshis, "BTC"), nil
}
