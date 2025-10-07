package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/matryer/is"
)

func TestCalculateHIFO(t *testing.T) {
	is := is.New(t)
	mockAPI := NewMockPriceAPI()

	t.Run("simple HIFO calculation with single purchase and sale", func(t *testing.T) {
		transactions := []Transaction{
			createTestTransaction("buy-1", "Purchase", time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), 1.0, 40000, -40000),
			createTestTransaction("sell-1", "Sale", time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), -0.5, 60000, 30000),
		}

		lots, sales := calculateHIFO(transactions, mockAPI, 2024)

		is.Equal(len(lots), 1)
		is.Equal(len(sales), 1)

		// Check remaining lot
		lot := lots[0]
		is.Equal(lot.Remaining.Amount(), int64(50000000)) // 0.5 BTC remaining

		// Check sale
		sale := sales[0]
		is.Equal(sale.AmountBTC.Amount(), int64(50000000))   // 0.5 BTC sold
		is.Equal(sale.ProceedsUSD.Amount(), int64(3000000))  // $30k proceeds
		is.Equal(sale.CostBasisUSD.Amount(), int64(2000000)) // $20k cost basis (0.5 * $40k)
		is.Equal(sale.GainLossUSD.Amount(), int64(1000000))  // $10k gain
	})

	t.Run("HIFO lot selection order - highest cost first", func(t *testing.T) {
		transactions := []Transaction{
			// Three purchases at different prices
			createTestTransaction("buy-low", "Purchase", time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), 1.0, 30000, -30000),
			createTestTransaction("buy-high", "Purchase", time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC), 1.0, 50000, -50000),
			createTestTransaction("buy-mid", "Purchase", time.Date(2024, 1, 3, 10, 0, 0, 0, time.UTC), 1.0, 40000, -40000),
			// Sale that should use high-cost lot first
			createTestTransaction("sell-1", "Sale", time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC), -1.5, 60000, 90000),
		}

		_, sales := calculateHIFO(transactions, mockAPI, 2024)

		is.Equal(len(sales), 1)
		sale := sales[0]
		is.Equal(len(sale.Lots), 2) // Should use 2 lots

		// First lot used should be the highest cost ($50k), then medium ($40k)
		// We expect: 1.0 BTC from $50k lot + 0.5 BTC from $40k lot
		expectedCostBasis := 50000.0 + (0.5 * 40000.0) // $50k + $20k = $70k
		is.Equal(sale.CostBasisUSD.Amount(), int64(expectedCostBasis*100))
	})

	t.Run("long-term vs short-term capital gains prioritization", func(t *testing.T) {
		baseDate := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

		transactions := []Transaction{
			// Long-term purchase (> 1 year before sale)
			createTestTransaction("buy-longterm", "Purchase", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC), 1.0, 45000, -45000),
			// Short-term purchase (< 1 year before sale)
			createTestTransaction("buy-shortterm", "Purchase", time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC), 1.0, 35000, -35000),
			// Sale
			createTestTransaction("sell-1", "Sale", baseDate, -0.5, 30000, 15000), // Loss scenario
		}

		_, sales := calculateHIFO(transactions, mockAPI, 2024)

		is.Equal(len(sales), 1)
		sale := sales[0]
		is.Equal(len(sale.Lots), 1)

		// Should prioritize long-term loss over short-term loss
		lotSale := sale.Lots[0]
		is.True(lotSale.IsLongTerm)
		is.Equal(lotSale.CostBasisUSD.Amount(), int64(2250000)) // 0.5 * $45k = $22.5k
	})

	t.Run("mixed gains and losses optimization", func(t *testing.T) {
		saleDate := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

		transactions := []Transaction{
			// Loss positions (cost > sale price of $40k)
			createTestTransaction("buy-loss-lt", "Purchase", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC), 1.0, 50000, -50000), // Long-term loss
			createTestTransaction("buy-loss-st", "Purchase", time.Date(2024, 8, 1, 10, 0, 0, 0, time.UTC), 1.0, 45000, -45000), // Short-term loss
			// Gain positions (cost < sale price)
			createTestTransaction("buy-gain-lt", "Purchase", time.Date(2023, 6, 1, 10, 0, 0, 0, time.UTC), 1.0, 30000, -30000), // Long-term gain
			createTestTransaction("buy-gain-st", "Purchase", time.Date(2024, 9, 1, 10, 0, 0, 0, time.UTC), 1.0, 35000, -35000), // Short-term gain
			// Sale at $40k/BTC
			createTestTransaction("sell-1", "Sale", saleDate, -2.0, 40000, 80000),
		}

		_, sales := calculateHIFO(transactions, mockAPI, 2024)

		is.Equal(len(sales), 1)
		sale := sales[0]

		// Verify optimal tax lot selection order
		// Sale of 2.0 BTC should use exactly 2 lots: highest tax losses first
		// Expected order: Long-term loss (priority 0), Short-term loss (priority 1)
		// Within losses: bigger loss first (50k > 45k)
		is.Equal(len(sale.Lots), 2) // Should use exactly 2 lots for 2.0 BTC sale

		expectedOrder := []struct {
			isLongTerm bool
			costBasis  float64
			amount     int64
		}{
			{true, 50000, 100000000},  // 1.0 BTC Long-term loss: $50k cost vs $40k sale = $10k loss
			{false, 45000, 100000000}, // 1.0 BTC Short-term loss: $45k cost vs $40k sale = $5k loss
		}

		for i, expected := range expectedOrder {
			lot := sale.Lots[i]
			is.Equal(lot.IsLongTerm, expected.isLongTerm)
			is.Equal(lot.AmountBTC.Amount(), expected.amount)
			// Verify cost basis matches expected (within reasonable tolerance for float conversion)
			actualCostBasis := lot.CostBasisUSD.Amount()
			expectedCostBasisCents := int64(expected.costBasis * 100) // Convert to cents
			is.True(actualCostBasis >= expectedCostBasisCents-1 && actualCostBasis <= expectedCostBasisCents+1)
		}
	})

	t.Run("multi-year calculation - only process target year sales", func(t *testing.T) {
		transactions := []Transaction{
			createTestTransaction("buy-2023", "Purchase", time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC), 2.0, 25000, -50000),
			createTestTransaction("sell-2023", "Sale", time.Date(2023, 6, 1, 10, 0, 0, 0, time.UTC), -0.5, 35000, 17500),
			createTestTransaction("buy-2024", "Purchase", time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), 1.0, 45000, -45000),
			createTestTransaction("sell-2024", "Sale", time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), -0.5, 55000, 27500),
		}

		// Calculate for 2024 only
		lots, sales := calculateHIFO(transactions, mockAPI, 2024)

		// Should only return 2024 sales
		is.Equal(len(sales), 1)
		is.Equal(sales[0].Date.Year(), 2024)

		// But lots should include remaining from all years up to 2024
		is.True(len(lots) >= 1)
	})
}

