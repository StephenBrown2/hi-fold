package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Rhymond/go-money"
)

// River represents a transaction from River Bitcoin exchange
type River struct {
	Date             riverDate `csv:"Date"`
	SentAmount       string    `csv:"Sent Amount"`
	SentCurrency     string    `csv:"Sent Currency"`
	ReceivedAmount   string    `csv:"Received Amount"`
	ReceivedCurrency string    `csv:"Received Currency"`
	FeeAmount        string    `csv:"Fee Amount"`
	FeeCurrency      string    `csv:"Fee Currency"`
	Tag              string    `csv:"Tag"`
}

type riverDate struct {
	time.Time
}

func (d *riverDate) UnmarshalCSV(data []byte) (err error) {
	d.Time, err = time.Parse(time.DateTime, string(data))
	return err
}

func (d *riverDate) MarshalCSV() ([]byte, error) {
	return []byte(d.Format(time.DateTime)), nil
}

func (d *riverDate) String() string {
	return d.Format(time.DateTime)
}

func (r River) ToTransaction() (Transaction, error) {
	sentAmount, err := strconv.ParseFloat(r.SentAmount, 64)
	if err != nil {
		sentAmount = 0
	}
	receivedAmount, err := strconv.ParseFloat(r.ReceivedAmount, 64)
	if err != nil {
		receivedAmount = 0
	}
	feeAmount, err := strconv.ParseFloat(r.FeeAmount, 64)
	if err != nil {
		feeAmount = 0
	}

	var amountBTC *money.Money
	var transactionType string
	var subtotalUSD *money.Money

	if r.ReceivedCurrency == "BTC" {
		amountBTC, _ = newBTCFromString(fmt.Sprintf("%.8f", receivedAmount))
		transactionType = "Deposit"
		subtotalUSD = money.NewFromFloat(sentAmount, money.USD)
	} else if r.SentCurrency == "BTC" {
		amountBTC, _ = newBTCFromString(fmt.Sprintf("%.8f", -sentAmount))
		if r.ReceivedCurrency == "USD" {
			transactionType = "Sale"
		} else {
			transactionType = "Withdrawal"
		}
		subtotalUSD = money.NewFromFloat(receivedAmount, money.USD)
	} else {
		return Transaction{}, fmt.Errorf("unsupported currency pair: %s -> %s", r.SentCurrency, r.ReceivedCurrency)
	}

	var feeUSD *money.Money
	if r.FeeCurrency == "USD" || r.FeeCurrency == "" {
		feeUSD = money.NewFromFloat(feeAmount, money.USD)
	} else {
		feeUSD = money.NewFromFloat(feeAmount, money.USD)
	}

	totalUSD, _ := subtotalUSD.Add(feeUSD)

	return Transaction{
		ReferenceID:     fmt.Sprintf("river_%d", r.Date.UnixNano()),
		Date:            r.Date.Time,
		TransactionType: transactionType,
		Description:     r.Tag,
		Asset:           "BTC",
		AmountBTC:       amountBTC,
		PricePerCoin:    money.New(0, money.USD),
		SubtotalUSD:     subtotalUSD,
		FeeUSD:          feeUSD,
		TotalUSD:        totalUSD,
		TransactionID:   "",
	}, nil
}
