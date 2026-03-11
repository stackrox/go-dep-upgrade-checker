package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseGoMod(t *testing.T) {
	// Create a temporary go.mod file
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")

	content := `module example.com/test

go 1.25

require (
	github.com/foo/bar/v3 v3.2.1
	github.com/baz/qux/v2 v2.1.0 // indirect
	github.com/no/suffix v1.5.0
	github.com/zero/version v0.3.2
)

replace github.com/foo/bar/v3 => ../local/bar
`

	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test go.mod: %v", err)
	}

	deps, err := ParseGoMod(goModPath)
	if err != nil {
		t.Fatalf("ParseGoMod failed: %v", err)
	}

	if len(deps) != 4 {
		t.Errorf("Expected 4 dependencies (including no-suffix packages), got %d", len(deps))
	}

	// Check versioned dependency (with suffix)
	var barDep *Dependency
	for i := range deps {
		if deps[i].Path == "github.com/foo/bar/v3" {
			barDep = &deps[i]
			break
		}
	}
	if barDep == nil {
		t.Fatal("Expected to find github.com/foo/bar/v3")
	}
	if barDep.BasePath != "github.com/foo/bar" {
		t.Errorf("Expected basePath 'github.com/foo/bar', got '%s'", barDep.BasePath)
	}
	if barDep.CurrentVer != "v3" {
		t.Errorf("Expected currentVer 'v3', got '%s'", barDep.CurrentVer)
	}
	if barDep.CurrentMajor != 3 {
		t.Errorf("Expected currentMajor 3, got %d", barDep.CurrentMajor)
	}
	if !barDep.IsDirect {
		t.Error("Expected bar to be direct")
	}
	if !barDep.IsReplaced {
		t.Error("Expected bar to be replaced")
	}

	// Check no-suffix dependency (v1)
	var noSuffixDep *Dependency
	for i := range deps {
		if deps[i].Path == "github.com/no/suffix" {
			noSuffixDep = &deps[i]
			break
		}
	}
	if noSuffixDep == nil {
		t.Fatal("Expected to find github.com/no/suffix")
	}
	if noSuffixDep.BasePath != "github.com/no/suffix" {
		t.Errorf("Expected basePath 'github.com/no/suffix', got '%s'", noSuffixDep.BasePath)
	}
	if noSuffixDep.CurrentVer != "v1" {
		t.Errorf("Expected currentVer 'v1' (inferred), got '%s'", noSuffixDep.CurrentVer)
	}
	if noSuffixDep.CurrentMajor != 1 {
		t.Errorf("Expected currentMajor 1 (inferred from v1.5.0), got %d", noSuffixDep.CurrentMajor)
	}

	// Check v0 dependency
	var zeroDep *Dependency
	for i := range deps {
		if deps[i].Path == "github.com/zero/version" {
			zeroDep = &deps[i]
			break
		}
	}
	if zeroDep == nil {
		t.Fatal("Expected to find github.com/zero/version")
	}
	if zeroDep.CurrentMajor != 0 {
		t.Errorf("Expected currentMajor 0 (inferred from v0.3.2), got %d", zeroDep.CurrentMajor)
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"3", 3},
		{"10", 10},
		{"0", 0},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		result := parseInt(tt.input)
		if result != tt.expected {
			t.Errorf("parseInt(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestExtractMajorFromVersion(t *testing.T) {
	tests := []struct {
		version  string
		expected int
	}{
		{"v1.8.0", 1},
		{"v2.3.1", 2},
		{"v0.3.2", 0},
		{"v10.5.3", 10},
		{"1.0.0", 1}, // Without 'v' prefix
		{"v3", 3},    // Just major
		{"", 0},      // Empty
		{"invalid", 0},
	}

	for _, tt := range tests {
		result := extractMajorFromVersion(tt.version)
		if result != tt.expected {
			t.Errorf("extractMajorFromVersion(%q) = %d, want %d", tt.version, result, tt.expected)
		}
	}
}

func TestFilterDependencies(t *testing.T) {
	deps := []Dependency{
		{Path: "github.com/foo/bar/v3", IsDirect: true, IsReplaced: false},
		{Path: "github.com/baz/qux/v2", IsDirect: false, IsReplaced: false},
		{Path: "github.com/replaced/pkg/v4", IsDirect: true, IsReplaced: true},
	}

	// Filter out indirect
	filtered := FilterDependencies(deps, false, false)
	if len(filtered) != 1 {
		t.Errorf("Expected 1 dependency when filtering indirect, got %d", len(filtered))
	}

	// Include indirect, exclude replaced
	filtered = FilterDependencies(deps, true, false)
	if len(filtered) != 2 {
		t.Errorf("Expected 2 dependencies when including indirect, got %d", len(filtered))
	}

	// Include both
	filtered = FilterDependencies(deps, true, true)
	if len(filtered) != 3 {
		t.Errorf("Expected 3 dependencies when including all, got %d", len(filtered))
	}
}

func TestFindVersionConflicts(t *testing.T) {
	deps := []Dependency{
		{Path: "github.com/foo/bar/v3", BasePath: "github.com/foo/bar", IsDirect: true},
		{Path: "github.com/foo/bar/v2", BasePath: "github.com/foo/bar", IsDirect: false},
		{Path: "github.com/baz/qux/v2", BasePath: "github.com/baz/qux", IsDirect: true},
	}

	conflicts := FindVersionConflicts(deps)
	if len(conflicts) != 1 {
		t.Fatalf("Expected 1 conflict, got %d", len(conflicts))
	}

	if conflicts[0].BasePath != "github.com/foo/bar" {
		t.Errorf("Expected conflict for 'github.com/foo/bar', got '%s'", conflicts[0].BasePath)
	}

	if len(conflicts[0].DirectDeps) != 1 {
		t.Errorf("Expected 1 direct dep in conflict, got %d", len(conflicts[0].DirectDeps))
	}

	if len(conflicts[0].IndirectDeps) != 1 {
		t.Errorf("Expected 1 indirect dep in conflict, got %d", len(conflicts[0].IndirectDeps))
	}
}
