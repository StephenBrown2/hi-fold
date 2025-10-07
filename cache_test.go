package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/matryer/is"
	"github.com/spf13/afero"
)

func TestCache(t *testing.T) {
	is := is.New(t)

	t.Run("create new cache", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-program")

		is.True(cache != nil)
		is.True(cache.GetCacheInfo() != "")
	})

	t.Run("cache put and get operations", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-cache-put-get")

		testData := []byte("test cache data")
		err := cache.Put("test-key", testData)
		is.NoErr(err)

		retrievedData, err := cache.Get("test-key")
		is.NoErr(err)
		is.Equal(string(retrievedData), string(testData))
	})

	t.Run("in-memory cache put and get operations", func(t *testing.T) {
		// Test with in-memory filesystem
		cache := NewCache(afero.NewMemMapFs(), "test-in-memory")

		testData := []byte("test cache data in memory")
		err := cache.Put("test-key", testData)
		is.NoErr(err)

		retrievedData, err := cache.Get("test-key")
		is.NoErr(err)
		is.Equal(string(retrievedData), string(testData))
	})

	t.Run("cache get non-existent key", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-cache-missing")

		_, err := cache.Get("non-existent-key")
		is.True(err != nil)
		is.True(os.IsNotExist(err))
	})

	t.Run("cache clear", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-cache-clear")

		// Put some data
		err := cache.Put("test-key", []byte("test data"))
		is.NoErr(err)

		// Verify data exists
		_, err = cache.Get("test-key")
		is.NoErr(err)

		// Clear cache
		err = cache.Clear()
		is.NoErr(err)

		// Verify data no longer exists
		_, err = cache.Get("test-key")
		is.True(err != nil)
	})
}

func TestGenerateFileHash(t *testing.T) {
	is := is.New(t)

	t.Run("generate hash for existing file", func(t *testing.T) {
		// Use one of our test files
		hash, err := generateFileHash("testdata/simple_transactions.csv")
		is.NoErr(err)
		is.True(hash != "")
		is.Equal(len(hash), 64) // SHA256 produces 64-char hex string

		// Generate hash again - should be identical
		hash2, err := generateFileHash("testdata/simple_transactions.csv")
		is.NoErr(err)
		is.Equal(hash, hash2)
	})

	t.Run("generate hash for different files produces different hashes", func(t *testing.T) {
		hash1, err := generateFileHash("testdata/simple_transactions.csv")
		is.NoErr(err)

		hash2, err := generateFileHash("testdata/hifo_test.csv")
		is.NoErr(err)

		is.True(hash1 != hash2)
	})

	t.Run("generate hash for non-existent file", func(t *testing.T) {
		_, err := generateFileHash("testdata/non-existent.csv")
		is.True(err != nil)
	})

	t.Run("hash consistency for identical content", func(t *testing.T) {
		// Create two temporary files with identical content
		tempDir := t.TempDir()

		content := []byte("identical content for testing")

		file1 := filepath.Join(tempDir, "file1.txt")
		file2 := filepath.Join(tempDir, "file2.txt")

		err := os.WriteFile(file1, content, 0o644)
		is.NoErr(err)

		err = os.WriteFile(file2, content, 0o644)
		is.NoErr(err)

		hash1, err := generateFileHash(file1)
		is.NoErr(err)

		hash2, err := generateFileHash(file2)
		is.NoErr(err)

		is.Equal(hash1, hash2)
	})
}

