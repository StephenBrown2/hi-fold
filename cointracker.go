package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Rhymond/go-money"
)

const coinTrackerDateTimeLayout = "01/02/2006 15:04:05"

type CoinTracker struct {
	Date             coinTrackerDate `csv:"Date"`              // Date of transaction
	ReceivedQuantity string          `csv:"Received Quantity"` // Amount of crypto or cash received
	ReceivedCurrency string          `csv:"Received Currency"` // Type of crypto received
	SentQuantity     string          `csv:"Sent Quantity"`     // Amount of crypto or cash sent
	SentCurrency     string          `csv:"Sent Currency"`     // Type of crypto sent
	FeeAmount        float64         `csv:"Fee Amount"`        // Transaction fee amount in the currency it was paid
	FeeCurrency      string          `csv:"Fee Currency"`      // Type of currency that your transaction fee was paid in
	Tag              CoinTrackerTag  `csv:"Tag"`               // [CoinTracker CSV tags](https://support.cointracker.io/hc/en-us/articles/4413049710225): Use tags to categorize send/receive transactions by type for better tracking and tax purposes.* Do not use tags for trades or transfers.
}

type coinTrackerDate struct {
	time.Time
}

func (d *coinTrackerDate) UnmarshalCSV(data []byte) (err error) {
	d.Time, err = time.Parse(coinTrackerDateTimeLayout, string(data))
	return err
}

func (d *coinTrackerDate) MarshalCSV() ([]byte, error) {
	return []byte(d.Time.UTC().Format(coinTrackerDateTimeLayout)), nil
}

func (d *coinTrackerDate) String() string {
	return d.Time.UTC().Format(coinTrackerDateTimeLayout)
}

func (c CoinTracker) ToTransaction() (Transaction, error) {
	receivedAmount, _ := strconv.ParseFloat(c.ReceivedQuantity, 64)
	sentAmount, _ := strconv.ParseFloat(c.SentQuantity, 64)

	var amountBTC *money.Money
	var transactionType string
	var subtotalUSD *money.Money

	if c.ReceivedCurrency == "BTC" {
		amountBTC, _ = newBTCFromString(fmt.Sprintf("%.8f", receivedAmount))
		transactionType = "Deposit"
		if c.SentCurrency == "USD" {
			subtotalUSD = money.NewFromFloat(sentAmount, money.USD)
		} else {
			subtotalUSD = money.New(0, money.USD)
		}
	} else if c.SentCurrency == "BTC" {
		amountBTC, _ = newBTCFromString(fmt.Sprintf("%.8f", -sentAmount))
		transactionType = "Withdrawal"
		if c.ReceivedCurrency == "USD" {
			subtotalUSD = money.NewFromFloat(receivedAmount, money.USD)
		} else {
			subtotalUSD = money.New(0, money.USD)
		}
	} else {
		return Transaction{}, fmt.Errorf("unsupported currency pair: %s -> %s", c.SentCurrency, c.ReceivedCurrency)
	}

	var feeUSD *money.Money
	if c.FeeCurrency == "USD" || c.FeeCurrency == "" {
		feeUSD = money.NewFromFloat(c.FeeAmount, money.USD)
	} else {
		feeUSD = money.NewFromFloat(c.FeeAmount, money.USD)
	}

	totalUSD, _ := subtotalUSD.Add(feeUSD)

	return Transaction{
		ReferenceID:     fmt.Sprintf("cointracker_%d", c.Date.UnixNano()),
		Date:            c.Date.Time,
		TransactionType: transactionType,
		Description:     string(c.Tag),
		Asset:           "BTC",
		AmountBTC:       amountBTC,
		PricePerCoin:    money.New(0, money.USD),
		SubtotalUSD:     subtotalUSD,
		FeeUSD:          feeUSD,
		TotalUSD:        totalUSD,
		TransactionID:   "",
	}, nil
}

type CoinTrackerTag string

// Transaction categories label cryptocurrency transactions for tax and reporting purposes. You can manually adjust the category of automatically synced transactions as needed.
