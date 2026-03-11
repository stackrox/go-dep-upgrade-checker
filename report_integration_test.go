package main

import (
	"strings"
	"testing"
)

func TestGenerateBasicReport(t *testing.T) {
	candidates := []UpgradeCandidate{
		{
			Dependency: Dependency{
				Path:         "github.com/foo/bar/v3",
				BasePath:     "github.com/foo/bar",
				CurrentVer:   "v3",
				CurrentFull:  "v3.2.1",
				CurrentMajor: 3,
				IsDirect:     true,
			},
			AvailableVersions: []AvailableVersion{
				{MajorVer: "v4", FullVersion: "v4.0.0", Major: 4},
				{MajorVer: "v5", FullVersion: "v5.1.0", Major: 5},
			},
			Impact: &ImpactAnalysis{
				FilesAffected: 5,
				Components:    []string{"central", "sensor"},
			},
		},
		{
			Dependency: Dependency{
				Path:         "github.com/baz/qux/v2",
				BasePath:     "github.com/baz/qux",
				CurrentVer:   "v2",
				CurrentFull:  "v2.0.0",
				CurrentMajor: 2,
				IsDirect:     false,
			},
			AvailableVersions: []AvailableVersion{
				{MajorVer: "v3", FullVersion: "v3.0.0", Major: 3},
			},
		},
	}

	report := GenerateBasicReport(candidates, nil)

	// Check report contains expected sections
	if !strings.Contains(report, "# Dependency Major Version Upgrade Analysis") {
		t.Error("Report missing title")
	}

	if !strings.Contains(report, "## Summary") {
		t.Error("Report missing summary section")
	}

	if !strings.Contains(report, "- Upgrade candidates: 2") {
		t.Error("Report missing candidate count")
	}

	if !strings.Contains(report, "## Available Upgrades") {
		t.Error("Report missing upgrades section")
	}

	// Check package details
	if !strings.Contains(report, "foo/bar: v3 → v5 (2 major versions)") {
		t.Error("Report missing foo/bar upgrade info")
	}

	if !strings.Contains(report, "baz/qux: v2 → v3 (1 major version)") {
		t.Error("Report missing baz/qux upgrade info")
	}

	// Check impact info
	if !strings.Contains(report, "**Impact**: 5 files affected (central, sensor)") {
		t.Error("Report missing impact info")
	}

	// Check latest version is shown
	if !strings.Contains(report, "**Latest**: v5.1.0") {
		t.Error("Report missing latest version")
	}

	// Check sorting (foo/bar should come before baz/qux due to larger version jump)
	fooIdx := strings.Index(report, "foo/bar")
	bazIdx := strings.Index(report, "baz/qux")
	if fooIdx == -1 || bazIdx == -1 || fooIdx > bazIdx {
		t.Error("Report not sorted correctly by version jump")
	}
}

func TestGenerateBasicReportWithChangelog(t *testing.T) {
	candidates := []UpgradeCandidate{
		{
			Dependency: Dependency{
				Path:         "github.com/test/pkg/v2",
				BasePath:     "github.com/test/pkg",
				CurrentVer:   "v2",
				CurrentFull:  "v2.0.0",
				CurrentMajor: 2,
				IsDirect:     true,
			},
			AvailableVersions: []AvailableVersion{
				{MajorVer: "v3", FullVersion: "v3.0.0", Major: 3},
			},
			Changelog: &ChangelogInfo{
				Found:  true,
				Source: "releases",
				URL:    "https://github.com/test/pkg/releases/tag/v3.0.0",
				BreakingChanges: []string{
					"API changed",
					"Removed deprecated functions",
				},
			},
		},
	}

	report := GenerateBasicReport(candidates, nil)

	if !strings.Contains(report, "**Changelog**: https://github.com/test/pkg/releases/tag/v3.0.0") {
		t.Error("Report missing changelog link")
	}

	if !strings.Contains(report, "**Breaking Changes**:") {
		t.Error("Report missing breaking changes section")
	}

	if !strings.Contains(report, "- API changed") {
		t.Error("Report missing first breaking change")
	}

	if !strings.Contains(report, "- Removed deprecated functions") {
		t.Error("Report missing second breaking change")
	}
}

