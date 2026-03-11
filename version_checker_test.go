package main

import (
	"testing"
)

func TestSemverRegex(t *testing.T) {
	tests := []struct {
		tag           string
		shouldMatch   bool
		expectedMajor string
		expectedMinor string
		expectedPatch string
	}{
		{
			tag:           "v1.2.3",
			shouldMatch:   true,
			expectedMajor: "1",
			expectedMinor: "2",
			expectedPatch: "3",
		},
		{
			tag:           "v60.0.0",
			shouldMatch:   true,
			expectedMajor: "60",
			expectedMinor: "0",
			expectedPatch: "0",
		},
		{
			tag:           "v2.10.15",
			shouldMatch:   true,
			expectedMajor: "2",
			expectedMinor: "10",
			expectedPatch: "15",
		},
		{
			tag:         "1.2.3",
			shouldMatch: false,
		},
		{
			tag:         "v1.2",
			shouldMatch: false,
		},
		{
			tag:         "latest",
			shouldMatch: false,
		},
		{
			tag:         "release-1.2.3",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		matches := semverRegex.FindStringSubmatch(tt.tag)

		if tt.shouldMatch {
			if len(matches) < 4 {
				t.Errorf("Expected tag %q to match semver pattern, but it didn't", tt.tag)
				continue
			}
			if matches[1] != tt.expectedMajor {
				t.Errorf("Tag %q: expected major %q, got %q", tt.tag, tt.expectedMajor, matches[1])
			}
			if matches[2] != tt.expectedMinor {
				t.Errorf("Tag %q: expected minor %q, got %q", tt.tag, tt.expectedMinor, matches[2])
			}
			if matches[3] != tt.expectedPatch {
				t.Errorf("Tag %q: expected patch %q, got %q", tt.tag, tt.expectedPatch, matches[3])
			}
		} else {
			if len(matches) > 0 {
				t.Errorf("Expected tag %q NOT to match semver pattern, but it did: %v", tt.tag, matches)
			}
		}
	}
}

func TestVersionJumpCalculation(t *testing.T) {
	uc := UpgradeCandidate{
		Dependency: Dependency{
			CurrentMajor: 60,
		},
		AvailableVersions: []AvailableVersion{
			{Major: 61, FullVersion: "v61.0.0"},
			{Major: 62, FullVersion: "v62.0.0"},
			{Major: 84, FullVersion: "v84.0.0"},
		},
	}

	jump := uc.VersionJump()
	if jump != 24 {
		t.Errorf("Expected version jump of 24 (60→84), got %d", jump)
	}
}

func TestResolveVanityImportInVersionChecker(t *testing.T) {
	// Test that version checker can handle vanity imports
	tests := []struct {
		basePath string
		expected string
	}{
		{
			basePath: "k8s.io/client-go",
			expected: "github.com/kubernetes/client-go",
		},
		{
			basePath: "github.com/google/go-github",
			expected: "github.com/google/go-github",
		},
	}

	for _, tt := range tests {
		result := resolveVanityImport(tt.basePath)
		if result != tt.expected {
			t.Errorf("resolveVanityImport(%q) = %q, want %q", tt.basePath, result, tt.expected)
		}
	}
}

func TestFetchVersionListFromProxy(t *testing.T) {
	// Test fetching versions from the Go module proxy
	// This is an integration test that requires network access
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	versions, err := fetchVersionListFromProxy("golang.org/x/mod")
	if err != nil {
		t.Skipf("Failed to fetch from proxy (may be offline): %v", err)
	}

	if len(versions) == 0 {
		t.Error("Expected to find versions for golang.org/x/mod")
	}

	// Verify versions are in expected format
	hasValidVersion := false
	for _, v := range versions {
		if semverRegex.MatchString(v) {
			hasValidVersion = true
			break
		}
	}

	if !hasValidVersion {
		t.Errorf("Expected at least one valid semver in %v", versions)
	}
}

func TestProcessProxyVersions(t *testing.T) {
	dep := Dependency{
		Path:         "example.com/pkg/v2",
		BasePath:     "example.com/pkg",
		CurrentMajor: 2,
	}

	versions := []string{
		"v2.0.0",
		"v2.1.0",
		"v3.0.0",
		"v3.5.2",
		"v4.0.0",
		"v5.1.3",
		"invalid-version",
		"v1.0.0", // Should be ignored (older than current)
	}

	result := processProxyVersions(dep, versions)

	if result == nil {
		t.Fatal("Expected upgrade candidate, got nil")
	}

	if len(result.AvailableVersions) != 3 {
		t.Errorf("Expected 3 versions (v3, v4, v5), got %d", len(result.AvailableVersions))
	}

	// Verify v3 has the latest patch version
	if result.AvailableVersions[0].FullVersion != "v3.5.2" {
		t.Errorf("Expected v3.5.2, got %s", result.AvailableVersions[0].FullVersion)
	}
}