func TestGenerateInputHashes(t *testing.T) {
	is := is.New(t)

	t.Run("generate hashes for multiple files", func(t *testing.T) {
		files := []string{
			"testdata/hifo_test.csv",
			"testdata/simple_transactions.csv",
		}

		hashes, err := generateInputHashes(files)
		is.NoErr(err)
		is.Equal(len(hashes), 2)

		// Expected results (generateInputHashes sorts by full path, but stores base name in hash)
		// These are the actual SHA256 hashes of the test files - hardcoded for reliable testing
		expectedResults := make([]struct {
			filename     string
			expectedHash string
		}, len(files))
		for i := range files {
			expectedResults[i].filename = filepath.Base(files[i])
			out, _ := exec.Command("sha256sum", files[i]).Output()
			expectedResults[i].expectedHash, _, _ = strings.Cut(string(out), " ")
		}

		// Validate each hash contains the expected filename and exact hash value
		for i, hash := range hashes {
			is.True(len(hash) > 0)

			// Should contain colon separator
			parts := strings.Split(hash, ":")
			is.Equal(len(parts), 2)

			filename := parts[0]
			hashValue := parts[1]

			// Verify actual filename matches expected sorted order
			is.Equal(filename, expectedResults[i].filename)

			// Verify exact hash value matches expected (validates actual file content)
			is.Equal(hashValue, expectedResults[i].expectedHash)

			// Hash should be 64 chars (SHA256) and only contain hex characters
			is.Equal(len(hashValue), 64)

			// Verify hash contains only valid hex characters
			for _, char := range hashValue {
				is.True((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f'))
			}
		}
	})

	t.Run("consistent ordering regardless of input order", func(t *testing.T) {
		files1 := []string{
			"testdata/simple_transactions.csv",
			"testdata/hifo_test.csv",
		}

		files2 := []string{
			"testdata/hifo_test.csv",
			"testdata/simple_transactions.csv",
		}

		hashes1, err := generateInputHashes(files1)
		is.NoErr(err)

		hashes2, err := generateInputHashes(files2)
		is.NoErr(err)

		// Should be identical despite different input order
		is.Equal(len(hashes1), len(hashes2))
		is.Equal(hashes1[0], hashes2[0])
		is.Equal(hashes1[1], hashes2[1])
	})

	t.Run("error for non-existent file", func(t *testing.T) {
		files := []string{
			"testdata/simple_transactions.csv",
			"testdata/non-existent.csv",
		}

		_, err := generateInputHashes(files)
		is.True(err != nil)
	})

	t.Run("empty file list", func(t *testing.T) {
		hashes, err := generateInputHashes([]string{})
		is.NoErr(err)
		is.Equal(len(hashes), 0)
	})

	t.Run("generate hashes with in-memory filesystem", func(t *testing.T) {
		// Create in-memory filesystem with test files
		memFs := afero.NewMemMapFs()

		// Create test files in memory
		err := afero.WriteFile(memFs, "file1.txt", []byte("content1"), 0o644)
		is.NoErr(err)

		err = afero.WriteFile(memFs, "file2.txt", []byte("content2"), 0o644)
		is.NoErr(err)

		files := []string{"file1.txt", "file2.txt"}
		hashes, err := generateInputHashesWithFs(memFs, files)
		is.NoErr(err)
		is.Equal(len(hashes), 2)

		// Verify structure
		for _, hash := range hashes {
			is.True(len(hash) > 0)
			parts := strings.Split(hash, ":")
			is.Equal(len(parts), 2)
			is.Equal(len(parts[1]), 64) // SHA256 hash length
		}

		// Verify consistency - same input should produce same hashes
		hashes2, err := generateInputHashesWithFs(memFs, files)
		is.NoErr(err)
		is.Equal(hashes[0], hashes2[0])
		is.Equal(hashes[1], hashes2[1])
	})
}

func TestGenerateCacheKey(t *testing.T) {
	is := is.New(t)

	t.Run("generate cache key for year and files", func(t *testing.T) {
		files := []string{"testdata/simple_transactions.csv"}

		key, err := generateCacheKey(2024, files)
		is.NoErr(err)
		is.True(key != "")
		is.True(key[:9] == "year_2024")      // Should start with year
		is.True(key[len(key)-5:] == ".json") // Should end with .json
	})

	t.Run("different years produce different keys", func(t *testing.T) {
		files := []string{"testdata/simple_transactions.csv"}

		key2024, err := generateCacheKey(2024, files)
		is.NoErr(err)

		key2025, err := generateCacheKey(2025, files)
		is.NoErr(err)

		is.True(key2024 != key2025)
	})

	t.Run("different file sets produce different keys", func(t *testing.T) {
		files1 := []string{"testdata/simple_transactions.csv"}
		files2 := []string{"testdata/hifo_test.csv"}

		key1, err := generateCacheKey(2024, files1)
		is.NoErr(err)

		key2, err := generateCacheKey(2024, files2)
		is.NoErr(err)

		is.True(key1 != key2)
	})

	t.Run("same inputs produce same key", func(t *testing.T) {
		files := []string{"testdata/simple_transactions.csv"}

		key1, err := generateCacheKey(2024, files)
		is.NoErr(err)

		key2, err := generateCacheKey(2024, files)
		is.NoErr(err)

		is.Equal(key1, key2)
	})
}

func TestYearEndState(t *testing.T) {
	is := is.New(t)

	t.Run("marshal and unmarshal year-end state with full data", func(t *testing.T) {
		// Create sample lots and sales with full data
		lot := Lot{
			Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			AmountBTC:    money.New(100000000, "BTC"), // 1 BTC
			CostBasisUSD: money.NewFromFloat(40000, money.USD),
			PricePerCoin: money.NewFromFloat(40000, money.USD),
			Remaining:    money.New(50000000, "BTC"), // 0.5 BTC remaining
		}

		sale := Sale{
			Date:         time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
			AmountBTC:    money.New(50000000, "BTC"), // 0.5 BTC
			ProceedsUSD:  money.NewFromFloat(30000, money.USD),
			CostBasisUSD: money.NewFromFloat(20000, money.USD),
			GainLossUSD:  money.NewFromFloat(10000, money.USD),
			Lots: []LotSale{
				{
					LotDate:      lot.Date,
					AmountBTC:    money.New(50000000, "BTC"),
					CostBasisUSD: money.NewFromFloat(20000, money.USD),
					PricePerCoin: money.NewFromFloat(40000, money.USD),
					IsLongTerm:   false,
				},
			},
		}

		originalState := YearEndState{
			Year:        2024,
			Lots:        []Lot{lot},
			Sales:       []Sale{sale},
			InputHashes: []string{"simple_transactions.csv:abc123"},
			CreatedAt:   time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
		}

		// Marshal to JSON
		data, err := json.Marshal(originalState)
		is.NoErr(err)
		is.True(len(data) > 0)

		// Unmarshal back
		var restoredState YearEndState
		err = json.Unmarshal(data, &restoredState)
		is.NoErr(err)

		// Verify the basic state properties
		is.Equal(originalState.Year, restoredState.Year)
		is.Equal(len(originalState.Lots), len(restoredState.Lots))
		is.Equal(len(originalState.Sales), len(restoredState.Sales))
		is.Equal(len(originalState.InputHashes), len(restoredState.InputHashes))
		is.Equal(originalState.InputHashes[0], restoredState.InputHashes[0])
		is.True(originalState.CreatedAt.Equal(restoredState.CreatedAt))

		// Verify lot data preservation through JSON
		restoredLot := restoredState.Lots[0]
		is.Equal(restoredLot.AmountBTC.Amount(), lot.AmountBTC.Amount())
		is.Equal(restoredLot.CostBasisUSD.Amount(), lot.CostBasisUSD.Amount())
		is.Equal(restoredLot.PricePerCoin.Amount(), lot.PricePerCoin.Amount())
		is.Equal(restoredLot.Remaining.Amount(), lot.Remaining.Amount())
		is.True(restoredLot.Date.Equal(lot.Date))

		// Verify sale data preservation through JSON
		restoredSale := restoredState.Sales[0]
		is.Equal(restoredSale.AmountBTC.Amount(), sale.AmountBTC.Amount())
		is.Equal(restoredSale.ProceedsUSD.Amount(), sale.ProceedsUSD.Amount())
		is.Equal(restoredSale.CostBasisUSD.Amount(), sale.CostBasisUSD.Amount())
		is.Equal(restoredSale.GainLossUSD.Amount(), sale.GainLossUSD.Amount())
		is.True(restoredSale.Date.Equal(sale.Date))
		is.Equal(len(restoredSale.Lots), len(sale.Lots))

		// Verify nested LotSale data preservation
		restoredLotSale := restoredSale.Lots[0]
		originalLotSale := sale.Lots[0]
		is.Equal(restoredLotSale.AmountBTC.Amount(), originalLotSale.AmountBTC.Amount())
		is.Equal(restoredLotSale.CostBasisUSD.Amount(), originalLotSale.CostBasisUSD.Amount())
		is.Equal(restoredLotSale.PricePerCoin.Amount(), originalLotSale.PricePerCoin.Amount())
		is.Equal(restoredLotSale.IsLongTerm, originalLotSale.IsLongTerm)
		is.True(restoredLotSale.LotDate.Equal(originalLotSale.LotDate))
	})
}

func TestCacheYearEndState(t *testing.T) {
	is := is.New(t)

	t.Run("save and load year-end state", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-year-end-state")

		// Create sample data
		lot := Lot{
			Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			AmountBTC:    money.New(100000000, "BTC"),
			CostBasisUSD: money.NewFromFloat(40000, money.USD),
			PricePerCoin: money.NewFromFloat(40000, money.USD),
			Remaining:    money.New(100000000, "BTC"),
		}

		sale := Sale{
			Date:         time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
			AmountBTC:    money.New(50000000, "BTC"),
			ProceedsUSD:  money.NewFromFloat(30000, money.USD),
			CostBasisUSD: money.NewFromFloat(20000, money.USD),
			GainLossUSD:  money.NewFromFloat(10000, money.USD),
			Lots: []LotSale{
				{
					LotDate:      lot.Date,
					AmountBTC:    money.New(50000000, "BTC"),
					CostBasisUSD: money.NewFromFloat(20000, money.USD),
					PricePerCoin: money.NewFromFloat(40000, money.USD),
					IsLongTerm:   false,
				},
			},
		}

		inputFiles := []string{"testdata/simple_transactions.csv"}

		// Save year-end state
		err := cache.saveYearEndState(2024, []Lot{lot}, []Sale{sale}, inputFiles)
		is.NoErr(err)

		// Load year-end state
		loadedState, err := cache.loadYearEndState(2024, inputFiles)
		is.NoErr(err)
		is.True(loadedState != nil)

		is.Equal(loadedState.Year, 2024)
		is.Equal(len(loadedState.Lots), 1)
		is.Equal(len(loadedState.Sales), 1)

		// Verify lot data
		loadedLot := loadedState.Lots[0]
		is.Equal(loadedLot.AmountBTC.Amount(), lot.AmountBTC.Amount())
		is.Equal(loadedLot.CostBasisUSD.Amount(), lot.CostBasisUSD.Amount())

		// Verify sale data
		loadedSale := loadedState.Sales[0]
		is.Equal(loadedSale.AmountBTC.Amount(), sale.AmountBTC.Amount())
		is.Equal(loadedSale.ProceedsUSD.Amount(), sale.ProceedsUSD.Amount())
	})

	t.Run("cache miss for non-existent year", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-cache-miss")

		inputFiles := []string{"testdata/simple_transactions.csv"}

		_, err := cache.loadYearEndState(9999, inputFiles)
		is.True(err != nil) // Should be cache miss
	})

	t.Run("cache invalidation when input files change", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-cache-invalidation")

		// Save with one set of input files
		inputFiles1 := []string{"testdata/simple_transactions.csv"}
		err := cache.saveYearEndState(2024, []Lot{}, []Sale{}, inputFiles1)
		is.NoErr(err)

		// Try to load with different input files - should be a cache miss
		inputFiles2 := []string{"testdata/hifo_test.csv"}
		_, err = cache.loadYearEndState(2024, inputFiles2)
		is.True(err != nil) // Should be cache miss due to different input files
	})

	t.Run("invalidate cache manually", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-manual-invalidation")

		inputFiles := []string{"testdata/simple_transactions.csv"}

		// Save some data
		err := cache.saveYearEndState(2024, []Lot{}, []Sale{}, inputFiles)
		is.NoErr(err)

		// Verify it exists
		_, err = cache.loadYearEndState(2024, inputFiles)
		is.NoErr(err)

		// Invalidate cache
		err = cache.invalidateCache(2024, inputFiles)
		is.NoErr(err)

		// Verify it's gone
		_, err = cache.loadYearEndState(2024, inputFiles)
		is.True(err != nil) // Should be cache miss
	})

	t.Run("handle corrupted cache data", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-corrupted-cache")

		// Write invalid JSON to cache
		cacheKey, err := generateCacheKey(2024, []string{"testdata/simple_transactions.csv"})
		is.NoErr(err)

		err = cache.Put(cacheKey, []byte("invalid json"))
		is.NoErr(err)

		// Try to load - should fail gracefully
		_, err = cache.loadYearEndState(2024, []string{"testdata/simple_transactions.csv"})
		is.True(err != nil)
		is.True(err.Error() == "failed to unmarshal cached state: invalid character 'i' looking for beginning of value")
	})
}

