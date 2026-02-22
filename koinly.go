package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Rhymond/go-money"
)

const koinlyDateTimeLayout = "01-02-2006 15:04:05 UTC"

type Koinly struct {
	Date koinlyDate `csv:"Date"` // the date in Koinly format (YYYY-MM-DD 00:00 UTC)
	// Note - these must be in UTC even if you live in another timezone
	SentAmount       string    `csv:"Sent Amount"`           // the number of tokens sent/withdrawn
	SentCurrency     string    `csv:"Sent Currency"`         // the token being sent/withdrawn
	ReceivedAmount   string    `csv:"Received Amount"`       // the number of tokens received/bought
	ReceivedCurrency string    `csv:"Received Currency"`     // the token being received/bought
	FeeAmount        float64   `csv:"Fee Amount"`            // the fee amount
	FeeCurrency      string    `csv:"Fee Currency"`          // the currency the fee was paid in
	NetWorthAmount   float64   `csv:"Net Worth Amount"`      // You can set these if you know what the market rate of the transaction was at the time of the transaction.
	NetWorthCurrency string    `csv:"Net Worth Currency"`    // the currency of the net worth amount. Note - [See the full description here](https://support.koinly.io/en/articles/9489976-how-to-create-a-custom-csv-file-with-your-data#h_1a6e1b0803)
	Label            KoinlyTag `csv:"Label,omitempty"`       // the tag, e.g. Cost, Lost, Gift. Note - you can find the [list of available tags here](https://support.koinly.io/en/articles/9490023-what-are-tags-labels)
	Description      string    `csv:"Description,omitempty"` // a description of the transaction. This is optional and has no effect on the import/calculations, but can be useful for record-keeping purposes
	TxHash           string    `csv:"TxHash,omitempty"`      // the transaction hash from the blockchain. This is optional.
}

type KoinlyTag string

// Tags can be added as appropriate. For regular deposits/withdrawals/trades, no tag is required.
const (
	// Koinly allows the following tags for outgoing transactions.
	KoinlyGift            = "gift"
	KoinlyLost            = "lost"
	KoinlyDonation        = "donation"
	KoinlyCost            = "cost"
	KoinlyLoanFee         = "loan fee"
	KoinlyMarginFee       = "margin fee"
	KoinlyLoanRepayment   = "loan repayment"
	KoinlyMarginRepayment = "margin repayment"
	KoinlyStakeOut        = "stake"
	KoinlyRealizedGainOut = "realized gain"

	// The following tags are allowed for incoming transactions.
	KoinlyAirdrop         = "airdrop"
	KoinlyFork            = "fork"
	KoinlyMining          = "mining"
	KoinlyReward          = "reward"
	KoinlyIncome          = "income"
	KoinlyLendingInterest = "lending interest"
	KoinlyCashback        = "cashback"
	KoinlySalary          = "salary"
	KoinlyFeeRefund       = "fee refund"
	KoinlyLoan            = "loan"
	KoinlyMarginLoan      = "margin loan"
	KoinlyStakeIn         = "stake"
	KoinlyRealizedGainIn  = "realized gain"
)

type koinlyDate struct {
	time.Time
}

func (d *koinlyDate) UnmarshalCSV(data []byte) (err error) {
	d.Time, err = time.Parse(koinlyDateTimeLayout, string(data))
	return err
}

func (d *koinlyDate) MarshalCSV() ([]byte, error) {
	return []byte(d.Time.UTC().Format(koinlyDateTimeLayout)), nil
}

func (d *koinlyDate) String() string {
	return d.Time.UTC().Format(koinlyDateTimeLayout)
}

func (k Koinly) ToTransaction() (Transaction, error) {
	sentAmount, _ := strconv.ParseFloat(k.SentAmount, 64)
	receivedAmount, _ := strconv.ParseFloat(k.ReceivedAmount, 64)

	var amountBTC *money.Money
	var transactionType string
	var subtotalUSD *money.Money

	if k.ReceivedCurrency == "BTC" {
		amountBTC, _ = newBTCFromString(fmt.Sprintf("%.8f", receivedAmount))
		transactionType = "Deposit"
		if k.SentCurrency == "USD" {
			subtotalUSD = money.NewFromFloat(sentAmount, money.USD)
		} else {
			subtotalUSD = money.New(0, money.USD)
		}
	} else if k.SentCurrency == "BTC" {
		amountBTC, _ = newBTCFromString(fmt.Sprintf("%.8f", -sentAmount))
		transactionType = "Withdrawal"
		if k.ReceivedCurrency == "USD" {
			subtotalUSD = money.NewFromFloat(receivedAmount, money.USD)
		} else {
			subtotalUSD = money.New(0, money.USD)
		}
	} else {
		return Transaction{}, fmt.Errorf("unsupported currency pair: %s -> %s", k.SentCurrency, k.ReceivedCurrency)
	}

	var feeUSD *money.Money
	if k.FeeCurrency == "USD" || k.FeeCurrency == "" {
		feeUSD = money.NewFromFloat(k.FeeAmount, money.USD)
	} else {
		feeUSD = money.NewFromFloat(k.FeeAmount, money.USD)
	}

	totalUSD, _ := subtotalUSD.Add(feeUSD)

	return Transaction{
		ReferenceID:     fmt.Sprintf("koinly_%d", k.Date.UnixNano()),
		Date:            k.Date.Time,
		TransactionType: transactionType,
		Description:     k.Description,
		Asset:           "BTC",
		AmountBTC:       amountBTC,
		PricePerCoin:    money.New(0, money.USD),
		SubtotalUSD:     subtotalUSD,
		FeeUSD:          feeUSD,
		TotalUSD:        totalUSD,
		TransactionID:   k.TxHash,
	}, nil
}
