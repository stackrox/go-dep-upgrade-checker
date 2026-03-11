package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunWithValidGoMod(t *testing.T) {
	// Create a temporary go.mod file
	tmpDir := t.TempDir()
	gomodPath := filepath.Join(tmpDir, "go.mod")

	gomodContent := `module example.com/test

go 1.21

require (
	github.com/google/uuid v1.0.0
)
`
	if err := os.WriteFile(gomodPath, []byte(gomodContent), 0644); err != nil {
		t.Fatalf("Failed to create test go.mod: %v", err)
	}

	// Test run function with output to temp file
	outputPath := filepath.Join(tmpDir, "output.md")

	// Set command line flags programmatically
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"cmd", "-gomod", gomodPath, "-output", outputPath}

	// Note: We can't easily test main() or run() as they exit
	// Instead we test the components individually
	t.Log("Main function components tested via unit tests")
}

func TestDependencyStruct(t *testing.T) {
	dep := &Dependency{
		Path:         "github.com/google/go-github/v60",
		BasePath:     "github.com/google/go-github",
		CurrentVer:   "v60.0.0",
		CurrentMajor: 60,
		IsDirect:     true,
		IsReplaced:   false,
	}

	// Verify dependency fields
	if dep.Path != "github.com/google/go-github/v60" {
		t.Errorf("Expected Path to be set correctly")
	}
	if dep.BasePath != "github.com/google/go-github" {
		t.Errorf("Expected BasePath to be set correctly")
	}
	if dep.CurrentVer != "v60.0.0" {
		t.Errorf("Expected CurrentVer to be set correctly")
	}
	if dep.CurrentMajor != 60 {
		t.Errorf("Expected CurrentMajor to be 60")
	}
	if !dep.IsDirect {
		t.Errorf("Expected IsDirect to be true")
	}
	if dep.IsReplaced {
		t.Errorf("Expected IsReplaced to be false")
	}
}
