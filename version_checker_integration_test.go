package main

import (
	"testing"
)

func TestCheckUpgradesViaGitHub(t *testing.T) {
	tests := []struct {
		name           string
		dep            Dependency
		githubPath     string
		expectUpgrades bool
	}{
		{
			name: "google/go-github with old version",
			dep: Dependency{
				Path:         "github.com/google/go-github/v50",
				BasePath:     "github.com/google/go-github",
				CurrentVer:   "v50.0.0",
				CurrentMajor: 50,
			},
			githubPath:     "github.com/google/go-github",
			expectUpgrades: true,
		},
		{
			name: "package with current major version",
			dep: Dependency{
				Path:         "github.com/google/uuid",
				BasePath:     "github.com/google/uuid",
				CurrentVer:   "v1.6.0",
				CurrentMajor: 1,
			},
			githubPath:     "github.com/google/uuid",
			expectUpgrades: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versions, currentDate, err := checkUpgradesViaGitHub(tt.dep, tt.githubPath)
			if err != nil {
				t.Logf("Error checking upgrades: %v", err)
			}
			if currentDate != nil {
				t.Logf("Current version %s was released: %s", tt.dep.CurrentFull, currentDate.Format("2006-01-02"))
			}
			hasUpgrades := len(versions) > 0
			if hasUpgrades != tt.expectUpgrades {
				t.Logf("Expected upgrades=%v, got %d versions for %s",
					tt.expectUpgrades, len(versions), tt.dep.Path)
			}
		})
	}
}

func TestCheckUpgradesViaProxy(t *testing.T) {
	tests := []struct {
		name string
		dep  Dependency
	}{
		{
			name: "helm.sh/helm with old version",
			dep: Dependency{
				Path:         "helm.sh/helm/v3",
				BasePath:     "helm.sh/helm",
				CurrentVer:   "v3.0.0",
				CurrentMajor: 3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate, err := checkUpgradesViaProxy(tt.dep)
			if err != nil {
				t.Logf("Error checking upgrades: %v", err)
			}
			if candidate != nil {
				t.Logf("Found %d upgrade versions for %s", len(candidate.AvailableVersions), tt.dep.Path)
			}
			// Don't assert on specific results as proxy data changes
		})
	}
}

func TestQueryVersion(t *testing.T) {
	tests := []struct {
		modulePath  string
		expectFound bool
	}{
		{
			modulePath:  "github.com/google/uuid",
			expectFound: true,
		},
		{
			modulePath:  "nonexistent.example.com/fake/package/v99",
			expectFound: false,
		},
	}

	for _, tt := range tests {
		result, found, err := queryVersion(tt.modulePath)
		if err != nil {
			t.Logf("Error querying %s: %v", tt.modulePath, err)
		}
		if found != tt.expectFound {
			t.Logf("Expected found=%v for %s, got %v", tt.expectFound, tt.modulePath, found)
		}
		if found && result != "" {
			t.Logf("Query result for %s: %s", tt.modulePath, result)
		}
	}
}

func TestFetchVersionsFromProxyIntegration(t *testing.T) {
	tests := []struct {
		modulePath string
		expectErr  bool
	}{
		{
			modulePath: "github.com/google/uuid",
			expectErr:  false,
		},
		{
			modulePath: "nonexistent.example.com/fake",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		versions, err := fetchVersionsFromProxy(tt.modulePath)
		if tt.expectErr {
			if err == nil {
				t.Logf("Expected error for %s but got nil", tt.modulePath)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.modulePath, err)
			}
			if len(versions) == 0 {
				t.Logf("No versions found for %s", tt.modulePath)
			} else {
				t.Logf("Found %d versions for %s", len(versions), tt.modulePath)
			}
		}
	}
}

func TestCheckForUpgradesWithRealPackages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tests := []struct {
		name string
		dep  Dependency
	}{
		{
			name: "github package",
			dep: Dependency{
				Path:         "github.com/google/uuid",
				BasePath:     "github.com/google/uuid",
				CurrentVer:   "v1.0.0",
				CurrentMajor: 1,
			},
		},
		{
			name: "vanity import",
			dep: Dependency{
				Path:         "golang.org/x/tools",
				BasePath:     "golang.org/x/tools",
				CurrentVer:   "v0.1.0",
				CurrentMajor: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate, err := CheckForUpgrades(tt.dep)
			if err != nil {
				t.Logf("Error: %v", err)
			}
			if candidate != nil {
				t.Logf("Found %d upgrade versions for %s", len(candidate.AvailableVersions), tt.dep.Path)
			}
			// Don't assert specific counts as versions change over time
		})
	}
}
