package main

import (
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/matryer/is"
)

func TestNewBTCFromString(t *testing.T) {
	is := is.New(t)

	t.Run("valid BTC amounts", func(t *testing.T) {
		testCases := []struct {
			input    string
			expected int64 // satoshis
		}{
			{"1.00000000", 100000000},               // 1 BTC
			{"0.50000000", 50000000},                // 0.5 BTC
			{"0.12345678", 12345678},                // Fractional BTC
			{"0.00000001", 1},                       // 1 satoshi
			{"21000000.00000000", 2100000000000000}, // Max BTC supply
			{"0", 0},                                // Zero
			{"1", 100000000},                        // No decimals
			{"0.1", 10000000},                       // Single decimal
		}

		for _, tc := range testCases {
			btc, err := newBTCFromString(tc.input)
			is.NoErr(err)
			is.Equal(btc.Amount(), tc.expected)
			is.Equal(btc.Currency().Code, "BTC")
		}
	})

	t.Run("negative BTC amounts", func(t *testing.T) {
		btc, err := newBTCFromString("-0.50000000")
		is.NoErr(err)
		is.Equal(btc.Amount(), int64(-50000000)) // -0.5 BTC in satoshis
	})

	t.Run("invalid BTC amounts", func(t *testing.T) {
		invalidInputs := []string{
			"not-a-number",
			"1.2.3",
			"abc",
			"",
			// Note: "1e10" scientific notation is valid for strconv.ParseFloat
		}

		for _, input := range invalidInputs {
			_, err := newBTCFromString(input)
			is.True(err != nil)
		}
	})

	t.Run("precision edge cases", func(t *testing.T) {
		// Test precision beyond 8 decimal places (should round)
		btc, err := newBTCFromString("1.123456789")
		is.NoErr(err)
		// Should round to 8 decimal places
		is.True(btc.Amount() > 112345678 && btc.Amount() <= 112345679)
	})
}

func TestTransactionStruct(t *testing.T) {
	is := is.New(t)

	t.Run("transaction creation and field access", func(t *testing.T) {
		date := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		amountBTC := money.New(100000000, "BTC") // 1 BTC
		pricePerCoin := money.NewFromFloat(40000.0, money.USD)
		totalUSD := money.NewFromFloat(40000.0, money.USD)

		tx := Transaction{
			ReferenceID:     "test-ref-123",
			Date:            date,
			TransactionType: "Purchase",
			Description:     "Test Purchase",
			Asset:           "BTC",
			AmountBTC:       amountBTC,
			PricePerCoin:    pricePerCoin,
			SubtotalUSD:     totalUSD,
			FeeUSD:          money.New(0, money.USD),
			TotalUSD:        totalUSD,
			TransactionID:   "blockchain-tx-id",
		}

		is.Equal(tx.ReferenceID, "test-ref-123")
		is.Equal(tx.Date, date)
		is.Equal(tx.TransactionType, "Purchase")
		is.Equal(tx.Description, "Test Purchase")
		is.Equal(tx.Asset, "BTC")
		is.Equal(tx.AmountBTC.Amount(), int64(100000000))
		is.Equal(tx.PricePerCoin.Amount(), int64(4000000)) // $40000 in cents
		is.Equal(tx.TotalUSD.Amount(), int64(4000000))
		is.Equal(tx.TransactionID, "blockchain-tx-id")
	})
}

func TestLotStruct(t *testing.T) {
	is := is.New(t)

	t.Run("lot creation and calculations", func(t *testing.T) {
		date := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		amountBTC := money.New(100000000, "BTC") // 1 BTC
		costBasis := money.NewFromFloat(40000.0, money.USD)
		pricePerCoin := money.NewFromFloat(40000.0, money.USD)

		lot := Lot{
			Date:         date,
			AmountBTC:    amountBTC,
			CostBasisUSD: costBasis,
			PricePerCoin: pricePerCoin,
			Remaining:    amountBTC,
		}

		is.Equal(lot.Date, date)
		is.Equal(lot.AmountBTC.Amount(), int64(100000000))
		is.Equal(lot.CostBasisUSD.Amount(), int64(4000000))
		is.Equal(lot.PricePerCoin.Amount(), int64(4000000))
		is.Equal(lot.Remaining.Amount(), int64(100000000))
	})

	t.Run("lot with partial remaining", func(t *testing.T) {
		amountBTC := money.New(100000000, "BTC") // 1 BTC original
		remaining := money.New(50000000, "BTC")  // 0.5 BTC remaining

		lot := Lot{
			Date:         time.Now(),
			AmountBTC:    amountBTC,
			CostBasisUSD: money.NewFromFloat(40000.0, money.USD),
			PricePerCoin: money.NewFromFloat(40000.0, money.USD),
			Remaining:    remaining,
		}

		is.Equal(lot.AmountBTC.Amount(), int64(100000000))
		is.Equal(lot.Remaining.Amount(), int64(50000000))

		// Verify remaining is less than original
		isGreater, _ := lot.AmountBTC.GreaterThan(lot.Remaining)
		is.True(isGreater)
	})
}

