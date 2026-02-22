package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Rhymond/go-money"
)

var errSkipTransaction = fmt.Errorf("skip transaction")

// Strike represents a transaction from Strike Bitcoin exchange
type Strike struct {
	TransactionID   string     `csv:"Transaction ID"`
	TimeUTC         strikeDate `csv:"Time (UTC)"`
	Status          string     `csv:"Status"`
	TransactionType string     `csv:"Transaction Type"`
	AmountUSD       string     `csv:"Amount USD"`
	FeeUSD          string     `csv:"Fee USD"`
	AmountBTC       string     `csv:"Amount BTC"`
	FeeBTC          string     `csv:"Fee BTC"`
	Description     string     `csv:"Description"`
	ExchangeRate    string     `csv:"Exchange Rate"`
	TransactionHash string     `csv:"Transaction Hash"`
}

// strikeDate is a custom type for parsing Strike's date format
type strikeDate struct {
	time.Time
}

// UnmarshalCSV parses Strike's date format: "Jan 15 2025 06:37:33"
func (d *strikeDate) UnmarshalCSV(data []byte) (err error) {
	d.Time, err = time.Parse("Jan 02 2006 15:04:05", string(data))
	return err
}

func (d *strikeDate) MarshalCSV() ([]byte, error) {
	return []byte(d.Format("Jan 02 2006 15:04:05")), nil
}

func (d *strikeDate) String() string {
	return d.Format("Jan 02 2006 15:04:05")
}

func (s Strike) ToTransaction() (Transaction, error) {
	if s.Status == "Reversed" {
		return Transaction{}, errSkipTransaction
	}

	amountBTC, err := newBTCFromString(s.AmountBTC)
	if err != nil {
		amountBTC = money.New(0, "BTC")
	}

	amountUSD, _ := strconv.ParseFloat(s.AmountUSD, 64)
	feeUSD, _ := strconv.ParseFloat(s.FeeUSD, 64)
	exchangeRate, _ := strconv.ParseFloat(s.ExchangeRate, 64)

	var transactionType string
	var subtotalUSD *money.Money

	switch s.TransactionType {
	case "Deposit":
		transactionType = "Deposit"
		subtotalUSD = money.NewFromFloat(amountUSD, money.USD)
	case "Purchase":
		transactionType = "Deposit"
		subtotalUSD = money.NewFromFloat(amountUSD, money.USD)
	case "Send", "Withdrawal":
		transactionType = "Withdrawal"
		subtotalUSD = money.NewFromFloat(amountUSD, money.USD)
	case "Receive":
		transactionType = "Deposit"
		subtotalUSD = money.New(0, money.USD)
	default:
		transactionType = "Other"
		subtotalUSD = money.NewFromFloat(amountUSD, money.USD)
	}

	feeUSDMoney := money.NewFromFloat(feeUSD, money.USD)
	totalUSD, _ := subtotalUSD.Add(feeUSDMoney)

	return Transaction{
		ReferenceID:     s.TransactionID,
		Date:            s.TimeUTC.Time,
		TransactionType: transactionType,
		Description:     s.Description,
		Asset:           "BTC",
		AmountBTC:       amountBTC,
		PricePerCoin:    money.NewFromFloat(exchangeRate, money.USD),
		SubtotalUSD:     subtotalUSD,
		FeeUSD:          feeUSDMoney,
		TotalUSD:        totalUSD,
		TransactionID:   s.TransactionHash,
	}, nil
}
