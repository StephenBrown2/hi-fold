package main

import (
	"context"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/Rhymond/go-money"
	"github.com/charmbracelet/fang"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

var (
	targetYear      int
	inputFiles      []string
	outputFile      string
	outputDir       string
	useMockPrices   bool
	mempoolBaseURL  string
	clearCache      bool
	invalidateCache bool
	showCacheInfo   bool
)

var rootCmd = &cobra.Command{
	Use:   "hi-fold",
	Short: "Calculate HIFO cost basis for Bitcoin transactions",
	Long:  "Process Fold Bitcoin transaction CSV and calculate Optimized HIFO cost basis for tax reporting",
	Run:   runHIFO,
}

func init() {
	// Set up BTC currency for go-money
	money.AddCurrency("BTC", "₿", "1$", ".", ",", 8)

	rootCmd.Flags().IntVarP(&targetYear, "year", "y", 0, "Specific tax year to calculate (default: process all years with sales)")
	rootCmd.Flags().StringSliceVarP(&inputFiles, "input", "i", []string{}, "Input CSV files (can specify multiple)")
	rootCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output CSV file (only used with --year)")
	rootCmd.Flags().StringVar(&outputDir, "output-dir", ".", "Output directory for generated CSV files (default: current directory)")
	rootCmd.Flags().BoolVarP(&useMockPrices, "mock-prices", "m", false, "Use mock prices instead of API for testing")
	rootCmd.Flags().StringVar(&mempoolBaseURL, "mempool-url", "", "Custom mempool API base URL (e.g., mempool.space, https://mempool.space, http://192.168.1.100:8080)")
	rootCmd.Flags().BoolVar(&clearCache, "clear-cache", false, "Clear entire cache and recalculate from scratch")
	rootCmd.Flags().BoolVar(&invalidateCache, "invalidate-cache", false, "Invalidate cache and recalculate from scratch")
	rootCmd.Flags().BoolVar(&showCacheInfo, "show-cache-info", false, "Show cache information and exit")
}

// expandGlobPatterns expands glob patterns in file paths and returns actual file paths
func expandGlobPatterns(patterns []string) ([]string, error) {
	var expandedFiles []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern '%s': %v", pattern, err)
		}

		if len(matches) == 0 {
			// If no matches found, check if it's a literal file that doesn't exist
			if _, err := os.Stat(pattern); os.IsNotExist(err) {
				return nil, fmt.Errorf("no files match pattern or file does not exist: %s", pattern)
			}
			// If it's a literal file that exists, add it
			expandedFiles = append(expandedFiles, pattern)
		} else {
			expandedFiles = append(expandedFiles, matches...)
		}
	}

	return expandedFiles, nil
}

func main() {
	if err := fang.Execute(context.Background(), rootCmd, fang.WithoutVersion()); err != nil {
		log.Fatal(err)
	}
}

func runHIFO(cmd *cobra.Command, args []string) {
	// Initialize cache
	cache := NewCache(afero.NewOsFs(), "hi-fold")

	if clearCache {
		if err := cache.Clear(); err != nil {
			log.Fatalf("Error clearing cache: %v", err)
		}
		fmt.Println("Cache cleared successfully")
		if len(inputFiles) == 0 {
			return
		}
	}

	if len(inputFiles) == 0 {
		log.Fatal("Error: At least one input CSV file must be specified with --input/-i")
	}

	// Expand glob patterns to actual file paths
	expandedFiles, err := expandGlobPatterns(inputFiles)
	if err != nil {
		log.Fatalf("Error expanding file patterns: %v", err)
	}

	if len(expandedFiles) == 0 {
		log.Fatal("Error: No files found matching the specified patterns")
	}

	// Check that all expanded files exist and are readable
	for _, file := range expandedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			log.Fatalf("Error: Input file does not exist: %s", file)
		}
	}

	// Handle cache info request
	if showCacheInfo {
		showCacheInformation(cache, expandedFiles)
		return
	}

	// Initialize price API
	var priceAPI PriceAPI
	if useMockPrices {
		priceAPI = NewMockPriceAPI()
		fmt.Println("Using mock prices for testing")
	} else {
		priceAPI = NewMempoolPriceAPIWithURL(mempoolBaseURL)
		if mempoolBaseURL != "" {
			fmt.Printf("Using custom mempool API: %s\n", mempoolBaseURL)
		} else {
			fmt.Println("Using mempool.space API for historical prices")
		}
	}

	// Parse and merge CSV files
	transactions, err := parseAndMergeCSVs(expandedFiles)
	if err != nil {
		log.Fatalf("Error parsing CSV files: %v", err)
	}

	fmt.Printf("Loaded %d unique transactions from %d file(s)\n", len(transactions), len(expandedFiles))

	if targetYear != 0 {
		// Single year mode - use cache if available
		processSingleYear(targetYear, transactions, priceAPI, cache, expandedFiles)
	} else {
		// Multi-year mode - process all years with sales
		processAllYears(transactions, priceAPI, cache, expandedFiles)
	}
}

func showCacheInformation(cache *Cache, inputFiles []string) {
	fmt.Println("Cache Information:")
	fmt.Printf("Cache directory: %s\n", cache.GetCacheInfo())

	// Find all years with potential cache entries
	// This is a simplified implementation - in practice you might scan the cache directory
	fmt.Println("Use --invalidate-cache to clear cached data")
}

func processSingleYear(year int, transactions []Transaction, priceAPI PriceAPI, cache *Cache, inputFiles []string) {
	if invalidateCache {
		if err := cache.invalidateCache(year, inputFiles); err != nil {
			log.Printf("Warning: Failed to invalidate cache: %v", err)
		} else {
			fmt.Printf("Cache invalidated for year %d\n", year)
		}
	}

	// Calculate HIFO cost basis using cache
	lots, sales := calculateHIFOWithCache(transactions, priceAPI, year, cache, inputFiles)

	// Display results
	displayResults(lots, sales, transactions, year, priceAPI)

	// Generate tax records CSV
	if outputFile == "" {
		outputFile = fmt.Sprintf("tax-records-%d.csv", year)
	}

	if err := generateTaxRecords(sales, outputFile); err != nil {
		log.Fatalf("Error generating tax records: %v", err)
	}

	fmt.Printf("\nTax records saved to: %s\n", outputFile)
}

func processAllYears(transactions []Transaction, priceAPI PriceAPI, cache *Cache, inputFiles []string) {
	// Find all years with sales
	salesByYear := make(map[int][]Sale)

	// Process transactions chronologically to build lots and track sales by year
	allYearResults := calculateAllYearsWithCache(transactions, priceAPI, cache, inputFiles)

	if len(allYearResults) == 0 {
		fmt.Println("No sales found in any year")
		return
	}

	// Display results in chronological order (earlier to later years)
	for _, year := range slices.Sorted(maps.Keys(allYearResults)) {
		result := allYearResults[year]
		salesByYear[year] = result.Sales

		// Display results for this year
		fmt.Printf("\n%s\n", "==================================================================================")
		displayResults(result.Lots, result.Sales, transactions, year, priceAPI)

		// Generate CSV for this year
		outputFile := filepath.Join(outputDir, fmt.Sprintf("tax-records-%d.csv", year))
		if err := generateTaxRecords(result.Sales, outputFile); err != nil {
			log.Printf("Error generating tax records for %d: %v", year, err)
		} else {
			fmt.Printf("Tax records for %d saved to: %s\n", year, outputFile)
		}
	}

	fmt.Printf("\nGenerated tax records for %d year(s)\n", len(allYearResults))
}