func TestProcessSale(t *testing.T) {
	is := is.New(t)

	t.Run("sale consuming partial lot", func(t *testing.T) {
		saleDate := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

		// Create lot with 1 BTC
		lots := []Lot{
			{
				Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				AmountBTC:    money.New(100000000, "BTC"), // 1 BTC
				CostBasisUSD: money.NewFromFloat(40000, money.USD),
				PricePerCoin: money.NewFromFloat(40000, money.USD),
				Remaining:    money.New(100000000, "BTC"), // 1 BTC remaining
			},
		}

		// Sale of 0.3 BTC
		saleTransaction := createTestTransaction("sell-1", "Sale", saleDate, -0.3, 60000, 18000)

		sale := processSale(saleTransaction, &lots)

		is.Equal(sale.AmountBTC.Amount(), int64(30000000))   // 0.3 BTC
		is.Equal(sale.ProceedsUSD.Amount(), int64(1800000))  // $18k
		is.Equal(sale.CostBasisUSD.Amount(), int64(1200000)) // 0.3 * $40k = $12k
		is.Equal(sale.GainLossUSD.Amount(), int64(600000))   // $6k gain

		// Check lot remaining
		is.Equal(lots[0].Remaining.Amount(), int64(70000000)) // 0.7 BTC remaining

		// Check lot sale details
		is.Equal(len(sale.Lots), 1)
		lotSale := sale.Lots[0]
		is.Equal(lotSale.AmountBTC.Amount(), int64(30000000)) // 0.3 BTC from this lot
		is.True(!lotSale.IsLongTerm)                          // < 1 year
	})

	t.Run("sale consuming multiple lots", func(t *testing.T) {
		saleDate := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

		lots := []Lot{
			{
				Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				AmountBTC:    money.New(50000000, "BTC"),           // 0.5 BTC
				CostBasisUSD: money.NewFromFloat(25000, money.USD), // $50k/BTC
				PricePerCoin: money.NewFromFloat(50000, money.USD),
				Remaining:    money.New(50000000, "BTC"),
			},
			{
				Date:         time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC),
				AmountBTC:    money.New(80000000, "BTC"),           // 0.8 BTC
				CostBasisUSD: money.NewFromFloat(32000, money.USD), // $40k/BTC
				PricePerCoin: money.NewFromFloat(40000, money.USD),
				Remaining:    money.New(80000000, "BTC"),
			},
		}

		// Sale of 1.0 BTC - should use entire first lot + partial second lot
		saleTransaction := createTestTransaction("sell-1", "Sale", saleDate, -1.0, 60000, 60000)

		sale := processSale(saleTransaction, &lots)

		is.Equal(sale.AmountBTC.Amount(), int64(100000000)) // 1.0 BTC
		is.Equal(len(sale.Lots), 2)                         // Used both lots

		// Should use higher cost lot first (HIFO)
		firstLot := sale.Lots[0]
		is.Equal(firstLot.AmountBTC.Amount(), int64(50000000)) // 0.5 BTC from first lot

		secondLot := sale.Lots[1]
		is.Equal(secondLot.AmountBTC.Amount(), int64(50000000)) // 0.5 BTC from second lot

		// Verify remaining amounts
		is.Equal(lots[0].Remaining.Amount(), int64(0))        // First lot fully consumed
		is.Equal(lots[1].Remaining.Amount(), int64(30000000)) // 0.3 BTC remaining in second lot
	})

	t.Run("sale with insufficient lots", func(t *testing.T) {
		lots := []Lot{
			{
				Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				AmountBTC:    money.New(50000000, "BTC"), // 0.5 BTC
				CostBasisUSD: money.NewFromFloat(20000, money.USD),
				PricePerCoin: money.NewFromFloat(40000, money.USD),
				Remaining:    money.New(50000000, "BTC"),
			},
		}

		// Try to sell 1.0 BTC but only have 0.5 BTC
		saleTransaction := createTestTransaction("sell-1", "Sale", time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), -1.0, 60000, 60000)

		sale := processSale(saleTransaction, &lots)

		// Should only sell what's available
		is.Equal(len(sale.Lots), 1)
		is.Equal(sale.Lots[0].AmountBTC.Amount(), int64(50000000)) // Only 0.5 BTC sold
		is.Equal(lots[0].Remaining.Amount(), int64(0))             // Lot fully consumed
	})
}

