package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ImpactAnalysis contains the results of codebase impact analysis
type ImpactAnalysis struct {
	FilesAffected int
	Components    []string // e.g., central, sensor, roxctl
	FileList      []string // List of affected files
}

// AnalyzeImpact analyzes the codebase to determine impact of upgrading a dependency
func AnalyzeImpact(basePath string, repoRoot string) (*ImpactAnalysis, error) {
	fileSet := make(map[string]bool)
	componentSet := make(map[string]bool)

	// Walk the repository tree looking for Go files that import this package
	err := filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Read file and check if it contains the base path
		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		// Simple check if file contains the base path
		if strings.Contains(string(content), basePath) {
			fileSet[path] = true

			// Extract component from file path
			component := extractComponent(path, repoRoot)
			if component != "" {
				componentSet[component] = true
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	// Convert sets to slices
	var files []string
	for file := range fileSet {
		files = append(files, file)
	}

	var components []string
	for component := range componentSet {
		components = append(components, component)
	}

	return &ImpactAnalysis{
		FilesAffected: len(files),
		Components:    components,
		FileList:      files,
	}, nil
}

// extractComponent extracts the top-level component name from a file path
// e.g., /path/to/repo/central/foo/bar.go -> "central"
// e.g., /path/to/repo/roxctl/cmd/main.go -> "roxctl"
func extractComponent(filePath, repoRoot string) string {
	// Make path relative to repo root
	relPath, err := filepath.Rel(repoRoot, filePath)
	if err != nil {
		relPath = filePath
	}

	// Split path and get first component
	parts := strings.Split(relPath, string(filepath.Separator))
	if len(parts) == 0 {
		return ""
	}

	component := parts[0]

	// Known StackRox components
	knownComponents := map[string]bool{
		"central":          true,
		"sensor":           true,
		"roxctl":           true,
		"scanner":          true,
		"operator":         true,
		"migrator":         true,
		"ui":               true,
		"pkg":              true,
		"generated":        true,
		"tools":            true,
		"qa-tests-backend": true,
	}

	if knownComponents[component] {
		return component
	}

	return "other"
}

// AnalyzeAllImpacts analyzes impact for all upgrade candidates
func AnalyzeAllImpacts(candidates []UpgradeCandidate, repoRoot string) error {
	for i := range candidates {
		impact, err := AnalyzeImpact(candidates[i].BasePath, repoRoot)
		if err != nil {
			// Log warning but continue
			fmt.Printf("Warning: failed to analyze impact for %s: %v\n", candidates[i].BasePath, err)
			continue
		}
		candidates[i].Impact = impact
	}
	return nil
}

// IsCritical determines if this is a critical component (central/sensor)
func (ia *ImpactAnalysis) IsCritical() bool {
	for _, component := range ia.Components {
		if component == "central" || component == "sensor" {
			return true
		}
	}
	return false
}
