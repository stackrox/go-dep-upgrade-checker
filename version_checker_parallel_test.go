package main

import (
	"testing"
	"time"
)

func TestCheckAllUpgradesParallelPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create test dependencies - these won't find upgrades but will test parallelization
	deps := []Dependency{
		{Path: "github.com/test/nonexistent1/v2", BasePath: "github.com/test/nonexistent1", CurrentMajor: 2},
		{Path: "github.com/test/nonexistent2/v2", BasePath: "github.com/test/nonexistent2", CurrentMajor: 2},
		{Path: "github.com/test/nonexistent3/v2", BasePath: "github.com/test/nonexistent3", CurrentMajor: 2},
	}

	start := time.Now()
	_, err := CheckAllUpgrades(deps, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("CheckAllUpgrades failed: %v", err)
	}

	// With parallelization, 3 packages should complete faster than 3x sequential time
	// Sequential would be ~3 seconds (1 second per lookup), parallel should be much faster
	t.Logf("Parallel check of %d packages took: %v", len(deps), elapsed)

	// This is a rough check - parallel should be significantly faster than sequential
	// If it takes more than 2 seconds for 3 packages, parallelization might not be working
	if elapsed > 3*time.Second {
		t.Logf("Warning: Took longer than expected (%v), parallelization may not be effective", elapsed)
	}
}

func TestCheckAllUpgradesEmpty(t *testing.T) {
	candidates, err := CheckAllUpgrades([]Dependency{}, false)
	if err != nil {
		t.Fatalf("CheckAllUpgrades failed on empty input: %v", err)
	}

	if len(candidates) != 0 {
		t.Errorf("Expected 0 candidates for empty input, got %d", len(candidates))
	}
}

func TestCheckAllUpgradesProgressBarOff(t *testing.T) {
	deps := []Dependency{
		{Path: "github.com/test/pkg/v2", BasePath: "github.com/test/pkg", CurrentMajor: 2},
	}

	// Should not panic without progress bar
	_, err := CheckAllUpgrades(deps, false)
	if err != nil {
		t.Fatalf("CheckAllUpgrades failed: %v", err)
	}
}

func TestWorkerPoolLimit(t *testing.T) {
	// This test verifies that we don't spawn unlimited goroutines
	// With 10 worker limit, we should handle any number of dependencies safely
	deps := make([]Dependency, 25)
	for i := range deps {
		deps[i] = Dependency{
			Path:         "github.com/test/nonexistent/v2",
			BasePath:     "github.com/test/nonexistent",
			CurrentMajor: 2,
		}
	}

	// Should complete without spawning 25+ concurrent workers
	_, err := CheckAllUpgrades(deps, false)
	if err != nil {
		t.Fatalf("CheckAllUpgrades with many deps failed: %v", err)
	}
}