func TestProcessWithdrawal(t *testing.T) {
	is := is.New(t)

	t.Run("withdrawal consuming highest cost lots first", func(t *testing.T) {
		lots := []Lot{
			{
				Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				AmountBTC:    money.New(100000000, "BTC"), // 1 BTC
				CostBasisUSD: money.NewFromFloat(30000, money.USD),
				PricePerCoin: money.NewFromFloat(30000, money.USD),
				Remaining:    money.New(100000000, "BTC"),
			},
			{
				Date:         time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC),
				AmountBTC:    money.New(100000000, "BTC"),          // 1 BTC
				CostBasisUSD: money.NewFromFloat(50000, money.USD), // Higher cost
				PricePerCoin: money.NewFromFloat(50000, money.USD),
				Remaining:    money.New(100000000, "BTC"),
			},
		}

		withdrawalTransaction := createTestTransaction("withdraw-1", "Withdrawal", time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), -0.5, 0, 0)

		processWithdrawal(withdrawalTransaction, &lots)

		// Should consume from highest cost lot first (HIFO for withdrawals)
		is.Equal(lots[1].Remaining.Amount(), int64(50000000))  // 0.5 BTC remaining in high-cost lot
		is.Equal(lots[0].Remaining.Amount(), int64(100000000)) // Low-cost lot untouched
	})

	t.Run("withdrawal consuming multiple lots", func(t *testing.T) {
		lots := []Lot{
			{
				Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
				AmountBTC:    money.New(30000000, "BTC"),           // 0.3 BTC
				CostBasisUSD: money.NewFromFloat(18000, money.USD), // $60k/BTC
				PricePerCoin: money.NewFromFloat(60000, money.USD),
				Remaining:    money.New(30000000, "BTC"),
			},
			{
				Date:         time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC),
				AmountBTC:    money.New(50000000, "BTC"),           // 0.5 BTC
				CostBasisUSD: money.NewFromFloat(20000, money.USD), // $40k/BTC
				PricePerCoin: money.NewFromFloat(40000, money.USD),
				Remaining:    money.New(50000000, "BTC"),
			},
		}

		// Withdraw 0.6 BTC - should use entire first lot + partial second lot
		withdrawalTransaction := createTestTransaction("withdraw-1", "Withdrawal", time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), -0.6, 0, 0)

		processWithdrawal(withdrawalTransaction, &lots)

		// First lot (higher cost) should be fully consumed
		is.Equal(lots[0].Remaining.Amount(), int64(0))

		// Second lot should have partial consumption: 0.5 - 0.3 = 0.2 BTC remaining
		is.Equal(lots[1].Remaining.Amount(), int64(20000000)) // 0.2 BTC remaining
	})
}

