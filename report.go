package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// plural returns "s" if n != 1, otherwise empty string
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// GenerateBasicReport creates a Markdown report of upgrade candidates and archived dependencies
func GenerateBasicReport(candidates []UpgradeCandidate, archivedDeps []ArchivedDependency) string {
	var sb strings.Builder

	sb.WriteString("# Dependency Major Version Upgrade Analysis\n")
	fmt.Fprintf(&sb, "Generated: %s\n\n", time.Now().UTC().Format("2006-01-02 15:04:05 MST"))

	// Summary section
	archivedWithUpgrades := 0
	for _, c := range candidates {
		if c.Archived {
			archivedWithUpgrades++
		}
	}
	totalArchived := archivedWithUpgrades + len(archivedDeps)

	sb.WriteString("## Summary\n")
	fmt.Fprintf(&sb, "- Upgrade candidates: %d\n", len(candidates))
	if totalArchived > 0 {
		fmt.Fprintf(&sb, "- ⚠️  Archived dependencies: %d\n", totalArchived)
	}
	sb.WriteString("\n")

	// Sort candidates by version jump (descending), then alphabetically by path
	sortedCandidates := make([]UpgradeCandidate, len(candidates))
	copy(sortedCandidates, candidates)
	sort.Slice(sortedCandidates, func(i, j int) bool {
		if sortedCandidates[i].VersionJump() != sortedCandidates[j].VersionJump() {
			return sortedCandidates[i].VersionJump() > sortedCandidates[j].VersionJump()
		}
		return sortedCandidates[i].Path < sortedCandidates[j].Path
	})

	// Available upgrades section
	if len(sortedCandidates) > 0 {
		sb.WriteString("## Available Upgrades\n\n")
		for _, uc := range sortedCandidates {
			writeUpgradeCandidate(&sb, uc)
		}
	}

	// Archived dependencies without upgrades section
	if len(archivedDeps) > 0 {
		sb.WriteString("## Archived Dependencies (No Upgrades Available)\n\n")
		sb.WriteString("The following dependencies are archived and no longer maintained. ")
		sb.WriteString("You are currently on the latest version, but you should consider migrating to actively maintained alternatives.\n\n")

		// Sort archived deps alphabetically
		sortedArchived := make([]ArchivedDependency, len(archivedDeps))
		copy(sortedArchived, archivedDeps)
		sort.Slice(sortedArchived, func(i, j int) bool {
			return sortedArchived[i].Path < sortedArchived[j].Path
		})

		for _, ad := range sortedArchived {
			writeArchivedDependency(&sb, ad)
		}
	}

	return sb.String()
}

// writeUpgradeCandidate writes a single upgrade candidate to the report
func writeUpgradeCandidate(sb *strings.Builder, uc UpgradeCandidate) {
	latest := uc.LatestAvailable()

	// Header with package name and version jump
	fmt.Fprintf(sb, "### %s: %s → %s (%d major version",
		extractPackageName(uc.Path), uc.CurrentVer, latest.MajorVer, uc.VersionJump())
	if uc.VersionJump() > 1 {
		sb.WriteString("s")
	}
	sb.WriteString(")\n\n")

	// Archived warning (prominent)
	if uc.Archived {
		sb.WriteString("**🗄️  ARCHIVED**: This repository is archived and no longer maintained\n\n")
	}

	// Current version with full version string, release date and age
	if uc.CurrentReleasedAt != nil {
		// Calculate age
		age := time.Since(*uc.CurrentReleasedAt)
		days := int(age.Hours() / 24)
		ageStr := fmt.Sprintf("%d days old", days)
		if days > 365 {
			years := days / 365
			months := (days % 365) / 30
			if months > 0 {
				ageStr = fmt.Sprintf("%d year%s, %d month%s old", years, plural(years), months, plural(months))
			} else {
				ageStr = fmt.Sprintf("%d year%s old", years, plural(years))
			}
		} else if days > 30 {
			months := days / 30
			ageStr = fmt.Sprintf("%d month%s old", months, plural(months))
		}
		fmt.Fprintf(sb, "**Current**: %s (released %s, %s)\n",
			uc.CurrentFull, uc.CurrentReleasedAt.Format("2006-01-02"), ageStr)
	} else {
		fmt.Fprintf(sb, "**Current**: %s\n", uc.CurrentFull)
	}

	// Latest available version with release date
	if latest.ReleasedAt != nil {
		fmt.Fprintf(sb, "**Latest**: %s (released %s)\n",
			latest.FullVersion, latest.ReleasedAt.Format("2006-01-02"))

		// Warn if the "upgrade" is actually older than current
		if uc.CurrentReleasedAt != nil && latest.ReleasedAt.Before(*uc.CurrentReleasedAt) {
			sb.WriteString("**⚠️  Warning**: This version is OLDER than your current version (suggests version numbering scheme changed)\n")
		}
	} else {
		fmt.Fprintf(sb, "**Latest**: %s\n", latest.FullVersion)
	}

	// Impact analysis
	if uc.Impact != nil {
		fmt.Fprintf(sb, "**Impact**: %d files affected", uc.Impact.FilesAffected)
		if len(uc.Impact.Components) > 0 {
			fmt.Fprintf(sb, " (%s)", strings.Join(uc.Impact.Components, ", "))
		}
		sb.WriteString("\n")
	}

	// GitHub link
	githubURL := extractGitHubURL(uc.BasePath)
	if githubURL != "" {
		fmt.Fprintf(sb, "**Repository**: %s\n", githubURL)
	}

	// Changelog information
	if uc.Changelog != nil && uc.Changelog.Found {
		fmt.Fprintf(sb, "**Changelog**: %s\n", uc.Changelog.URL)

		// Breaking changes
		if len(uc.Changelog.BreakingChanges) > 0 {
			sb.WriteString("**Breaking Changes**:\n")
			for _, change := range uc.Changelog.BreakingChanges {
				fmt.Fprintf(sb, "- %s\n", change)
			}
		}
	}

	sb.WriteString("\n")
}

