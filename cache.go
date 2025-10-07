package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/spf13/afero"
)

type Cache struct {
	dir       string
	dirErr    error
	fs        afero.Fs
	mkdirOnce sync.Once
}

// NewCache creates a new cache using the provided filesystem restricted to the cache directory
func NewCache(fs afero.Fs, program string) *Cache {
	dir, dirErr := os.UserCacheDir()
	if dirErr != nil {
		return &Cache{dirErr: dirErr}
	}

	cacheDir := filepath.Join(dir, program)

	// Create the cache directory using the provided filesystem (not os.MkdirAll)
	// This allows tests to use in-memory filesystem while production uses OS filesystem
	if err := fs.MkdirAll(cacheDir, 0o700); err != nil {
		return &Cache{dirErr: err}
	}

	// Use BasePathFs to restrict operations to the cache directory only
	basePath := afero.NewBasePathFs(fs, cacheDir)

	return &Cache{
		fs:     basePath,
		dir:    cacheDir,
		dirErr: nil,
	}
}

// GetCacheInfo returns information about the cache for display purposes
func (c *Cache) GetCacheInfo() string {
	if c.dirErr != nil {
		return "Cache disabled due to error"
	}

	// For BasePathFs, try to get the underlying OS path for display
	switch fs := c.fs.(type) {
	case *afero.BasePathFs:
		if realPath, err := fs.RealPath("/"); err == nil {
			return realPath
		}
	}

	// For other filesystems (like MemMapFs in tests), return a generic message
	return "In-memory cache (testing)"
}

func (c *Cache) Get(name string) ([]byte, error) {
	if c.dirErr != nil {
		return nil, &os.PathError{Op: "getCache", Path: name, Err: os.ErrNotExist}
	}
	return afero.ReadFile(c.fs, name)
}

func (c *Cache) Put(name string, b []byte) error {
	if c.dirErr != nil {
		return &os.PathError{Op: "putCache", Path: name, Err: c.dirErr}
	}
	c.mkdirOnce.Do(func() {
		// Create any necessary parent directories
		if dir := filepath.Dir(name); dir != "." {
			if err := c.fs.MkdirAll(dir, 0o700); err != nil {
				log.Printf("can't create cache subdirectories: %v", err)
			}
		}
	})
	return afero.WriteFile(c.fs, name, b, 0o600)
}

func (c *Cache) Clear() error {
	if c.dirErr != nil {
		return &os.PathError{Op: "clearCache", Path: "/", Err: c.dirErr}
	}

	// SAFETY CHECK: Only proceed if we're dealing with BasePathFs (both production and test use this)
	basePath, ok := c.fs.(*afero.BasePathFs)
	if !ok {
		return fmt.Errorf("SAFETY CHECK FAILED: expected BasePathFs, got %T", c.fs)
	}

	// SAFETY CHECK: For BasePathFs, verify we're operating in the correct directory
	realPath, err := basePath.RealPath("/")
	if err != nil {
		return fmt.Errorf("cannot get real path for safety check: %v", err)
	}

	// Verify the real path matches our expected cache directory
	if realPath != c.dir {
		return fmt.Errorf("SAFETY CHECK FAILED: real path %s does not match expected cache dir %s", realPath, c.dir)
	}

	// Get entries in the cache directory
	entries, err := afero.ReadDir(c.fs, "/")
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist or is already empty
		}
		return err
	}

	// Remove each entry individually
	for _, entry := range entries {
		entryPath := "/" + entry.Name()
		if err := basePath.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("failed to remove %s: %v", entryPath, err)
		}
	}

	return nil
}

// YearEndState represents the cached state of lots at the end of a tax year
type YearEndState struct {
	Year        int       `json:"year"`
	Lots        []Lot     `json:"lots"`
	Sales       []Sale    `json:"sales"`
	InputHashes []string  `json:"input_hashes"`
	CreatedAt   time.Time `json:"created_at"`
}

// generateFileHash creates a SHA256 hash of file contents using the OS filesystem
func generateFileHash(filePath string) (string, error) {
	return generateFileHashWithFs(afero.NewOsFs(), filePath)
}