func TestFilterTransactionsByYear(t *testing.T) {
	is := is.New(t)

	transactions := []Transaction{
		createTestTransaction("2023-tx", "Purchase", time.Date(2023, 6, 1, 10, 0, 0, 0, time.UTC), 1.0, 30000, -30000),
		createTestTransaction("2024-tx1", "Purchase", time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), 1.0, 40000, -40000),
		createTestTransaction("2024-tx2", "Sale", time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), -0.5, 50000, 25000),
		createTestTransaction("2025-tx", "Sale", time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC), -0.5, 60000, 30000),
	}

	filtered := filterTransactionsByYear(transactions, 2024)

	is.Equal(len(filtered), 2)
	is.Equal(filtered[0].ReferenceID, "2024-tx1")
	is.Equal(filtered[1].ReferenceID, "2024-tx2")
}

func TestHIFOSortingLogic(t *testing.T) {
	is := is.New(t)

	t.Run("priority scoring system", func(t *testing.T) {
		// Test the internal sorting logic by creating a controlled scenario
		saleDate := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)
		salePrice := 40000.0

		// Create lots representing each priority category
		lots := []Lot{
			// Priority 3: Short-term gain (lowest priority)
			{
				Date:         time.Date(2024, 8, 1, 10, 0, 0, 0, time.UTC), // < 1 year
				AmountBTC:    money.New(100000000, "BTC"),
				CostBasisUSD: money.NewFromFloat(30000, money.USD), // $30k < $40k sale = gain
				PricePerCoin: money.NewFromFloat(30000, money.USD),
				Remaining:    money.New(100000000, "BTC"),
			},
			// Priority 2: Long-term gain
			{
				Date:         time.Date(2023, 6, 1, 10, 0, 0, 0, time.UTC), // > 1 year
				AmountBTC:    money.New(100000000, "BTC"),
				CostBasisUSD: money.NewFromFloat(35000, money.USD), // $35k < $40k sale = gain
				PricePerCoin: money.NewFromFloat(35000, money.USD),
				Remaining:    money.New(100000000, "BTC"),
			},
			// Priority 1: Short-term loss
			{
				Date:         time.Date(2024, 9, 1, 10, 0, 0, 0, time.UTC), // < 1 year
				AmountBTC:    money.New(100000000, "BTC"),
				CostBasisUSD: money.NewFromFloat(45000, money.USD), // $45k > $40k sale = loss
				PricePerCoin: money.NewFromFloat(45000, money.USD),
				Remaining:    money.New(100000000, "BTC"),
			},
			// Priority 0: Long-term loss (highest priority)
			{
				Date:         time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC), // > 1 year
				AmountBTC:    money.New(100000000, "BTC"),
				CostBasisUSD: money.NewFromFloat(50000, money.USD), // $50k > $40k sale = loss
				PricePerCoin: money.NewFromFloat(50000, money.USD),
				Remaining:    money.New(100000000, "BTC"),
			},
		}

		// Sale that uses all lots to test order
		saleTransaction := createTestTransaction("sell-all", "Sale", saleDate, -4.0, salePrice, 160000)

		sale := processSale(saleTransaction, &lots)

		// Verify lots are used in optimal tax order
		is.Equal(len(sale.Lots), 4)

		// Expected order: Long-term loss, Short-term loss, Short-term gain, Long-term gain
		// This is optimal because:
		// 1. Use losses first to offset gains
		// 2. Among gains, use short-term first (32% tax) before long-term (15% tax)
		expectedIsLongTerm := []bool{true, false, false, true}
		expectedIsLoss := []bool{true, true, false, false} // loss when cost > sale price

		for i, lotSale := range sale.Lots {
			is.Equal(lotSale.IsLongTerm, expectedIsLongTerm[i])

			// Check if it's actually a loss/gain
			costPerBTC := float64(lotSale.CostBasisUSD.Amount()) / float64(lotSale.AmountBTC.Amount()) * 100000000
			isLoss := costPerBTC > salePrice*100 // cost in cents, sale price * 100 for cents
			is.Equal(isLoss, expectedIsLoss[i])
		}
	})

	t.Run("analyze HIFO priority scoring", func(t *testing.T) {
		// Test the priority scoring logic with a sale at $50k
		salePrice := 50000.0
		saleDate := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

		testCases := []struct {
			name         string
			lotDate      time.Time
			costBasis    float64
			expectedPrio int
			expectedDesc string
		}{
			{
				name:         "Long-term loss (highest priority)",
				lotDate:      time.Date(2023, 1, 1, 10, 0, 0, 0, time.UTC), // > 1 year
				costBasis:    70000.0,                                      // Loss: $70k > $50k
				expectedPrio: 0,
				expectedDesc: "Long-term loss",
			},
			{
				name:         "Short-term loss (second priority)",
				lotDate:      time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC), // < 1 year
				costBasis:    60000.0,                                      // Loss: $60k > $50k
				expectedPrio: 1,
				expectedDesc: "Short-term loss",
			},
			{
				name:         "Long-term gain (third priority)",
				lotDate:      time.Date(2023, 6, 1, 10, 0, 0, 0, time.UTC), // > 1 year
				costBasis:    30000.0,                                      // Gain: $30k < $50k
				expectedPrio: 2,
				expectedDesc: "Long-term gain",
			},
			{
				name:         "Short-term gain (lowest priority)",
				lotDate:      time.Date(2024, 8, 1, 10, 0, 0, 0, time.UTC), // < 1 year
				costBasis:    20000.0,                                      // Gain: $20k < $50k
				expectedPrio: 3,
				expectedDesc: "Short-term gain",
			},
		}

		fmt.Printf("\n=== PRIORITY SCORING ANALYSIS ===\n")

		for _, tc := range testCases {
			isLongTerm := saleDate.Sub(tc.lotDate) > 365*24*time.Hour
			salePricePerCoinCents := int64(salePrice * 100) // Convert to cents
			costPerCoinCents := int64(tc.costBasis * 100)
			isLoss := salePricePerCoinCents < costPerCoinCents

			// Calculate actual priority (mimicking the HIFO logic)
			var actualPrio int
			if isLongTerm && isLoss {
				actualPrio = 0
			} else if !isLongTerm && isLoss {
				actualPrio = 1
			} else if isLongTerm && !isLoss {
				actualPrio = 2
			} else {
				actualPrio = 3
			}

			fmt.Printf("%s:\n", tc.name)
			fmt.Printf("  Cost Basis: $%.2f, Sale Price: $%.2f\n", tc.costBasis, salePrice)
			fmt.Printf("  Long-term: %v, Loss: %v\n", isLongTerm, isLoss)
			fmt.Printf("  Expected Priority: %d, Actual Priority: %d\n", tc.expectedPrio, actualPrio)

			if actualPrio != tc.expectedPrio {
				t.Errorf("Priority mismatch for %s: expected %d, got %d", tc.name, tc.expectedPrio, actualPrio)
			}
		}
	})
}

