package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Rhymond/go-money"
)

const foldDateTimeLayout = "2006-01-02 15:04:05.999999-07:00"

type Fold struct {
	ReferenceID     string   `csv:"Reference ID"`
	Date            foldDate `csv:"Date (UTC)"`
	TransactionType string   `csv:"Transaction Type"`
	Description     string   `csv:"Description"`
	Asset           string   `csv:"Asset"`
	AmountBTC       string   `csv:"Amount (BTC)"`
	PricePerCoin    string   `csv:"Price per Coin (USD)"`
	SubtotalUSD     string   `csv:"Subtotal (USD)"`
	FeeUSD          string   `csv:"Fee (USD)"`
	TotalUSD        string   `csv:"Total (USD)"`
	TransactionID   string   `csv:"Transaction ID"`
}

type foldDate struct {
	time.Time
}

func (d *foldDate) UnmarshalCSV(data []byte) (err error) {
	d.Time, err = time.Parse(foldDateTimeLayout, string(data))
	return err
}

func (d *foldDate) MarshalCSV() ([]byte, error) {
	return []byte(d.Format(foldDateTimeLayout)), nil
}

func (d *foldDate) String() string {
	return d.Format(foldDateTimeLayout)
}

func (f Fold) ToTransaction() (Transaction, error) {
	amountBTC, err := newBTCFromString(f.AmountBTC)
	if err != nil {
		return Transaction{}, fmt.Errorf("invalid BTC amount: %s", f.AmountBTC)
	}

	pricePerCoin := money.New(0, money.USD)
	if f.PricePerCoin != "" {
		pricePerCoinFloat, err := strconv.ParseFloat(f.PricePerCoin, 64)
		if err != nil {
			return Transaction{}, fmt.Errorf("invalid price per coin: %s", f.PricePerCoin)
		}
		pricePerCoin = money.NewFromFloat(pricePerCoinFloat, money.USD)
	}

	subtotalUSD := money.New(0, money.USD)
	if f.SubtotalUSD != "" {
		subtotalUSDFloat, err := strconv.ParseFloat(f.SubtotalUSD, 64)
		if err != nil {
			return Transaction{}, fmt.Errorf("invalid subtotal USD: %s", f.SubtotalUSD)
		}
		subtotalUSD = money.NewFromFloat(subtotalUSDFloat, money.USD)
	}

	feeUSD := money.New(0, money.USD)
	if f.FeeUSD != "" {
		feeUSDFloat, err := strconv.ParseFloat(f.FeeUSD, 64)
		if err != nil {
			return Transaction{}, fmt.Errorf("invalid fee USD: %s", f.FeeUSD)
		}
		feeUSD = money.NewFromFloat(feeUSDFloat, money.USD)
	}

	totalUSD := money.New(0, money.USD)
	if f.TotalUSD != "" {
		totalUSDFloat, err := strconv.ParseFloat(f.TotalUSD, 64)
		if err != nil {
			return Transaction{}, fmt.Errorf("invalid total USD: %s", f.TotalUSD)
		}
		totalUSD = money.NewFromFloat(totalUSDFloat, money.USD)
	}

	return Transaction{
		ReferenceID:     f.ReferenceID,
		Date:            f.Date.Time,
		TransactionType: f.TransactionType,
		Description:     f.Description,
		Asset:           f.Asset,
		AmountBTC:       amountBTC,
		PricePerCoin:    pricePerCoin,
		SubtotalUSD:     subtotalUSD,
		FeeUSD:          feeUSD,
		TotalUSD:        totalUSD,
		TransactionID:   f.TransactionID,
	}, nil
}