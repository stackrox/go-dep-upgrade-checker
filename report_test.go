package main

import (
	"testing"
)

func TestExtractPackageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "github.com/vbauerster/mpb/v4",
			expected: "vbauerster/mpb",
		},
		{
			input:    "github.com/google/go-github/v60",
			expected: "google/go-github",
		},
		{
			input:    "github.com/foo/bar",
			expected: "foo/bar",
		},
		{
			input:    "k8s.io/api/v2",
			expected: "k8s.io/api",
		},
		{
			input:    "example.com/pkg",
			expected: "example.com/pkg",
		},
	}

	for _, tt := range tests {
		result := extractPackageName(tt.input)
		if result != tt.expected {
			t.Errorf("extractPackageName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractGitHubURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "github.com/google/go-github",
			expected: "https://github.com/google/go-github",
		},
		{
			input:    "github.com/kubernetes/kubernetes",
			expected: "https://github.com/kubernetes/kubernetes",
		},
		{
			input:    "k8s.io/api",
			expected: "https://github.com/kubernetes/api",
		},
		{
			input:    "k8s.io/client-go",
			expected: "https://github.com/kubernetes/client-go",
		},
		{
			input:    "go.uber.org/zap",
			expected: "https://github.com/uber-go/zap",
		},
		{
			input:    "golang.org/x/tools",
			expected: "https://github.com/golang/tools",
		},
		{
			input:    "example.com/unknown",
			expected: "",
		},
	}

	for _, tt := range tests {
		result := extractGitHubURL(tt.input)
		if result != tt.expected {
			t.Errorf("extractGitHubURL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestVersionJump(t *testing.T) {
	uc := UpgradeCandidate{
		Dependency: Dependency{
			CurrentMajor: 2,
		},
		AvailableVersions: []AvailableVersion{
			{MajorVer: "v3", Major: 3},
			{MajorVer: "v4", Major: 4},
			{MajorVer: "v5", Major: 5},
		},
	}

	jump := uc.VersionJump()
	if jump != 3 {
		t.Errorf("VersionJump() = %d, want 3", jump)
	}

	// Test with no available versions
	uc.AvailableVersions = nil
	jump = uc.VersionJump()
	if jump != 0 {
		t.Errorf("VersionJump() with no versions = %d, want 0", jump)
	}
}

func TestLatestAvailable(t *testing.T) {
	uc := UpgradeCandidate{
		AvailableVersions: []AvailableVersion{
			{MajorVer: "v3", FullVersion: "v3.0.0", Major: 3},
			{MajorVer: "v4", FullVersion: "v4.1.2", Major: 4},
			{MajorVer: "v5", FullVersion: "v5.0.0", Major: 5},
		},
	}

	latest := uc.LatestAvailable()
	if latest.Major != 5 {
		t.Errorf("LatestAvailable() major = %d, want 5", latest.Major)
	}
	if latest.FullVersion != "v5.0.0" {
		t.Errorf("LatestAvailable() version = %s, want v5.0.0", latest.FullVersion)
	}

	// Test with no versions
	uc.AvailableVersions = nil
	latest = uc.LatestAvailable()
	if latest.Major != 0 {
		t.Errorf("LatestAvailable() with no versions = %d, want 0", latest.Major)
	}
}