// Test in-memory cache operations for better test isolation
func TestInMemoryCacheYearEndState(t *testing.T) {
	is := is.New(t)

	t.Run("save and load year-end state with in-memory filesystem", func(t *testing.T) {
		// Use in-memory filesystem for faster, isolated testing
		memFs := afero.NewMemMapFs()
		cache := NewCache(memFs, "test-in-memory-year-end")

		// Also create an in-memory filesystem for the input files
		inputFs := afero.NewMemMapFs()

		// Create test input file
		testCSVContent := `"Reference ID","Date (UTC)","Transaction Type","Description","Asset","Amount (BTC)","Price per Coin (USD)","Subtotal (USD)","Fee (USD)","Total (USD)","Transaction ID"
"test-001","2024-01-01 10:00:00.000000+00:00","Purchase","Test Purchase","BTC","1.00000000","40000.00","-40000.00","0.00","-40000.00",""`

		err := afero.WriteFile(inputFs, "test.csv", []byte(testCSVContent), 0o644)
		is.NoErr(err)

		// Create sample data
		lot := Lot{
			Date:         time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			AmountBTC:    money.New(100000000, "BTC"),
			CostBasisUSD: money.NewFromFloat(40000, money.USD),
			PricePerCoin: money.NewFromFloat(40000, money.USD),
			Remaining:    money.New(100000000, "BTC"),
		}

		// Calculate input file hashes using the in-memory filesystem
		inputFiles := []string{"test.csv"}
		inputHashes, err := generateInputHashesWithFs(inputFs, inputFiles)
		is.NoErr(err)

		// Save year-end state using the in-memory filesystem for input files
		err = cache.saveYearEndStateWithFs(inputFs, 2024, []Lot{lot}, []Sale{}, inputFiles)
		is.NoErr(err)

		// Load year-end state using the in-memory filesystem for input files
		loadedState, err := cache.loadYearEndStateWithFs(inputFs, 2024, inputFiles)
		is.NoErr(err)
		is.True(loadedState != nil)

		is.Equal(loadedState.Year, 2024)
		is.Equal(len(loadedState.Lots), 1)
		is.Equal(len(loadedState.Sales), 0)

		// Verify lot data
		loadedLot := loadedState.Lots[0]
		is.Equal(loadedLot.AmountBTC.Amount(), lot.AmountBTC.Amount())
		is.Equal(loadedLot.CostBasisUSD.Amount(), lot.CostBasisUSD.Amount())

		// Verify input hashes match
		is.Equal(len(loadedState.InputHashes), len(inputHashes))
	})
}

