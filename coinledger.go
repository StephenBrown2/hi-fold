package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Rhymond/go-money"
)

const coinLedgerDateTimeLayout = "01-02-2006 15:04:05"

type CoinLedgerTag string

const (
	CoinLedgerAirdrop         CoinLedgerTag = "Airdrop"
	CoinLedgerCasualtyLoss    CoinLedgerTag = "Casualty Loss"
	CoinLedgerDeposit         CoinLedgerTag = "Deposit"
	CoinLedgerGiftReceived    CoinLedgerTag = "Gift Received"
	CoinLedgerGiftSent        CoinLedgerTag = "Gift Sent"
	CoinLedgerHardFork        CoinLedgerTag = "Hard Fork"
	CoinLedgerIncome          CoinLedgerTag = "Income"
	CoinLedgerInterest        CoinLedgerTag = "Interest"
	CoinLedgerInterestPayment CoinLedgerTag = "Interest Payment"
	CoinLedgerInvestmentLoss  CoinLedgerTag = "Investment Loss"
	CoinLedgerMerchantPayment CoinLedgerTag = "Merchant Payment"
	CoinLedgerMining          CoinLedgerTag = "Mining"
	CoinLedgerStaking         CoinLedgerTag = "Staking"
	CoinLedgerTheftLoss       CoinLedgerTag = "Theft Loss"
	CoinLedgerTrade           CoinLedgerTag = "Trade"
	CoinLedgerWithdrawal      CoinLedgerTag = "Withdrawal"
)

type CoinLedger struct {
	DateUTC        coinLedgerDate `csv:"Date (UTC)"`
	Platform       string         `csv:"Platform (Optional),omitempty"`
	AssetSent      string         `csv:"Asset Sent"`
	AmountSent     string         `csv:"Amount Sent"` // Note: this is a string to ensure proper formatting
	AssetReceived  string         `csv:"Asset Received"`
	AmountReceived string         `csv:"Amount Received"` // Note: this is a string to ensure proper formatting
	FeeCurrency    string         `csv:"Fee Currency (Optional),omitempty"`
	FeeAmount      float64        `csv:"Fee Amount (Optional),omitempty"`
	Type           CoinLedgerTag  `csv:"Type"`
	Description    string         `csv:"Description (Optional),omitempty"`
	TxHash         string         `csv:"TxHash (Optional),omitempty"`
}

type coinLedgerDate struct {
	time.Time
}

func (d *coinLedgerDate) UnmarshalCSV(data []byte) (err error) {
	d.Time, err = time.Parse(coinLedgerDateTimeLayout, string(data))
	return err
}

func (d *coinLedgerDate) MarshalCSV() ([]byte, error) {
	return []byte(d.Format(coinLedgerDateTimeLayout)), nil
}

func (d *coinLedgerDate) String() string {
	return d.Format(coinLedgerDateTimeLayout)
}

func (c CoinLedger) ToTransaction() (Transaction, error) {
	amountSent, _ := strconv.ParseFloat(c.AmountSent, 64)
	amountReceived, _ := strconv.ParseFloat(c.AmountReceived, 64)

	var amountBTC *money.Money
	var transactionType string
	var subtotalUSD *money.Money

	if c.AssetReceived == "BTC" {
		amountBTC, _ = newBTCFromString(fmt.Sprintf("%.8f", amountReceived))
		transactionType = "Deposit"
		if c.AssetSent == "USD" {
			subtotalUSD = money.NewFromFloat(amountSent, money.USD)
		} else {
			subtotalUSD = money.New(0, money.USD)
		}
	} else if c.AssetSent == "BTC" {
		amountBTC, _ = newBTCFromString(fmt.Sprintf("%.8f", -amountSent))
		transactionType = "Withdrawal"
		if c.AssetReceived == "USD" {
			subtotalUSD = money.NewFromFloat(amountReceived, money.USD)
		} else {
			subtotalUSD = money.New(0, money.USD)
		}
	} else {
		return Transaction{}, fmt.Errorf("unsupported currency pair: %s -> %s", c.AssetSent, c.AssetReceived)
	}

	var feeUSD *money.Money
	if c.FeeCurrency == "USD" || c.FeeCurrency == "" {
		feeUSD = money.NewFromFloat(c.FeeAmount, money.USD)
	} else {
		feeUSD = money.NewFromFloat(c.FeeAmount, money.USD)
	}

	totalUSD, _ := subtotalUSD.Add(feeUSD)

	return Transaction{
		ReferenceID:     fmt.Sprintf("coinledger_%d", c.DateUTC.UnixNano()),
		Date:            c.DateUTC.Time,
		TransactionType: transactionType,
		Description:     fmt.Sprintf("%s: %s", c.Type, c.Description),
		Asset:           "BTC",
		AmountBTC:       amountBTC,
		PricePerCoin:    money.New(0, money.USD),
		SubtotalUSD:     subtotalUSD,
		FeeUSD:          feeUSD,
		TotalUSD:        totalUSD,
		TransactionID:   c.TxHash,
	}, nil
}
