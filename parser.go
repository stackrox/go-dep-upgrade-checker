package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/mod/modfile"
)

// Dependency represents a Go module dependency with version information
type Dependency struct {
	Path              string     // Full module path (e.g., github.com/foo/bar/v4)
	BasePath          string     // Path without version suffix (e.g., github.com/foo/bar)
	CurrentVer        string     // Current major version (e.g., v4)
	CurrentFull       string     // Full version string (e.g., v4.12.2)
	IsDirect          bool       // Whether it's a direct dependency
	IsReplaced        bool       // Whether it's replaced by a custom fork
	CurrentMajor      int        // Numeric major version (4)
	CurrentReleasedAt *time.Time // When the current version was released
}

// versionSuffixRegex matches /v2, /v3, /v4, etc. at the end of a module path
var versionSuffixRegex = regexp.MustCompile(`/v(\d+)$`)

// ParseGoMod reads and parses a go.mod file, returning versioned dependencies
func ParseGoMod(goModPath string) ([]Dependency, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read go.mod: %w", err)
	}

	modFile, err := modfile.Parse(goModPath, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse go.mod: %w", err)
	}

	// Build a set of replaced module paths to skip
	replacedPaths := make(map[string]bool)
	for _, replace := range modFile.Replace {
		replacedPaths[replace.Old.Path] = true
	}

	var deps []Dependency
	seen := make(map[string]bool) // Deduplicate

	// Process all require directives
	for _, req := range modFile.Require {
		path := req.Mod.Path
		version := req.Mod.Version

		// Skip if already processed
		if seen[path] {
			continue
		}
		seen[path] = true

		var dep Dependency

		// Check if this module has a version suffix (/v2, /v3, etc.)
		matches := versionSuffixRegex.FindStringSubmatch(path)
		if len(matches) > 0 {
			// Has version suffix (e.g., github.com/foo/bar/v3)
			majorVer := matches[1]
			basePath := versionSuffixRegex.ReplaceAllString(path, "")

			dep = Dependency{
				Path:         path,
				BasePath:     basePath,
				CurrentVer:   "v" + majorVer,
				CurrentFull:  version,
				IsDirect:     !req.Indirect,
				IsReplaced:   replacedPaths[path],
				CurrentMajor: parseInt(majorVer),
			}
		} else {
			// No version suffix - infer major version from actual version string
			// e.g., github.com/spf13/cobra v1.8.0 -> major is 1
			// e.g., github.com/foo/bar v0.3.0 -> major is 0
			currentMajor := extractMajorFromVersion(version)

			dep = Dependency{
				Path:         path,
				BasePath:     path, // No suffix, path is base path
				CurrentVer:   fmt.Sprintf("v%d", currentMajor),
				CurrentFull:  version,
				IsDirect:     !req.Indirect,
				IsReplaced:   replacedPaths[path],
				CurrentMajor: currentMajor,
			}
		}

		deps = append(deps, dep)
	}

	return deps, nil
}

// parseInt converts a string to int, returning 0 on error
func parseInt(s string) int {
	result, _ := strconv.Atoi(s)
	return result
}

// extractMajorFromVersion extracts the major version number from a version string
// e.g., "v1.8.0" -> 1, "v0.3.2" -> 0, "v2.1.0" -> 2
func extractMajorFromVersion(version string) int {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	// Split by '.' and get first part
	parts := strings.Split(version, ".")
	if len(parts) == 0 {
		return 0
	}

	major, _ := strconv.Atoi(parts[0])
	return major
}

// FilterDependencies returns dependencies matching the given criteria
func FilterDependencies(deps []Dependency, includeIndirect, includeReplaced bool) []Dependency {
	var filtered []Dependency
	for _, dep := range deps {
		if !includeIndirect && !dep.IsDirect {
			continue
		}
		if !includeReplaced && dep.IsReplaced {
			continue
		}
		filtered = append(filtered, dep)
	}
	return filtered
}

// GroupByBasePath groups dependencies by their base path (without version suffix)
// This helps identify version conflicts where multiple majors of the same package exist
func GroupByBasePath(deps []Dependency) map[string][]Dependency {
	groups := make(map[string][]Dependency)
	for _, dep := range deps {
		groups[dep.BasePath] = append(groups[dep.BasePath], dep)
	}
	return groups
}

// FindVersionConflicts identifies cases where both direct and indirect deps
// have different major versions of the same package
func FindVersionConflicts(deps []Dependency) []VersionConflict {
	groups := GroupByBasePath(deps)
	var conflicts []VersionConflict

	for basePath, groupDeps := range groups {
		if len(groupDeps) < 2 {
			continue
		}

		// Find if there's a mix of direct and indirect with different versions
		var directDeps, indirectDeps []Dependency
		for _, dep := range groupDeps {
			if dep.IsDirect {
				directDeps = append(directDeps, dep)
			} else {
				indirectDeps = append(indirectDeps, dep)
			}
		}

		if len(directDeps) > 0 && len(indirectDeps) > 0 {
			conflicts = append(conflicts, VersionConflict{
				BasePath:     basePath,
				DirectDeps:   directDeps,
				IndirectDeps: indirectDeps,
			})
		}
	}

	return conflicts
}

// VersionConflict represents a case where direct and indirect deps have different versions
type VersionConflict struct {
	BasePath     string
	DirectDeps   []Dependency
	IndirectDeps []Dependency
}

// String formats a version conflict for display
func (vc VersionConflict) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Package: %s", vc.BasePath))

	if len(vc.DirectDeps) > 0 {
		var versions []string
		for _, d := range vc.DirectDeps {
			versions = append(versions, fmt.Sprintf("%s (%s)", d.CurrentVer, d.CurrentFull))
		}
		parts = append(parts, fmt.Sprintf("  Direct: %s", strings.Join(versions, ", ")))
	}

	if len(vc.IndirectDeps) > 0 {
		var versions []string
		for _, d := range vc.IndirectDeps {
			versions = append(versions, fmt.Sprintf("%s (%s)", d.CurrentVer, d.CurrentFull))
		}
		parts = append(parts, fmt.Sprintf("  Indirect: %s", strings.Join(versions, ", ")))
	}

	return strings.Join(parts, "\n")
}