// Test cache directory handling and permissions
func TestCacheDirectoryHandling(t *testing.T) {
	is := is.New(t)

	t.Run("cache directory creation", func(t *testing.T) {
		cache := NewCache(afero.NewMemMapFs(), "test-dir-creation")

		// Put something to trigger directory creation
		err := cache.Put("test", []byte("data"))
		is.NoErr(err)

		// Verify we can read back the data (directory was created successfully)
		data, err := cache.Get("test")
		is.NoErr(err)
		is.Equal(string(data), "data")
	})

	t.Run("cache with error during initialization", func(t *testing.T) {
		// Create cache with initialization error
		cache := &Cache{
			fs:     nil,
			dirErr: os.ErrPermission,
		}

		// Operations should fail gracefully
		err := cache.Put("test", []byte("data"))
		is.True(err != nil)

		_, err = cache.Get("test")
		is.True(err != nil)
	})

	t.Run("in-memory cache operations", func(t *testing.T) {
		// Test comprehensive in-memory cache functionality
		memFs := afero.NewMemMapFs()
		cache := NewCache(memFs, "test-comprehensive-inmemory")

		// Test multiple operations
		err := cache.Put("file1", []byte("content1"))
		is.NoErr(err)

		err = cache.Put("subdir/file2", []byte("content2"))
		is.NoErr(err)

		// Read back data
		data1, err := cache.Get("file1")
		is.NoErr(err)
		is.Equal(string(data1), "content1")

		data2, err := cache.Get("subdir/file2")
		is.NoErr(err)
		is.Equal(string(data2), "content2")

		// Test clear
		err = cache.Clear()
		is.NoErr(err)

		// Verify files are gone
		data1, err1 := cache.Get("file1")
		data2, err2 := cache.Get("subdir/file2")

		// Debug: print what we got
		if err1 == nil {
			t.Errorf("Expected error for file1, but got data: %s", string(data1))
		}
		if err2 == nil {
			t.Errorf("Expected error for subdir/file2, but got data: %s", string(data2))
		}

		is.True(err1 != nil)
		is.True(err2 != nil)
	})
}

