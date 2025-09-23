# Bitcoin HIFO Cost Basis Calculator

A Golang program to process Bitcoin transaction statements from Fold and calculate the Optimized HIFO (Highest In, First Out) cost basis for tax reporting purposes.

## ⚠️ Important: Complete Transaction History Required

**For accurate HIFO calculations, it is highly recommended to include ALL transaction history from Fold.** The HIFO algorithm requires complete transaction history to properly:

- Build accurate tax lot inventory from all purchases and deposits
- Calculate correct cost basis using the highest-cost lots first
- Handle withdrawals that reduce lot quantities without creating taxable events
- Ensure chronological integrity (sales can only use lots acquired before the sale date)

Even if you're only generating a tax report for 2025, include transaction files from 2022, 2023, and 2024 for the most accurate calculations.

## Features

- **HIFO Cost Basis Calculation**: Uses the tax-optimized HIFO method to minimize capital gains
- **Multi-Year Processing**: Processes all historical transactions but reports on specific tax years
- **Beautiful Terminal Display**: Uses Charm Bracelet's lipgloss for formatted tables with color-coded gains/losses
- **IRS-Compliant Output**: Generates CSV files suitable for tax return preparation (Form 8949)
- **Historical Price Integration**: Automatically fetches missing Bitcoin prices from mempool.space API
- **Modular Architecture**: Clean separation of concerns with dedicated modules for parsing, calculations, and display
- **Precise Monetary Calculations**: Uses go-money library for exact decimal arithmetic avoiding floating-point errors

## Installation

```bash
# Clone or download the project
cd hi-fold

# Install dependencies
go mod tidy

# Build the binary
go build -o hi-fold .
```

## Usage

### Command Line Options

```bash
./hi-fold [flags]

Flags:
  -h, --help            help for hi-fold
  -i, --input strings   Input CSV files (can specify multiple, required)
  -m, --mock-prices     Use mock prices instead of API for testing
  -o, --output string   Output CSV file (default: tax-records-{year}.csv)
  -y, --year int        Tax year to calculate (default: previous year)
```

### Basic Usage Examples

```bash
# Calculate cost basis for 2025 with single file
./hi-fold --year 2025 --input fold-history-2025.csv

# Custom output file
./hi-fold --year 2025 --input transactions.csv --output my-tax-records.csv

# Use mock prices for testing (offline mode)
./hi-fold --year 2025 --input transactions.csv --mock-prices
```

### Multiple File Processing

The program supports processing multiple CSV files simultaneously with automatic deduplication:

```bash
# Process multiple files with repeated flags
./hi-fold --year 2025 --input jan-2025.csv --input feb-2025.csv --input mar-2025.csv

# Process multiple files with comma-separated syntax
./hi-fold --year 2025 --input jan-2025.csv,feb-2025.csv,mar-2025.csv

# Use glob patterns to match multiple files automatically
./hi-fold --year 2025 --input "fold-*.csv"

# More specific glob patterns
./hi-fold --year 2025 --input "fold-*-202[45].csv"

# Example with complete history for accurate HIFO calculations
./hi-fold --year 2025 --input fold-2022.csv,fold-2023.csv,fold-2024.csv,fold-2025.csv
```

#### File Processing Features

- **Glob pattern support**: Use wildcards like `*.csv` or `fold-*-20*.csv` to match multiple files automatically
- **Automatic deduplication**: Transactions with duplicate Reference IDs are detected and skipped
- **Chronological sorting**: All transactions are sorted by date after merging
- **File validation**: Each input file is checked for existence before processing
- **Progress reporting**: Shows processing status and duplicate detection results

## Historical Price Integration

