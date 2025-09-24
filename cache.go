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
	"sort"
	"sync"
	"time"
)

type Cache struct {
	mkdirOnce sync.Once
	dirErr    error
	dir       string
}

func NewCache(program string) *Cache {
	dir, dirErr := os.UserCacheDir()
	cache := Cache{dirErr: dirErr}
	if dirErr == nil {
		dir = filepath.Join(dir, program)
	}
	cache.dir = dir

	return &cache
}

func (c *Cache) Get(name string) ([]byte, error) {
	if c.dirErr != nil {
		return nil, &os.PathError{Op: "getCache", Path: name, Err: os.ErrNotExist}
	}
	return os.ReadFile(filepath.Join(c.dir, name))
}

func (c *Cache) Put(name string, b []byte) error {
	if c.dirErr != nil {
		return &os.PathError{Op: "putCache", Path: name, Err: c.dirErr}
	}
	c.mkdirOnce.Do(func() {
		if err := os.MkdirAll(c.dir, 0o700); err != nil {
			log.Printf("can't create user cache dir: %v", err)
		}
	})
	return os.WriteFile(filepath.Join(c.dir, name), b, 0o600)
}

func (c *Cache) Clear() error {
	if c.dirErr != nil {
		return &os.PathError{Op: "clearCache", Path: c.dir, Err: c.dirErr}
	}
	return os.RemoveAll(c.dir)
}

// YearEndState represents the cached state of lots at the end of a tax year
type YearEndState struct {
	Year        int       `json:"year"`
	Lots        []Lot     `json:"lots"`
	Sales       []Sale    `json:"sales"`
	InputHashes []string  `json:"input_hashes"`
	CreatedAt   time.Time `json:"created_at"`
}

// generateFileHash creates a SHA256 hash of file contents
func generateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
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

// generateInputHashes creates hashes for all input files
func generateInputHashes(filePaths []string) ([]string, error) {
	var hashes []string

	// Sort file paths for consistent ordering
	sortedPaths := make([]string, len(filePaths))
	copy(sortedPaths, filePaths)
	sort.Strings(sortedPaths)

	for _, filePath := range sortedPaths {
		hash, err := generateFileHash(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to hash file %s: %v", filePath, err)
		}
		hashes = append(hashes, fmt.Sprintf("%s:%s", filepath.Base(filePath), hash))
	}

	return hashes, nil
}

// generateCacheKey creates a cache key for a given year and input files
func generateCacheKey(year int, inputFiles []string) (string, error) {
	hashes, err := generateInputHashes(inputFiles)
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
	hashes, err := generateInputHashes(inputFiles)
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

	cacheKey, err := generateCacheKey(year, inputFiles)
	if err != nil {
		return fmt.Errorf("failed to generate cache key: %v", err)
	}

	return c.Put(cacheKey, data)
}

// loadYearEndState loads cached lot state for a specific year
func (c *Cache) loadYearEndState(year int, inputFiles []string) (*YearEndState, error) {
	cacheKey, err := generateCacheKey(year, inputFiles)
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
	currentHashes, err := generateInputHashes(inputFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to validate cache: %v", err)
	}

	if !stringSlicesEqual(state.InputHashes, currentHashes) {
		return nil, fmt.Errorf("cache invalidated: input files have changed")
	}

	return &state, nil
}

// stringSlicesEqual compares two string slices for equality
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

// invalidateCache removes cached data for a specific year
func (c *Cache) invalidateCache(year int, inputFiles []string) error {
	cacheKey, err := generateCacheKey(year, inputFiles)
	if err != nil {
		return fmt.Errorf("failed to generate cache key: %v", err)
	}

	cachePath := filepath.Join(c.dir, cacheKey)
	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %v", err)
	}

	return nil
}