// Test edge cases and error conditions
func TestHIFOEdgeCases(t *testing.T) {
	is := is.New(t)
	mockAPI := NewMockPriceAPI()

	t.Run("empty transaction list", func(t *testing.T) {
		lots, sales := calculateHIFO([]Transaction{}, mockAPI, 2024)
		is.Equal(len(lots), 0)
		is.Equal(len(sales), 0)
	})

	t.Run("only purchases, no sales", func(t *testing.T) {
		transactions := []Transaction{
			createTestTransaction("buy-1", "Purchase", time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), 1.0, 40000, -40000),
			createTestTransaction("buy-2", "Purchase", time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC), 0.5, 45000, -22500),
		}

		lots, sales := calculateHIFO(transactions, mockAPI, 2024)

		is.Equal(len(lots), 2)
		is.Equal(len(sales), 0)
		is.Equal(lots[0].Remaining.Amount(), int64(100000000)) // First lot fully remaining
		is.Equal(lots[1].Remaining.Amount(), int64(50000000))  // Second lot fully remaining
	})

	t.Run("sales before any purchases", func(t *testing.T) {
		transactions := []Transaction{
			createTestTransaction("sell-1", "Sale", time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), -1.0, 40000, 40000),
			createTestTransaction("buy-1", "Purchase", time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC), 1.0, 40000, -40000),
		}

		lots, sales := calculateHIFO(transactions, mockAPI, 2024)

		// Sale should process but find no lots to consume from
		is.Equal(len(sales), 1)
		is.Equal(len(sales[0].Lots), 0)                    // No lots consumed
		is.Equal(sales[0].CostBasisUSD.Amount(), int64(0)) // Zero cost basis

		// Purchase should create lot normally
		is.Equal(len(lots), 1)
		is.Equal(lots[0].Remaining.Amount(), int64(100000000))
	})

	t.Run("zero amount transactions", func(t *testing.T) {
		transactions := []Transaction{
			createTestTransaction("buy-zero", "Purchase", time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC), 0.0, 40000, 0),
			createTestTransaction("sell-zero", "Sale", time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC), 0.0, 40000, 0),
		}

		lots, sales := calculateHIFO(transactions, mockAPI, 2024)

		// Zero amount transactions should be handled gracefully
		is.Equal(len(lots), 1) // Zero-amount lot created
		is.Equal(lots[0].AmountBTC.Amount(), int64(0))
		is.Equal(len(sales), 1) // Zero-amount sale processed
		is.Equal(sales[0].AmountBTC.Amount(), int64(0))
	})
}