// Testify enhancement opportunities:
/*
The following functions could benefit from testify/suite:
- TestCacheYearEndState: Could use suite for cache setup/cleanup and shared test data
- TestCacheDirectoryHandling: Could use suite for directory cleanup

The following could benefit from testify/assert for better error messages:
- JSON marshaling/unmarshaling: assert.JSONEq for comparing JSON structures
- File system operations: assert.FileExists, assert.DirExists for file/directory checks
- Complex struct comparisons: assert.Equal with better diff output for Lot/Sale structs

The following could benefit from testify/require for fail-fast behavior:
- Cache directory creation: require.NoError to fail fast if setup fails
- File hash generation: require.NoError before comparing hash values
- Cache key generation: require.NotEmpty before using cache keys

The following could benefit from testify/mock:
- File system operations: Mock for testing permission errors and I/O failures
- Time provider: Mock for testing timestamp-dependent cache behavior
- Hash generation: Mock for testing hash collision scenarios

Custom test helpers that would be useful:
- CreateTempCacheWithData(data): Set up temporary cache with test data
- AssertCacheHit/Miss(cache, key): Verify cache behavior with better error messages
- AssertFileHashEqual(file1, file2): Compare file hashes with file content context
- MockFileSystem(): Create in-memory file system for isolated testing
*/
