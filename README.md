# Bitcoin HIFO Cost Basis Calculator

A Golang program to process Bitcoin transaction statements from Fold and calculate the Optimized HIFO (Highest In, First Out) cost basis for tax reporting purposes.

## Features

- **HIFO Cost Basis Calculation**: Uses the tax-optimized HIFO method to minimize capital gains
- **Beautiful Terminal Display**: Uses Charm Bracelet's lipgloss for formatted tables
- **IRS-Compliant Output**: Generates CSV files suitable for tax return preparation
- **Year Filtering**: Calculate cost basis for specific tax years
- **Comprehensive Reporting**: Shows sales details, remaining holdings, and summary statistics

## Installation

```bash
# Clone or download the project
cd hi-fold

# Install dependencies
go mod tidy

# Build the binary
go build -o hi-fold main.go
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

### Examples

```bash
# Calculate cost basis for 2025 with single file
./hi-fold --year 2025 --input fold-history-2025.csv

# Calculate for 2024 with multiple files
./hi-fold --year 2024 --input file1.csv --input file2.csv

# Multiple files using comma-separated syntax
./hi-fold --year 2025 --input file1.csv,file2.csv,file3.csv

# Custom output file
./hi-fold --year 2025 --input transactions.csv --output my-tax-records.csv
./hi-fold --year 2025 --output my-tax-records.csv

# Use mock prices for testing (offline mode)
./hi-fold --year 2025 --mock-prices
```

## Historical Price Integration

The program automatically fetches historical Bitcoin prices from [mempool.space](https://mempool.space) API for:

- **Deposit transactions** without price information
- **Transactions** missing price data
- **Accurate cost basis calculation** using real market prices

### Price API Features

- **Automatic lookup**: Fetches USD prices for the exact transaction date
- **Fallback handling**: Graceful error handling with warnings for API failures
- **Mock mode**: Use `--mock-prices` flag for testing without API calls
- **Rate limiting**: Built-in HTTP client with reasonable timeout

The mempool.space API provides reliable historical Bitcoin price data going back several years, ensuring accurate cost basis calculations for tax reporting.

## Multiple File Processing

The program supports processing multiple CSV files simultaneously with automatic deduplication:

### File Processing Features

- **Multiple input files**: Specify multiple CSV files using repeated `--input` flags or comma-separated values
- **Automatic deduplication**: Transactions with duplicate Reference IDs are automatically detected and skipped
- **Chronological sorting**: All transactions are sorted by date after merging
- **File validation**: Each input file is checked for existence before processing
- **Progress reporting**: Shows processing status for each file and duplicate detection

### Usage Examples

```bash
# Process multiple files with repeated flags
./hi-fold --year 2025 --input jan-2025.csv --input feb-2025.csv --input mar-2025.csv

# Process multiple files with comma-separated syntax
./hi-fold --year 2025 --input jan-2025.csv,feb-2025.csv,mar-2025.csv

# The program will report duplicates found:
# Processing file 1/3: jan-2025.csv
#   Loaded 25 transactions (0 duplicates skipped)
# Processing file 2/3: feb-2025.csv
#   Duplicate transaction found (Reference ID: abc-123), keeping first occurrence
#   Loaded 30 transactions (1 duplicates skipped)
```

## Input Format

The program expects CSV files exported from Fold with the following format:

- Account information in the first 3 rows
- Transaction header row starting with "Reference ID"
- Transaction data with columns: Reference ID, Date (UTC), Transaction Type, Description, Asset, Amount (BTC), Price per Coin (USD), Subtotal (USD), Fee (USD), Total (USD), Transaction ID

## Output

### Terminal Display

- **Summary Table**: Total sales, proceeds, cost basis, and gain/loss
- **Sales Details**: Individual sale transactions with HIFO calculations
- **Remaining Holdings**: Current Bitcoin holdings with cost basis

### CSV Output

The generated CSV file contains records suitable for IRS Form 8949:

- Description (BTC amount)
- Date Acquired
- Date Sold
- Proceeds
- Cost Basis
- Gain/Loss

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

**HIFO (Highest In, First Out)**: A tax optimization strategy where you sell the Bitcoin lots with the highest purchase price first, minimizing capital gains taxes. This only applies to Bitcoin still available for trading.

## HIFO Method

The program implements the Optimized HIFO (Highest In, First Out) method:

1. **Track Lots**: Each purchase creates a tax lot with acquisition date and cost basis
2. **Sales Matching**: When selling, match against lots with highest cost basis first
3. **Lot Tracking**: Maintain remaining quantities in each lot
4. **Gain/Loss Calculation**: Calculate capital gains/losses for each matched portion

## Tax Compliance

The output CSV format is designed to be compatible with:

- IRS Form 8949 (Sales and Other Dispositions of Capital Assets)
- Schedule D (Capital Gains and Losses)
- Popular tax software (TurboTax, FreeTaxUSA, etc.)

## Example Output

```text
Bitcoin HIFO Cost Basis Report - 2025

┌────────────────┬──────────┐
│Metric          │Value     │
├────────────────┼──────────┤
│Total Sales     │27        │
│Total Proceeds  │$118822.11│
│Total Cost Basis│$110990.11│
│Total Gain/Loss │$7832.00  │
└────────────────┴──────────┘
```

## Dependencies

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling and tables

## Disclaimer

This software is provided for informational purposes only and should not be considered tax advice. Always consult with a qualified tax professional for your specific situation. Verify all calculations before filing tax returns.