func TestGenerateBasicReportEmpty(t *testing.T) {
	report := GenerateBasicReport([]UpgradeCandidate{}, nil)

	if !strings.Contains(report, "- Upgrade candidates: 0") {
		t.Error("Empty report should show 0 candidates")
	}
}

func TestGenerateBasicReportArchived(t *testing.T) {
	candidates := []UpgradeCandidate{
		{
			Dependency: Dependency{
				Path:         "github.com/archived/pkg/v1",
				BasePath:     "github.com/archived/pkg",
				CurrentVer:   "v1",
				CurrentFull:  "v1.0.0",
				CurrentMajor: 1,
			},
			AvailableVersions: []AvailableVersion{
				{MajorVer: "v2", FullVersion: "v2.0.0", Major: 2},
			},
			Archived: true,
		},
		{
			Dependency: Dependency{
				Path:         "github.com/active/pkg/v1",
				BasePath:     "github.com/active/pkg",
				CurrentVer:   "v1",
				CurrentFull:  "v1.0.0",
				CurrentMajor: 1,
			},
			AvailableVersions: []AvailableVersion{
				{MajorVer: "v2", FullVersion: "v2.0.0", Major: 2},
			},
			Archived: false,
		},
	}

	report := GenerateBasicReport(candidates, nil)

	// Check archived warning appears
	if !strings.Contains(report, "🗄️  ARCHIVED") {
		t.Error("Report missing archived warning")
	}

	// Check archived count in summary
	if !strings.Contains(report, "⚠️  Archived dependencies: 1") {
		t.Error("Report missing archived count in summary")
	}
}

func TestGenerateBasicReportArchivedWithoutUpgrades(t *testing.T) {
	archivedDeps := []ArchivedDependency{
		{
			Dependency: Dependency{
				Path:         "github.com/archived/pkg/v2",
				BasePath:     "github.com/archived/pkg",
				CurrentVer:   "v2",
				CurrentFull:  "v2.5.0",
				CurrentMajor: 2,
			},
		},
	}

	report := GenerateBasicReport(nil, archivedDeps)

	// Check archived section appears
	if !strings.Contains(report, "## Archived Dependencies (No Upgrades Available)") {
		t.Error("Report missing archived dependencies section")
	}

	// Check archived warning appears
	if !strings.Contains(report, "🗄️  ARCHIVED") {
		t.Error("Report missing archived warning")
	}

	// Check archived count in summary
	if !strings.Contains(report, "⚠️  Archived dependencies: 1") {
		t.Error("Report missing archived count in summary")
	}

	// Check guidance text
	if !strings.Contains(report, "consider migrating to actively maintained alternatives") {
		t.Error("Report missing migration guidance")
	}
}

func TestWriteUpgradeCandidateVersionPlural(t *testing.T) {
	// Test singular "version"
	candidates := []UpgradeCandidate{
		{
			Dependency: Dependency{
				Path:         "github.com/test/pkg/v2",
				BasePath:     "github.com/test/pkg",
				CurrentVer:   "v2",
				CurrentFull:  "v2.0.0",
				CurrentMajor: 2,
			},
			AvailableVersions: []AvailableVersion{
				{MajorVer: "v3", FullVersion: "v3.0.0", Major: 3},
			},
		},
	}

	report := GenerateBasicReport(candidates, nil)
	if !strings.Contains(report, "(1 major version)") {
		t.Error("Should say 'version' (singular) for 1 version jump")
	}

	// Test plural "versions"
	candidates[0].AvailableVersions = append(candidates[0].AvailableVersions,
		AvailableVersion{MajorVer: "v4", FullVersion: "v4.0.0", Major: 4})

	report = GenerateBasicReport(candidates, nil)
	if !strings.Contains(report, "(2 major versions)") {
		t.Error("Should say 'versions' (plural) for 2+ version jump")
	}
}
