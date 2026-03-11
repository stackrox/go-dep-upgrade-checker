package main

import (
	"strings"
	"testing"
)

func TestResolveVanityImport(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "k8s.io/api",
			expected: "github.com/kubernetes/api",
		},
		{
			input:    "k8s.io/client-go",
			expected: "github.com/kubernetes/client-go",
		},
		{
			input:    "sigs.k8s.io/yaml",
			expected: "github.com/kubernetes-sigs/yaml",
		},
		{
			input:    "go.uber.org/zap",
			expected: "github.com/uber-go/zap",
		},
		{
			input:    "golang.org/x/tools",
			expected: "github.com/golang/tools",
		},
		{
			input:    "google.golang.org/grpc",
			expected: "github.com/grpc/grpc-go",
		},
		{
			input:    "google.golang.org/protobuf",
			expected: "github.com/protocolbuffers/protobuf-go",
		},
		{
			input:    "gocloud.dev/blob",
			expected: "github.com/google/go-cloud/blob",
		},
		{
			input:    "github.com/foo/bar",
			expected: "github.com/foo/bar",
		},
		{
			input:    "example.com/unknown",
			expected: "example.com/unknown",
		},
	}

	for _, tt := range tests {
		result := resolveVanityImport(tt.input)
		if result != tt.expected {
			t.Errorf("resolveVanityImport(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExtractGitHubOwnerRepo(t *testing.T) {
	tests := []struct {
		input        string
		expectOwner  string
		expectRepo   string
		expectOk     bool
	}{
		{
			input:       "github.com/google/go-github",
			expectOwner: "google",
			expectRepo:  "go-github",
			expectOk:    true,
		},
		{
			input:       "github.com/kubernetes/kubernetes",
			expectOwner: "kubernetes",
			expectRepo:  "kubernetes",
			expectOk:    true,
		},
		{
			input:    "k8s.io/api",
			expectOk: false,
		},
		{
			input:    "github.com/foo",
			expectOk: false,
		},
		{
			input:    "example.com/foo/bar",
			expectOk: false,
		},
	}

	for _, tt := range tests {
		owner, repo, ok := extractGitHubOwnerRepo(tt.input)
		if ok != tt.expectOk {
			t.Errorf("extractGitHubOwnerRepo(%q) ok = %v, want %v", tt.input, ok, tt.expectOk)
			continue
		}
		if ok {
			if owner != tt.expectOwner {
				t.Errorf("extractGitHubOwnerRepo(%q) owner = %q, want %q", tt.input, owner, tt.expectOwner)
			}
			if repo != tt.expectRepo {
				t.Errorf("extractGitHubOwnerRepo(%q) repo = %q, want %q", tt.input, repo, tt.expectRepo)
			}
		}
	}
}

func TestExtractBreakingChanges(t *testing.T) {
	content := `
# v3.0.0

## ⚠️ Breaking Changes

BREAKING: Removed deprecated API endpoint /v1/users
⚠️ Changed function signature for ProcessData(ctx context.Context)
This is not a breaking change

## ⚠️ Changes

REMOVED: Old feature that was deprecated in v2.5.0

[breaking] Important API change: Authentication now requires tokens

## Features

Added new feature
`

	changes := extractBreakingChanges(content)

	if len(changes) == 0 {
		t.Fatal("Expected to find breaking changes, got none")
	}

	// Should find at least 3 breaking changes (but skip generic headers)
	if len(changes) < 3 {
		t.Errorf("Expected at least 3 breaking changes, got %d: %v", len(changes), changes)
	}

	// Should limit to 5 changes max
	if len(changes) > 5 {
		t.Errorf("Expected max 5 breaking changes, got %d", len(changes))
	}

	// Verify it DOESN'T capture generic headers
	for _, change := range changes {
		changeLower := strings.ToLower(change)
		if changeLower == "changes" || changeLower == "breaking changes" || changeLower == "breaking" {
			t.Errorf("Should not capture generic header: %q", change)
		}
		// Should have substantial content (min 10 chars)
		if len(change) < 10 {
			t.Errorf("Change too short (likely a header): %q", change)
		}
	}

	// Verify it captures actual breaking changes
	hasAPIRemoval := false
	hasFunctionChange := false
	hasFeatureRemoval := false

	for _, change := range changes {
		if strings.Contains(change, "Removed deprecated API endpoint") {
			hasAPIRemoval = true
		}
		if strings.Contains(change, "Changed function signature") {
			hasFunctionChange = true
		}
		if strings.Contains(change, "Old feature that was deprecated") {
			hasFeatureRemoval = true
		}
	}

	if !hasAPIRemoval {
		t.Errorf("Expected to find API removal in changes: %v", changes)
	}
	if !hasFunctionChange {
		t.Errorf("Expected to find function change in changes: %v", changes)
	}
	if !hasFeatureRemoval {
		t.Errorf("Expected to find feature removal in changes: %v", changes)
	}
}
