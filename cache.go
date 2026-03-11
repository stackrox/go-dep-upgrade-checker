package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	cacheTTL     = 24 * time.Hour // Cache entries expire after 24 hours
	cacheVersion = "v2"            // Cache format version (increment if schema changes)
)

// Cache provides persistent storage for dependency upgrade information
type Cache struct {
	dir     string
	enabled bool
}

// CacheEntry represents a cached upgrade check result
type CacheEntry struct {
	Version        string             `json:"version"`         // Cache format version
	Timestamp      time.Time          `json:"timestamp"`       // When this was cached
	Dependency     Dependency         `json:"dependency"`      // Original dependency info
	Versions       []AvailableVersion `json:"versions"`        // Available upgrade versions
	ChangelogInfo  *ChangelogInfo     `json:"changelog_info"`  // Changelog data
	ImpactAnalysis *ImpactAnalysis    `json:"impact_analysis"` // Impact analysis data
	Archived       bool               `json:"archived"`        // Whether the repository is archived
}

// NewCache creates a new cache instance
func NewCache(enabled bool) (*Cache, error) {
	if !enabled {
		return &Cache{enabled: false}, nil
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user cache directory: %w", err)
	}

	// Create subdirectory for our tool
	appCacheDir := filepath.Join(cacheDir, "go-dep-upgrade-checker")
	if err := os.MkdirAll(appCacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &Cache{
		dir:     appCacheDir,
		enabled: true,
	}, nil
}

// Get retrieves a cached entry for the given dependency
// Returns (entry, found, error)
func (c *Cache) Get(dep Dependency) (*CacheEntry, bool, error) {
	if !c.enabled {
		return nil, false, nil
	}

	cacheFile := c.cacheFilePath(dep)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil // Cache miss
		}
		return nil, false, fmt.Errorf("failed to read cache file: %w", err)
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		// Invalid cache file, treat as miss
		os.Remove(cacheFile)
		return nil, false, nil
	}

	// Check cache version
	if entry.Version != cacheVersion {
		os.Remove(cacheFile)
		return nil, false, nil
	}

	// Check if expired
	if time.Since(entry.Timestamp) > cacheTTL {
		os.Remove(cacheFile)
		return nil, false, nil
	}

	return &entry, true, nil
}

// Set stores a cache entry for the given dependency
func (c *Cache) Set(dep Dependency, versions []AvailableVersion, changelog *ChangelogInfo, impact *ImpactAnalysis, archived bool) error {
	if !c.enabled {
		return nil
	}

	entry := CacheEntry{
		Version:        cacheVersion,
		Timestamp:      time.Now(),
		Dependency:     dep,
		Versions:       versions,
		ChangelogInfo:  changelog,
		ImpactAnalysis: impact,
		Archived:       archived,
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	cacheFile := c.cacheFilePath(dep)
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Clear removes all cached entries
func (c *Cache) Clear() error {
	if !c.enabled {
		return nil
	}

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			if err := os.Remove(filepath.Join(c.dir, entry.Name())); err != nil {
				return fmt.Errorf("failed to remove cache file: %w", err)
			}
		}
	}

	return nil
}

// CacheStats returns statistics about the cache
type CacheStats struct {
	Dir        string
	TotalFiles int
	TotalSize  int64
	OldestFile time.Time
	NewestFile time.Time
}

// Stats returns cache statistics
func (c *Cache) Stats() (*CacheStats, error) {
	if !c.enabled {
		return nil, nil
	}

	stats := &CacheStats{
		Dir: c.dir,
	}

	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		stats.TotalFiles++
		stats.TotalSize += info.Size()

		modTime := info.ModTime()
		if stats.OldestFile.IsZero() || modTime.Before(stats.OldestFile) {
			stats.OldestFile = modTime
		}
		if stats.NewestFile.IsZero() || modTime.After(stats.NewestFile) {
			stats.NewestFile = modTime
		}
	}

	return stats, nil
}

// cacheFilePath generates a cache file path for a dependency
func (c *Cache) cacheFilePath(dep Dependency) string {
	// Use hash of the dependency path and current version to create unique filename
	key := fmt.Sprintf("%s@%s", dep.Path, dep.CurrentVer)
	hash := sha256.Sum256([]byte(key))
	filename := fmt.Sprintf("%x.json", hash[:16])
	return filepath.Join(c.dir, filename)
}
