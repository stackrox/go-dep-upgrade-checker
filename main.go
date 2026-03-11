package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

const (
	defaultFilePermissions = 0644
)

func main() {
	var (
		goModPath       string
		outputPath      string
		packageFilter   string
		includeIndirect bool
		includeReplaced bool
		noCache         bool
		clearCache      bool
		cacheStats      bool
	)

	flag.StringVar(&goModPath, "gomod", "go.mod", "Path to go.mod file")
	flag.StringVar(&outputPath, "output", "", "Output file path (default: stdout)")
	flag.StringVar(&packageFilter, "package", "", "Filter by specific package path")
	flag.BoolVar(&includeIndirect, "include-indirect", false, "Include indirect dependencies")
	flag.BoolVar(&includeReplaced, "include-replaced", false, "Include replaced dependencies")
	flag.BoolVar(&noCache, "no-cache", false, "Disable caching")
	flag.BoolVar(&clearCache, "clear-cache", false, "Clear cache and exit")
	flag.BoolVar(&cacheStats, "cache-stats", false, "Show cache statistics and exit")
	flag.Parse()

	// Handle cache-only commands
	if clearCache {
		cache, err := NewCache(true)
		if err != nil {
			log.Fatalf("Error creating cache: %v\n", err)
		}
		if err := cache.Clear(); err != nil {
			log.Fatalf("Error clearing cache: %v\n", err)
		}
		log.Println("Cache cleared successfully")
		return
	}

	if cacheStats {
		cache, err := NewCache(true)
		if err != nil {
			log.Fatalf("Error creating cache: %v\n", err)
		}
		stats, err := cache.Stats()
		if err != nil {
			log.Fatalf("Error getting cache stats: %v\n", err)
		}
		if stats == nil || stats.TotalFiles == 0 {
			log.Println("Cache is empty")
		} else {
			log.Printf("Cache directory: %s", stats.Dir)
			log.Printf("Total entries: %d", stats.TotalFiles)
			log.Printf("Total size: %.2f KB", float64(stats.TotalSize)/1024)
			if !stats.OldestFile.IsZero() {
				log.Printf("Oldest entry: %s", stats.OldestFile.Format("2006-01-02 15:04:05"))
			}
			if !stats.NewestFile.IsZero() {
				log.Printf("Newest entry: %s", stats.NewestFile.Format("2006-01-02 15:04:05"))
			}
		}
		return
	}

	if err := run(goModPath, outputPath, packageFilter, includeIndirect, includeReplaced, !noCache); err != nil {
		log.Fatalf("Error: %v\n", err)
	}
}

// filterByPackage returns dependencies matching the specified package filter
func filterByPackage(deps []Dependency, packageFilter string) []Dependency {
	var filtered []Dependency
	for _, dep := range deps {
		if dep.Path == packageFilter || dep.BasePath == packageFilter {
			filtered = append(filtered, dep)
		}
	}
	return filtered
}

func run(goModPath, outputPath, packageFilter string, includeIndirect, includeReplaced, useCache bool) error {
	// Make path absolute if relative
	if !filepath.IsAbs(goModPath) {
		absPath, err := filepath.Abs(goModPath)
		if err != nil {
			return fmt.Errorf("failed to resolve go.mod path: %w", err)
		}
		goModPath = absPath
	}

	// Check if go.mod exists
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found at %s", goModPath)
	}

	log.Printf("Parsing %s...", goModPath)
	deps, err := ParseGoMod(goModPath)
	if err != nil {
		return fmt.Errorf("failed to parse go.mod: %w", err)
	}
	log.Printf("Found %d versioned dependencies", len(deps))

	// Filter dependencies based on flags
	deps = FilterDependencies(deps, includeIndirect, includeReplaced)
	log.Printf("After filtering: %d dependencies to check", len(deps))

	// Apply package filter if specified
	if packageFilter != "" {
		deps = filterByPackage(deps, packageFilter)
		log.Printf("After package filter: %d dependencies", len(deps))
	}

	// Initialize cache
	cache, err := NewCache(useCache)
	if err != nil {
		log.Printf("Warning: failed to initialize cache: %v", err)
		cache = &Cache{enabled: false}
	}
	if cache.enabled {
		log.Println("Cache enabled (24h TTL)")
	}

	// Check for upgrades with caching
	log.Println("Checking for available upgrades...")
	candidates, err := CheckAllUpgradesWithCache(deps, cache, true) // Always show progress bar
	if err != nil {
		return fmt.Errorf("failed to check upgrades: %w", err)
	}
	log.Printf("Found %d upgrade candidates", len(candidates))

	// Check for archived dependencies (even those without upgrades)
	log.Println("Checking for archived dependencies...")
	archivedDeps := CheckArchivedDependencies(deps, candidates)
	if len(archivedDeps) > 0 {
		log.Printf("Found %d archived dependencies without upgrades", len(archivedDeps))
	}

	// Analyze impact on codebase (not cached since it's codebase-specific)
	if len(candidates) > 0 {
		log.Println("Analyzing codebase impact...")
		repoRoot := filepath.Dir(goModPath)
		if err := AnalyzeAllImpacts(candidates, repoRoot); err != nil {
			log.Printf("Warning: impact analysis failed: %v", err)
		}
	}

	// Generate report
	log.Println("Generating report...")
	report := GenerateBasicReport(candidates, archivedDeps)

	// Output report
	if outputPath != "" {
		if err := os.WriteFile(outputPath, []byte(report), defaultFilePermissions); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		log.Printf("Report written to %s", outputPath)
	} else {
		fmt.Println(report)
	}

	return nil
}
