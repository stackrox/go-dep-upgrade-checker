package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeImpact(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create some test Go files
	files := map[string]string{
		"central/deployment/manager.go": `package deployment
import "github.com/test/pkg/v2"
func main() {}`,
		"sensor/client.go": `package sensor
import "github.com/test/pkg/v2"
func main() {}`,
		"pkg/utils/helper.go": `package utils
import "github.com/test/pkg/v2"
func main() {}`,
		"other/file.go": `package other
import "github.com/different/pkg"
func main() {}`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}

	// Test impact analysis
	impact, err := AnalyzeImpact("github.com/test/pkg", tmpDir)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	if impact.FilesAffected != 3 {
		t.Errorf("Expected 3 files affected, got %d", impact.FilesAffected)
	}

	// Check components
	expectedComponents := map[string]bool{
		"central": true,
		"sensor":  true,
		"pkg":     true,
	}

	for _, comp := range impact.Components {
		if !expectedComponents[comp] {
			t.Errorf("Unexpected component: %s", comp)
		}
		delete(expectedComponents, comp)
	}

	if len(expectedComponents) > 0 {
		t.Errorf("Missing expected components: %v", expectedComponents)
	}

	// Test IsCritical
	if !impact.IsCritical() {
		t.Error("Expected impact to be critical (contains 'central' and 'sensor')")
	}
}

func TestAnalyzeImpactNonExistentPackage(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file that doesn't import our package
	testFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(testFile, []byte(`package test
import "fmt"
func main() { fmt.Println("hello") }`), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	impact, err := AnalyzeImpact("github.com/nonexistent/pkg", tmpDir)
	if err != nil {
		t.Fatalf("AnalyzeImpact failed: %v", err)
	}

	if impact.FilesAffected != 0 {
		t.Errorf("Expected 0 files affected, got %d", impact.FilesAffected)
	}

	if impact.IsCritical() {
		t.Error("Expected impact to not be critical")
	}
}

func TestAnalyzeAllImpacts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "central", "test.go")
	if err := os.MkdirAll(filepath.Dir(testFile), 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}
	if err := os.WriteFile(testFile, []byte(`package central
import "github.com/pkg1/lib/v2"
import "github.com/pkg2/lib/v3"`), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	candidates := []UpgradeCandidate{
		{
			Dependency: Dependency{
				BasePath: "github.com/pkg1/lib",
			},
		},
		{
			Dependency: Dependency{
				BasePath: "github.com/pkg2/lib",
			},
		},
	}

	err := AnalyzeAllImpacts(candidates, tmpDir)
	if err != nil {
		t.Fatalf("AnalyzeAllImpacts failed: %v", err)
	}

	// Both should have impact now
	if candidates[0].Impact == nil {
		t.Error("Expected first candidate to have impact")
	}
	if candidates[1].Impact == nil {
		t.Error("Expected second candidate to have impact")
	}

	if candidates[0].Impact.FilesAffected != 1 {
		t.Errorf("Expected 1 file affected for pkg1, got %d", candidates[0].Impact.FilesAffected)
	}
}