The program automatically fetches historical Bitcoin prices from [mempool.space](https://mempool.space) API for transactions missing price data, ensuring accurate cost basis calculations. Use the `--mock-prices` flag for testing without API calls.

## Input Format

The program expects CSV files exported from Fold with the following format:

- Account information in the first 3 rows
- Transaction header row starting with "Reference ID"
- Transaction data with columns: Reference ID, Date (UTC), Transaction Type, Description, Asset, Amount (BTC), Price per Coin (USD), Subtotal (USD), Fee (USD), Total (USD), Transaction ID

## Output Formats

The program generates two types of output:

### Terminal Display

- **Summary Table**: Total sales, proceeds, cost basis, and gain/loss for the target year
- **Sales Details**: Aggregated view showing one row per sale transaction with total amounts
- **Holdings Details**: Current Bitcoin holdings with acquisition dates and cost basis
- **Holdings Summary**: Net position, average price, and unrealized gains/losses

### CSV Output (tax-records-{year}.csv)

The CSV file breaks down each sale into **individual tax lots** for IRS Form 8949 compliance:

- **Multiple rows per sale**: Each sale transaction generates multiple CSV rows (one per tax lot used)
- **Lot-by-lot breakdown**: Shows exactly which acquisition lots were sold and their individual gains/losses
- **HIFO ordering**: Reflects the highest-cost lots being sold first

**CSV Columns**: Description, Date Acquired, Date Sold, Proceeds, Cost Basis, Gain/Loss

## Understanding Holdings Metrics

The program displays two important but different metrics in the Holdings Summary:

### Net BTC Position

This represents your **actual Bitcoin balance** - the simple mathematical sum of all Bitcoin transactions that have affected your wallet balance. It includes:

- ✅ **Purchases** (positive amounts)
- ✅ **Deposits/Receives** (positive amounts)
- ✅ **Sales** (negative amounts)
- ✅ **Withdrawals** (negative amounts)

**Example**: If you bought 1.0 BTC, received 0.5 BTC, sold 0.3 BTC, and withdrew 0.8 BTC, your Net BTC Position would be: 1.0 + 0.5 - 0.3 - 0.8 = **0.4 BTC**

### Remaining Holdings After HIFO Processing

This represents the Bitcoin amounts that are still available for **tax lot tracking** and cost basis calculations. This number excludes Bitcoin that has been withdrawn from the exchange because:

1. **Withdrawn Bitcoin cannot be sold from the exchange** (it's no longer in your exchange account)
2. **Tax lots track what you can sell** for capital gains calculations
3. **HIFO processing only applies to tradeable assets** still under exchange custody

**Key Difference**: Withdrawals reduce your Net BTC Position (because you no longer own that Bitcoin in that account) but don't affect tax lot calculations for the remaining Bitcoin you can still trade.

### Why They Differ

- **Net BTC Position**: Your actual Bitcoin ownership across all transactions
- **Remaining Holdings**: Only the Bitcoin still available for trading and tax lot calculations

**Real-world analogy**: Think of it like a bank account. If you deposit $1000, withdraw $300 in cash, your account balance is $700 (like Net BTC Position). But if you want to calculate interest on money still in the bank, you only count the $700 remaining (like Remaining Holdings).

### Tax Implications

- **Net BTC Position**: Useful for understanding your total Bitcoin exposure
- **Remaining Holdings**: What matters for future capital gains calculations when you sell from the exchange

## HIFO Algorithm

The program implements the Optimized HIFO (Highest In, First Out) method - a tax optimization strategy that sells the highest-cost Bitcoin lots first to minimize capital gains.

### How It Works

1. **Multi-Year Processing**: Processes ALL transactions chronologically to build complete lot inventory
2. **Lot Creation**: Each purchase/deposit creates a tax lot with acquisition date, amount, and cost basis
3. **HIFO Sales Matching**: When selling, matches against lots with highest price-per-coin first
4. **Withdrawal Handling**: Withdrawals reduce lot quantities without creating taxable events
5. **Year-Specific Reporting**: Only sales from the target year appear in output

### Key Features

- **Precise Calculations**: Uses go-money library to avoid floating-point precision errors
- **Chronological Integrity**: Sales can only use lots acquired before the sale date
- **Lot Splitting**: Supports partial lot sales with accurate remaining quantities
- **Complete History Required**: Processes all years to ensure accurate cost basis calculations

## Tax Compliance

The output CSV format is designed to be compatible with:

- IRS Form 8949 (Sales and Other Dispositions of Capital Assets)
- Schedule D (Capital Gains and Losses)
- Popular tax software (TurboTax, FreeTaxUSA, etc.)

## Complete Example: Processing and Output

This section shows a complete example of processing multiple files and the resulting output.

### Command Example

```bash
# Process complete transaction history for accurate HIFO calculations using glob patterns
./hi-fold --year 2025 --input "fold-*.csv"

# Alternative: specify files explicitly
./hi-fold --year 2025 --input fold-2024.csv,fold-2025.csv
```

### Sample Terminal Output

```text
Using mempool.space API for historical prices
Processing file 1/2: fold-bitcoin-transaction-history-2024.csv
  Loaded 23 transactions (0 duplicates skipped)
Processing file 2/2: fold-bitcoin-transaction-history-2025.csv
  Loaded 82 transactions (0 duplicates skipped)
Loaded 105 unique transactions from 2 file(s)


 Bitcoin HIFO Cost Basis Report - 2025


┌────────────────┬───────────┐
│     Metric     │   Value   │
├────────────────┼───────────┤
│Total Sales     │         28│
│Total Proceeds  │$119,821.31│
│Total Cost Basis│$122,268.38│
│Total Gain/Loss │ -$2,447.07│
└────────────────┴───────────┘

 Sales Details
┌──────────┬────────────┬────────────┬──────────────┬───────────┬─────────────┐
│Date Sold │Amount (BTC)│Proceeds ($)│Cost Basis ($)│ Price/BTC │Gain/Loss ($)│
├──────────┼────────────┼────────────┼──────────────┼───────────┼─────────────┤
│2025-02-01│ 0.02473770₿│   $2,501.05│     $2,644.38│$101,102.77│     -$143.33│
│2025-02-03│ 0.01115567₿│   $1,041.77│     $1,048.86│ $93,394.00│       -$7.09│
│2025-02-14│ 0.01000000₿│     $945.46│     $1,045.46│ $94,546.00│     -$100.00│
...
└──────────┴────────────┴────────────┴──────────────┴───────────┴─────────────┘

Remaining Holdings

┌─────────────┬────────────┬──────────────┬───────────┐
│Date Acquired│Amount (BTC)│Cost Basis ($)│ Price/BTC │
├─────────────┼────────────┼──────────────┼───────────┤
│ 2024-10-30  │ 0.01718401₿│     $1,250.00│ $72,742.04│
│ 2025-02-28  │ 0.03675968₿│     $3,000.00│ $81,611.15│
│ 2025-03-19  │ 0.00299466₿│       $250.00│ $83,481.93│
...
└─────────────┴────────────┴──────────────┴───────────┘

┌────────────────────┬───────────┐
│  Holdings Summary  │   Value   │
├────────────────────┼───────────┤
│Net BTC Position    │0.12345678₿│
│Average BTC Price   │ $84,210.45│
│Total Cost Basis    │ $10,396.35│
│Current BTC Price   │$111,111.00│
│Current Value       │ $13,717.41│
│Unrealized Gain/Loss│  $3,321.06│
└────────────────────┴───────────┘
```

### CSV Output (tax-records-2025.csv)

```csv
Description,Date Acquired,Date Sold,Proceeds,Cost Basis,Gain/Loss
0.01115567 BTC,12/15/2024,02/03/2025,1041.77,1048.86,-7.09
0.01000000 BTC,12/15/2024,02/14/2025,945.46,1045.46,-100.00
```

### Key Difference: Terminal vs CSV Output

**Terminal Sales Details** (aggregated per transaction):

```text
2025-03-15 | 0.50000000₿ | $45,000.00 | $42,000.00 | $90,000.00 | $3,000.00
```

**CSV Output** (individual tax lots from same transaction):

```csv
0.30000000 BTC, 2024-01-15, 2025-03-15, $27,000.00, $24,000.00, $3,000.00
0.20000000 BTC, 2024-02-20, 2025-03-15, $18,000.00, $18,000.00, $0.00
```

The terminal shows **aggregated sales**, while the CSV shows **individual tax lot breakdowns** required for Form 8949.

## Project Structure

The application is organized into focused modules:

- **`main.go`**: CLI interface and application coordination
- **`models.go`**: Data structures (Transaction, Lot, Sale, etc.) and helper functions
- **`csv.go`**: CSV parsing and tax record generation
- **`hifo.go`**: Core HIFO algorithm implementation and lot matching logic
- **`display.go`**: Terminal output formatting and styled table rendering

## Dependencies

- [go-money](https://github.com/Rhymond/go-money) - Precise monetary calculations with 8-decimal BTC support
- [Cobra](https://github.com/spf13/cobra) - CLI framework and command handling
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling and table formatting
- [Fang](https://github.com/charmbracelet/fang) - Enhanced CLI execution

## Disclaimer

This software is provided for informational purposes only and should not be considered tax advice. Always consult with a qualified tax professional for your specific situation. Verify all calculations before filing tax returns.