func TestSaleStruct(t *testing.T) {
	is := is.New(t)

	t.Run("sale with single lot", func(t *testing.T) {
		saleDate := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)
		lotDate := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)

		amountBTC := money.New(50000000, "BTC") // 0.5 BTC
		proceeds := money.NewFromFloat(30000.0, money.USD)
		costBasis := money.NewFromFloat(20000.0, money.USD)

		lotSale := LotSale{
			LotDate:      lotDate,
			AmountBTC:    amountBTC,
			CostBasisUSD: costBasis,
			PricePerCoin: money.NewFromFloat(40000.0, money.USD),
			IsLongTerm:   true, // More than 1 year
		}

		sale := Sale{
			Date:         saleDate,
			AmountBTC:    amountBTC,
			ProceedsUSD:  proceeds,
			CostBasisUSD: costBasis,
			Lots:         []LotSale{lotSale},
		}

		// Calculate gain/loss
		gainLoss, _ := sale.ProceedsUSD.Subtract(sale.CostBasisUSD)
		sale.GainLossUSD = gainLoss

		is.Equal(sale.Date, saleDate)
		is.Equal(sale.AmountBTC.Amount(), int64(50000000))
		is.Equal(sale.ProceedsUSD.Amount(), int64(3000000))  // $30k in cents
		is.Equal(sale.CostBasisUSD.Amount(), int64(2000000)) // $20k in cents
		is.Equal(sale.GainLossUSD.Amount(), int64(1000000))  // $10k gain in cents
		is.Equal(len(sale.Lots), 1)
		is.True(sale.Lots[0].IsLongTerm)
	})

	t.Run("sale with multiple lots (mixed term)", func(t *testing.T) {
		saleDate := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

		// Long-term lot (> 1 year old)
		longTermLot := LotSale{
			LotDate:      time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC),
			AmountBTC:    money.New(25000000, "BTC"),            // 0.25 BTC
			CostBasisUSD: money.NewFromFloat(7500.0, money.USD), // $7.5k
			PricePerCoin: money.NewFromFloat(30000.0, money.USD),
			IsLongTerm:   true,
		}

		// Short-term lot (< 1 year old)
		shortTermLot := LotSale{
			LotDate:      time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC),
			AmountBTC:    money.New(25000000, "BTC"),             // 0.25 BTC
			CostBasisUSD: money.NewFromFloat(12500.0, money.USD), // $12.5k
			PricePerCoin: money.NewFromFloat(50000.0, money.USD),
			IsLongTerm:   false,
		}

		totalAmount := money.New(50000000, "BTC")                // 0.5 BTC total
		totalCostBasis := money.NewFromFloat(20000.0, money.USD) // $20k total
		totalProceeds := money.NewFromFloat(30000.0, money.USD)  // $30k total

		sale := Sale{
			Date:         saleDate,
			AmountBTC:    totalAmount,
			ProceedsUSD:  totalProceeds,
			CostBasisUSD: totalCostBasis,
			Lots:         []LotSale{longTermLot, shortTermLot},
		}

		gainLoss, _ := sale.ProceedsUSD.Subtract(sale.CostBasisUSD)
		sale.GainLossUSD = gainLoss

		is.Equal(len(sale.Lots), 2)
		is.True(sale.Lots[0].IsLongTerm)
		is.True(!sale.Lots[1].IsLongTerm)
		is.Equal(sale.GainLossUSD.Amount(), int64(1000000)) // $10k gain
	})
}

func TestLotSaleStruct(t *testing.T) {
	is := is.New(t)

	t.Run("long-term lot sale", func(t *testing.T) {
		lotDate := time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC)

		lotSale := LotSale{
			LotDate:      lotDate,
			AmountBTC:    money.New(100000000, "BTC"), // 1 BTC
			CostBasisUSD: money.NewFromFloat(30000.0, money.USD),
			PricePerCoin: money.NewFromFloat(30000.0, money.USD),
			IsLongTerm:   true,
		}

		is.Equal(lotSale.LotDate, lotDate)
		is.Equal(lotSale.AmountBTC.Amount(), int64(100000000))
		is.Equal(lotSale.CostBasisUSD.Amount(), int64(3000000))
		is.Equal(lotSale.PricePerCoin.Amount(), int64(3000000))
		is.True(lotSale.IsLongTerm)
	})

	t.Run("short-term lot sale", func(t *testing.T) {
		lotSale := LotSale{
			LotDate:      time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC),
			AmountBTC:    money.New(50000000, "BTC"), // 0.5 BTC
			CostBasisUSD: money.NewFromFloat(25000.0, money.USD),
			PricePerCoin: money.NewFromFloat(50000.0, money.USD),
			IsLongTerm:   false,
		}

		is.True(!lotSale.IsLongTerm)
		is.Equal(lotSale.AmountBTC.Amount(), int64(50000000))
		is.Equal(lotSale.CostBasisUSD.Amount(), int64(2500000))
	})
}