// writeArchivedDependency writes a single archived dependency to the report
func writeArchivedDependency(sb *strings.Builder, ad ArchivedDependency) {
	// Header with package name
	fmt.Fprintf(sb, "### %s: %s\n\n", extractPackageName(ad.Path), ad.CurrentFull)

	// Archived warning
	sb.WriteString("**🗄️  ARCHIVED**: This repository is archived and no longer maintained\n\n")

	// Current version info
	if ad.CurrentReleasedAt != nil {
		// Calculate age
		age := time.Since(*ad.CurrentReleasedAt)
		days := int(age.Hours() / 24)
		ageStr := fmt.Sprintf("%d days old", days)
		if days > 365 {
			years := days / 365
			months := (days % 365) / 30
			if months > 0 {
				ageStr = fmt.Sprintf("%d year%s, %d month%s old", years, plural(years), months, plural(months))
			} else {
				ageStr = fmt.Sprintf("%d year%s old", years, plural(years))
			}
		} else if days > 30 {
			months := days / 30
			ageStr = fmt.Sprintf("%d month%s old", months, plural(months))
		}
		fmt.Fprintf(sb, "**Current Version**: %s (released %s, %s)\n", ad.CurrentFull, ad.CurrentReleasedAt.Format("2006-01-02"), ageStr)
	} else {
		fmt.Fprintf(sb, "**Current Version**: %s\n", ad.CurrentFull)
	}

	// GitHub link
	githubURL := extractGitHubURL(ad.BasePath)
	if githubURL != "" {
		fmt.Fprintf(sb, "**Repository**: %s\n", githubURL)
	}

	sb.WriteString("\n")
}

// extractPackageName extracts a short package name from the full path
// e.g., github.com/vbauerster/mpb/v4 -> vbauerster/mpb
func extractPackageName(path string) string {
	// Remove version suffix
	basePath := versionSuffixRegex.ReplaceAllString(path, "")

	// For github.com/owner/repo format, return owner/repo
	parts := strings.Split(basePath, "/")
	if len(parts) >= 3 && parts[0] == "github.com" {
		return parts[1] + "/" + parts[2]
	}

	// For other paths, return last 2 components or the whole thing
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}

	return basePath
}

// extractGitHubURL converts a module path to a GitHub URL if possible
func extractGitHubURL(path string) string {
	// Resolve vanity imports first
	resolvedPath := resolveVanityImport(path)

	if strings.HasPrefix(resolvedPath, "github.com/") {
		parts := strings.Split(resolvedPath, "/")
		if len(parts) >= 3 {
			return fmt.Sprintf("https://github.com/%s/%s", parts[1], parts[2])
		}
	}
	return ""
}