// generateFileHashWithFs creates a SHA256 hash using a specific filesystem
func generateFileHashWithFs(fs afero.Fs, filePath string) (string, error) {
	file, err := fs.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// generateInputHashes creates hashes for all input files using the OS filesystem
func generateInputHashes(filePaths []string) ([]string, error) {
	return generateInputHashesWithFs(afero.NewOsFs(), filePaths)
}

// generateInputHashesWithFs creates hashes for all input files using a specific filesystem
func generateInputHashesWithFs(fs afero.Fs, filePaths []string) ([]string, error) {
	var hashes []string

	// Sort file paths for consistent ordering
	sortedPaths := make([]string, len(filePaths))
	copy(sortedPaths, filePaths)
	sort.Strings(sortedPaths)

	for _, filePath := range sortedPaths {
		hash, err := generateFileHashWithFs(fs, filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to hash file %s: %v", filePath, err)
		}
		hashes = append(hashes, fmt.Sprintf("%s:%s", filepath.Base(filePath), hash))
	}

	return hashes, nil
}

// generateCacheKey creates a cache key for a given year and input files
func generateCacheKey(year int, inputFiles []string) (string, error) {
	return generateCacheKeyWithFs(afero.NewOsFs(), year, inputFiles)
}

// generateCacheKeyWithFs creates a cache key using a specific filesystem for input files
func generateCacheKeyWithFs(inputFs afero.Fs, year int, inputFiles []string) (string, error) {
	hashes, err := generateInputHashesWithFs(inputFs, inputFiles)
	if err != nil {
		return "", err
	}

	// Create a deterministic key from year and file hashes
	keyData := fmt.Sprintf("year_%d_files_%v", year, hashes)
	hash := sha256.Sum256([]byte(keyData))
	return fmt.Sprintf("year_%d_%s.json", year, hex.EncodeToString(hash[:8])), nil
}

// saveYearEndState saves the lot state and sales for a specific year to cache
func (c *Cache) saveYearEndState(year int, lots []Lot, sales []Sale, inputFiles []string) error {
	return c.saveYearEndStateWithFs(afero.NewOsFs(), year, lots, sales, inputFiles)
}

// saveYearEndStateWithFs saves the lot state and sales for a specific year to cache using a specific filesystem for input files
func (c *Cache) saveYearEndStateWithFs(inputFs afero.Fs, year int, lots []Lot, sales []Sale, inputFiles []string) error {
	hashes, err := generateInputHashesWithFs(inputFs, inputFiles)
	if err != nil {
		return fmt.Errorf("failed to generate input hashes: %v", err)
	}

	state := YearEndState{
		Year:        year,
		Lots:        lots,
		Sales:       sales,
		InputHashes: hashes,
		CreatedAt:   time.Now(),
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal year-end state: %v", err)
	}

	cacheKey, err := generateCacheKeyWithFs(inputFs, year, inputFiles)
	if err != nil {
		return fmt.Errorf("failed to generate cache key: %v", err)
	}

	return c.Put(cacheKey, data)
}

// loadYearEndState loads cached lot state for a specific year
func (c *Cache) loadYearEndState(year int, inputFiles []string) (*YearEndState, error) {
	return c.loadYearEndStateWithFs(afero.NewOsFs(), year, inputFiles)
}

// loadYearEndStateWithFs loads cached lot state for a specific year using a specific filesystem for input files
func (c *Cache) loadYearEndStateWithFs(inputFs afero.Fs, year int, inputFiles []string) (*YearEndState, error) {
	cacheKey, err := generateCacheKeyWithFs(inputFs, year, inputFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cache key: %v", err)
	}

	data, err := c.Get(cacheKey)
	if err != nil {
		return nil, err // Cache miss
	}

	var state YearEndState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached state: %v", err)
	}

	// Validate cache against current input files
	currentHashes, err := generateInputHashesWithFs(inputFs, inputFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to validate cache: %v", err)
	}

	if !slices.Equal(state.InputHashes, currentHashes) {
		return nil, fmt.Errorf("cache invalidated: input files have changed")
	}

	return &state, nil
}

// invalidateCache removes cached data for a specific year
func (c *Cache) invalidateCache(year int, inputFiles []string) error {
	cacheKey, err := generateCacheKey(year, inputFiles)
	if err != nil {
		return fmt.Errorf("failed to generate cache key: %v", err)
	}

	if err := c.fs.Remove(cacheKey); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %v", err)
	}

	return nil
}
