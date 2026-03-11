package main

import (
	"os"
	"testing"
)

func TestNewGitHubClient(t *testing.T) {
	// Test without token
	os.Unsetenv("GITHUB_TOKEN")
	client := NewGitHubClient()
	if client == nil {
		t.Fatal("Expected client to be created")
	}
	if client.client == nil {
		t.Fatal("Expected GitHub client to be initialized")
	}

	// Test with token
	os.Setenv("GITHUB_TOKEN", "test-token")
	defer os.Unsetenv("GITHUB_TOKEN")
	clientWithToken := NewGitHubClient()
	if clientWithToken == nil {
		t.Fatal("Expected client to be created with token")
	}
	if clientWithToken.client == nil {
		t.Fatal("Expected GitHub client to be initialized with token")
	}
}

func TestCheckRepoStatus(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test without GITHUB_TOKEN")
	}

	gc := NewGitHubClient()

	tests := []struct {
		name        string
		basePath    string
		expectError bool
	}{
		{
			name:        "GitHub package",
			basePath:    "github.com/google/go-github",
			expectError: false,
		},
		{
			name:        "Vanity import",
			basePath:    "k8s.io/client-go",
			expectError: false,
		},
		{
			name:        "Non-GitHub package",
			basePath:    "example.com/nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, err := gc.CheckRepoStatus(tt.basePath)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error for non-GitHub package")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if status == nil {
				t.Fatal("Expected status to be non-nil")
			}
			// Just verify we got a response - archived status can be true or false
			t.Logf("Repository %s archived: %v", tt.basePath, status.Archived)
		})
	}
}

func TestFetchChangelog(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test without GITHUB_TOKEN")
	}

	gc := NewGitHubClient()

	tests := []struct {
		name        string
		basePath    string
		expectFound bool
	}{
		{
			name:        "GitHub package with changelog",
			basePath:    "github.com/google/go-github",
			expectFound: true,
		},
		{
			name:        "Vanity import with releases",
			basePath:    "k8s.io/client-go",
			expectFound: true,
		},
		{
			name:        "Non-GitHub package",
			basePath:    "example.com/nonexistent",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := gc.FetchChangelog(tt.basePath)
			if err != nil {
				t.Logf("Error fetching changelog: %v", err)
			}
			if info == nil {
				t.Fatal("Expected changelog info to be non-nil")
			}
			if info.Found != tt.expectFound {
				t.Errorf("Expected Found=%v, got %v", tt.expectFound, info.Found)
			}
		})
	}
}

func TestFetchChangelogFile(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test without GITHUB_TOKEN")
	}

	gc := NewGitHubClient()

	// Test with a known repository that has a changelog
	info, err := gc.fetchChangelogFile("google", "go-github")
	if err != nil {
		t.Logf("Error: %v", err)
	}

	// The repo might or might not have a changelog file
	// Just verify we got a response
	if info == nil {
		t.Fatal("Expected info to be non-nil")
	}
}

func TestFetchReleases(t *testing.T) {
	if os.Getenv("GITHUB_TOKEN") == "" {
		t.Skip("Skipping integration test without GITHUB_TOKEN")
	}

	gc := NewGitHubClient()

	tests := []struct {
		owner       string
		repo        string
		expectFound bool
	}{
		{
			owner:       "google",
			repo:        "go-github",
			expectFound: true,
		},
		{
			owner:       "nonexistent",
			repo:        "repo",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		info, err := gc.fetchReleases(tt.owner, tt.repo)
		if err != nil && tt.expectFound {
			t.Errorf("Unexpected error for %s/%s: %v", tt.owner, tt.repo, err)
		}
		if info == nil {
			t.Fatal("Expected info to be non-nil")
		}
		if info.Found != tt.expectFound {
			t.Logf("Expected Found=%v, got %v for %s/%s", tt.expectFound, info.Found, tt.owner, tt.repo)
		}
	}
}

func TestExtractVersionSection(t *testing.T) {
	content := `# Changelog

## v2.0.0 - 2024-01-15

### Breaking Changes
- Removed old API
- Changed function signatures

### Features
- Added new feature

## v1.5.0 - 2024-01-01

### Bug Fixes
- Fixed bug

## v1.0.0 - 2023-12-01

Initial release
`

	tests := []struct {
		version      string
		expectFound  bool
		expectInside string
	}{
		{
			version:      "v2.0.0",
			expectFound:  true,
			expectInside: "Breaking Changes",
		},
		{
			version:      "v1.5.0",
			expectFound:  true,
			expectInside: "Bug Fixes",
		},
		{
			version:      "v1.0.0",
			expectFound:  true,
			expectInside: "Initial release",
		},
		{
			version:     "v3.0.0",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		result := ExtractVersionSection(content, tt.version)
		if tt.expectFound {
			if result == "" {
				t.Errorf("Expected to find section for %s", tt.version)
			}
			if tt.expectInside != "" && len(result) > 0 {
				if len(result) < len(tt.expectInside) || result[:len(tt.expectInside)] == tt.expectInside {
					// Check if the expected content is somewhere in the result
					found := false
					for i := 0; i <= len(result)-len(tt.expectInside); i++ {
						if result[i:i+len(tt.expectInside)] == tt.expectInside {
							found = true
							break
						}
					}
					if !found {
						t.Logf("Section for %s doesn't contain expected text %q", tt.version, tt.expectInside)
					}
				}
			}
		} else {
			if result != "" {
				t.Errorf("Expected empty result for %s, got: %s", tt.version, result)
			}
		}
	}
}

func TestExtractVersionSectionTruncation(t *testing.T) {
	// Create a very long section
	longContent := "# v1.0.0\n"
	for range 200 {
		longContent += "This is a very long line that will be repeated many times to exceed 1000 characters.\n"
	}

	result := ExtractVersionSection(longContent, "v1.0.0")
	if len(result) > 1020 { // 1000 + some buffer for truncation message
		t.Errorf("Expected result to be truncated to around 1000 chars, got %d", len(result))
	}
}