func TestTaxRecordStruct(t *testing.T) {
	is := is.New(t)

	t.Run("tax record creation", func(t *testing.T) {
		record := TaxRecord{
			DateAcquired: "01/01/2024",
			DateSold:     "06/01/2024",
			Description:  "1.0 BTC",
			Proceeds:     money.NewFromFloat(60000.0, money.USD),
			CostBasis:    money.NewFromFloat(40000.0, money.USD),
			GainLoss:     money.NewFromFloat(20000.0, money.USD),
		}

		is.Equal(record.DateAcquired, "01/01/2024")
		is.Equal(record.DateSold, "06/01/2024")
		is.Equal(record.Description, "1.0 BTC")
		is.Equal(record.Proceeds.Amount(), int64(6000000))  // $60k in cents
		is.Equal(record.CostBasis.Amount(), int64(4000000)) // $40k in cents
		is.Equal(record.GainLoss.Amount(), int64(2000000))  // $20k in cents
	})
}

func TestMoneyPrecisionAndRounding(t *testing.T) {
	is := is.New(t)

	t.Run("BTC satoshi precision", func(t *testing.T) {
		// Test that we can represent the smallest BTC unit (1 satoshi)
		btc, err := newBTCFromString("0.00000001")
		is.NoErr(err)
		is.Equal(btc.Amount(), int64(1))

		// Test maximum precision
		btc, err = newBTCFromString("0.12345678")
		is.NoErr(err)
		is.Equal(btc.Amount(), int64(12345678))
	})

	t.Run("USD cent precision", func(t *testing.T) {
		// Test USD amounts with 2-decimal precision
		usd := money.NewFromFloat(1234.56, money.USD)
		is.Equal(usd.Amount(), int64(123456)) // $1234.56 in cents

		// Test very small USD amounts
		usd = money.NewFromFloat(0.01, money.USD)
		is.Equal(usd.Amount(), int64(1)) // 1 cent
	})

	t.Run("large amounts within int64 range", func(t *testing.T) {
		// Test maximum BTC supply (21 million BTC)
		maxBTC, err := newBTCFromString("21000000.00000000")
		is.NoErr(err)
		is.Equal(maxBTC.Amount(), int64(2100000000000000))

		// Test large USD amounts (millions of dollars)
		largeUSD := money.NewFromFloat(1000000.00, money.USD)
		is.Equal(largeUSD.Amount(), int64(100000000)) // $1M in cents
	})
}

// Test money operations that are critical for HIFO calculations
func TestMoneyOperations(t *testing.T) {
	is := is.New(t)

	t.Run("BTC addition and subtraction", func(t *testing.T) {
		btc1 := money.New(100000000, "BTC") // 1 BTC
		btc2 := money.New(50000000, "BTC")  // 0.5 BTC

		sum, err := btc1.Add(btc2)
		is.NoErr(err)
		is.Equal(sum.Amount(), int64(150000000)) // 1.5 BTC

		diff, err := btc1.Subtract(btc2)
		is.NoErr(err)
		is.Equal(diff.Amount(), int64(50000000)) // 0.5 BTC
	})

	t.Run("USD addition and subtraction", func(t *testing.T) {
		usd1 := money.NewFromFloat(1000.00, money.USD)
		usd2 := money.NewFromFloat(500.00, money.USD)

		sum, err := usd1.Add(usd2)
		is.NoErr(err)
		is.Equal(sum.Amount(), int64(150000)) // $1500.00 in cents

		diff, err := usd1.Subtract(usd2)
		is.NoErr(err)
		is.Equal(diff.Amount(), int64(50000)) // $500.00 in cents
	})

	t.Run("money comparisons", func(t *testing.T) {
		btc1 := money.New(100000000, "BTC") // 1 BTC
		btc2 := money.New(50000000, "BTC")  // 0.5 BTC

		isGreater, err := btc1.GreaterThan(btc2)
		is.NoErr(err)
		is.True(isGreater)

		isEqual, err := btc1.Equals(btc1)
		is.NoErr(err)
		is.True(isEqual)

		isZero := money.New(0, "BTC").IsZero()
		is.True(isZero)
	})
}

// Testify enhancement opportunities:
/*
The following functions could benefit from testify/suite:
- TestMoneyOperations: Could use suite for setting up common money amounts
- TestLotStruct/TestSaleStruct: Could share common test data setup

The following could benefit from testify/assert for better error messages:
- Money amount comparisons: assert.Equal would show better diffs for large numbers
- Time-based assertions: assert.WithinDuration for date comparisons
- Complex struct assertions: assert.Equal for better struct field comparison output

The following could benefit from testify/require to fail fast:
- newBTCFromString validation: require.NoError to stop test on money creation failure
- Struct field validation: require.NotNil before accessing nested fields

Custom assertion helpers that could be useful:
- AssertBTCAmount(t, got, expectedSatoshis): Compare BTC amounts with better error messages
- AssertUSDAmount(t, got, expectedCents): Compare USD amounts with dollar formatting
- AssertMoneyEqual(t, got, expected): Currency-aware money comparison
*/
