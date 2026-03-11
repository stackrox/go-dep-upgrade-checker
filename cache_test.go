package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewCache(t *testing.T) {
	// Test with cache disabled
	cache, err := NewCache(false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cache.enabled {
		t.Error("Cache should be disabled")
	}

	// Test with cache enabled
	cache, err = NewCache(true)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !cache.enabled {
		t.Error("Cache should be enabled")
	}
	if cache.dir == "" {
		t.Error("Cache directory should be set")
	}
}

func TestCacheGetSet(t *testing.T) {
	// Create cache in temp directory
	tmpDir := t.TempDir()
	cache := &Cache{
		dir:     tmpDir,
		enabled: true,
	}

	dep := Dependency{
		Path:         "github.com/example/pkg/v2",
		BasePath:     "github.com/example/pkg",
		CurrentVer:   "v2.0.0",
		CurrentMajor: 2,
		IsDirect:     true,
	}

	versions := []AvailableVersion{
		{
			MajorVer:    "v3",
			FullVersion: "v3.0.0",
			Major:       3,
			ReleasedAt:  timePtr(time.Now()),
		},
	}

	changelog := &ChangelogInfo{
		Found:           true,
		Source:          "releases",
		URL:             "https://github.com/example/pkg/releases",
		Content:         "Some content",
		BreakingChanges: []string{"Breaking change 1"},
	}

	impact := &ImpactAnalysis{
		FilesAffected: 5,
		Components:    []string{"component1"},
	}

	// Test Set
	err := cache.Set(dep, versions, changelog, impact, false)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Test Get
	entry, found, err := cache.Get(dep)
	if err != nil {
		t.Fatalf("Failed to get cache: %v", err)
	}
	if !found {
		t.Fatal("Cache entry should be found")
	}

	// Verify cached data
	if entry.Dependency.Path != dep.Path {
		t.Errorf("Expected path %s, got %s", dep.Path, entry.Dependency.Path)
	}
	if len(entry.Versions) != 1 {
		t.Errorf("Expected 1 version, got %d", len(entry.Versions))
	}
	if entry.ChangelogInfo.Found != true {
		t.Error("Changelog should be found")
	}
	if entry.ImpactAnalysis.FilesAffected != 5 {
		t.Errorf("Expected 5 affected files, got %d", entry.ImpactAnalysis.FilesAffected)
	}

	// Test cache miss
	otherDep := Dependency{
		Path:         "github.com/other/pkg",
		BasePath:     "github.com/other/pkg",
		CurrentVer:   "v1.0.0",
		CurrentMajor: 1,
	}
	_, found, err = cache.Get(otherDep)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if found {
		t.Error("Should not find cache entry for different dependency")
	}
}

func TestCacheExpiration(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &Cache{
		dir:     tmpDir,
		enabled: true,
	}

	dep := Dependency{
		Path:         "github.com/example/pkg",
		BasePath:     "github.com/example/pkg",
		CurrentVer:   "v1.0.0",
		CurrentMajor: 1,
	}

	// Set cache entry
	err := cache.Set(dep, []AvailableVersion{}, nil, nil, false)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	// Modify the cache file to have an old timestamp
	cacheFile := cache.cacheFilePath(dep)
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}

	// Create an entry with old timestamp
	oldTime := time.Now().Add(-25 * time.Hour) // Older than TTL
	entry := CacheEntry{
		Version:   cacheVersion,
		Timestamp: oldTime,
		Dependency: dep,
		Versions:  []AvailableVersion{},
	}

	// Write expired entry
	data, _ = marshalCacheEntry(entry)
	err = os.WriteFile(cacheFile, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write cache file: %v", err)
	}

	// Try to get expired entry
	_, found, err := cache.Get(dep)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if found {
		t.Error("Expired cache entry should not be found")
	}

	// Verify cache file was deleted
	if _, err := os.Stat(cacheFile); !os.IsNotExist(err) {
		t.Error("Expired cache file should be deleted")
	}
}

func TestCacheClear(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &Cache{
		dir:     tmpDir,
		enabled: true,
	}

	// Create multiple cache entries
	for i := 0; i < 3; i++ {
		dep := Dependency{
			Path:         filepath.Join("github.com/example", string(rune('a'+i))),
			BasePath:     filepath.Join("github.com/example", string(rune('a'+i))),
			CurrentVer:   "v1.0.0",
			CurrentMajor: 1,
		}
		err := cache.Set(dep, []AvailableVersion{}, nil, nil, false)
		if err != nil {
			t.Fatalf("Failed to set cache: %v", err)
		}
	}

	// Verify files exist
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read cache dir: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Expected 3 cache files, got %d", len(entries))
	}

	// Clear cache
	err = cache.Clear()
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Verify all files removed
	entries, err = os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read cache dir: %v", err)
	}
	jsonCount := 0
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			jsonCount++
		}
	}
	if jsonCount != 0 {
		t.Errorf("Expected 0 cache files after clear, got %d", jsonCount)
	}
}

func TestCacheStats(t *testing.T) {
	tmpDir := t.TempDir()
	cache := &Cache{
		dir:     tmpDir,
		enabled: true,
	}

	// Empty cache
	stats, err := cache.Stats()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if stats.TotalFiles != 0 {
		t.Errorf("Expected 0 files in empty cache, got %d", stats.TotalFiles)
	}

	// Add some entries
	dep := Dependency{
		Path:         "github.com/example/pkg",
		BasePath:     "github.com/example/pkg",
		CurrentVer:   "v1.0.0",
		CurrentMajor: 1,
	}
	err = cache.Set(dep, []AvailableVersion{}, nil, nil, false)
	if err != nil {
		t.Fatalf("Failed to set cache: %v", err)
	}

	stats, err = cache.Stats()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if stats.TotalFiles != 1 {
		t.Errorf("Expected 1 file, got %d", stats.TotalFiles)
	}
	if stats.TotalSize == 0 {
		t.Error("Cache size should be > 0")
	}
	if stats.Dir != tmpDir {
		t.Errorf("Expected dir %s, got %s", tmpDir, stats.Dir)
	}
}

func TestCacheDisabled(t *testing.T) {
	cache := &Cache{enabled: false}

	dep := Dependency{
		Path:         "github.com/example/pkg",
		BasePath:     "github.com/example/pkg",
		CurrentVer:   "v1.0.0",
		CurrentMajor: 1,
	}

	// Set should do nothing
	err := cache.Set(dep, []AvailableVersion{}, nil, nil, false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Get should return not found
	_, found, err := cache.Get(dep)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if found {
		t.Error("Disabled cache should not find entries")
	}

	// Clear should do nothing
	err = cache.Clear()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Stats should return nil
	stats, err := cache.Stats()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if stats != nil {
		t.Error("Disabled cache should return nil stats")
	}
}

// Helper functions

func timePtr(t time.Time) *time.Time {
	return &t
}

func marshalCacheEntry(entry CacheEntry) ([]byte, error) {
	return json.MarshalIndent(entry, "", "  ")
}
