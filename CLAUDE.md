# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A standalone Go tool that discovers major version upgrade opportunities for Go dependencies, fetches changelogs from GitHub, analyzes codebase impact, and generates actionable upgrade reports. It fills a gap left by tools like Dependabot which only handle patch/minor updates.

## Build and Run

```bash
# Build the binary
go build -o dep-upgrade-checker .

# Run from source
go run . -output upgrade-report.md

# Run with specific go.mod
go run . -gomod /path/to/go.mod -output report.md

# Check specific package
go run . -package github.com/google/go-github/v60

# Include indirect dependencies
go run . -include-indirect -output full-report.md

# With GitHub token (recommended for rate limits)
GITHUB_TOKEN=ghp_xxx go run . -output report.md

# Cache management
go run . --cache-stats    # Show cache statistics
go run . --clear-cache    # Clear the cache
go run . --no-cache       # Disable caching for this run
```

## CLI Flags

- `-gomod string`: Path to go.mod file (default "go.mod")
- `-output string`: Output file path (default: stdout)
- `-package string`: Filter by specific package path
- `-include-indirect`: Include indirect dependencies (default: false)
- `-include-replaced`: Include replaced dependencies (default: false)
- `-no-cache`: Disable caching (cache is enabled by default)
- `-clear-cache`: Clear all cached data and exit
- `-cache-stats`: Show cache statistics and exit

## Caching

The tool caches dependency upgrade information to speed up repeated runs (especially useful in CI/CD):
- **Cache location**: `~/.cache/go-dep-upgrade-checker/` (from `os.UserCacheDir()`)
- **TTL**: 24 hours (entries expire after 24 hours)
- **Cached data**: Available versions, changelog info, release dates
- **Cache key**: Based on package path and current version
- **Not cached**: Impact analysis (codebase-specific)

## Report Output

For each upgrade candidate, the tool shows:
- **Current version**: Full version from go.mod with release date and age (e.g., "v50.0.0 (released 2023-01-26, 3 years, 1 month old)")
- **Latest version**: Highest available major version with release date (e.g., "v84.0.0 (released 2026-02-27)")
- **Version jump**: Number of major versions behind (e.g., "34 major versions")
- **Breaking changes**: Extracted from changelog/releases
- **Impact analysis**: Files and components affected in your codebase

## Architecture

### Core Workflow

1. **parser.go**: Parses go.mod using `golang.org/x/mod/modfile`, extracts ALL dependencies (with or without version suffixes). For packages without suffixes (e.g., `v1.8.0`), infers major version from version string
2. **version_checker.go** (with parallel execution):
   - **Parallelism**: Worker pool scaled to 2x CPU cores (minimum 4) for concurrent package checking
   - **GitHub packages**: Fetches all repository tags via GitHub API (one call), parses semantic versions to find all major upgrades (unlimited)
   - **Non-GitHub packages**: Fetches version list from Go module proxy `proxy.golang.org/<module>/@v/list` API (one call per versioned path), parses all versions (unlimited)
   - **Ultimate fallback**: Sequential `go list -m -json <package>/v<N+1>@latest` queries only if both APIs fail (rare)
   - Resolves vanity imports (k8s.io → kubernetes) before querying
3. **github_client.go**: Fetches changelogs from GitHub using go-github API (tries CHANGELOG.md, CHANGES.md, HISTORY.md on main/master, falls back to Releases API with specific release tag links)
4. **analyzer.go**: Uses native Go filepath.Walk to find affected .go files importing the package, extracts components from file paths
5. **report.go**: Generates Markdown reports sorted by version jump magnitude (descending)

### Key Data Structures

- **Dependency**: Represents a Go module with version info (Path, BasePath, CurrentVer, IsDirect, IsReplaced, CurrentMajor)
- **UpgradeCandidate**: Dependency + available versions + changelog + impact analysis
- **ImpactAnalysis**: Files affected, components (central, sensor, roxctl, etc.)

### Component Detection

The analyzer extracts top-level component names from file paths (StackRox-specific):
- Known components: central, sensor, roxctl, scanner, operator, migrator, ui, pkg, generated, tools, qa-tests-backend
- Critical components: central, sensor

### Report Sorting

Upgrades are sorted by:
1. Version jump magnitude (descending) - bigger jumps listed first
2. Package path (alphabetically)

## Environment Variables

- `GITHUB_TOKEN`: GitHub personal access token (increases rate limit from 60/hr to 5000/hr)
- `GOPROXY`: Custom Go module proxy URL (default: https://proxy.golang.org)
  - Supports comma-separated list of proxies (e.g., "https://goproxy.io,https://proxy.golang.org,direct")
  - First HTTP(S) proxy in the list is used
- `GOPRIVATE`: Comma-separated glob patterns for private modules
  - Private modules skip proxy lookups
  - Example: `GOPRIVATE=github.com/myorg/*,*.internal.company.com`

## Important Notes

- **GitHub packages**: Fast single API call to fetch all tags and find all major versions (no limit)
- **Non-GitHub packages**: Uses Go module proxy API (`proxy.golang.org/@v/list`) to fetch all versions in one call per major version path (no limit)
- **Sequential fallback**: Only used if both GitHub and proxy APIs fail (extremely rare)
- **Vanity imports**: Automatically resolved (k8s.io, go.uber.org, golang.org/x, etc.)
- **Works with any Go package**: helm.sh/helm, custom domains, everything in the Go ecosystem
- Changelog fetching supports **GitHub-hosted** packages (including vanity imports)
- Impact analysis uses **native Go filepath.Walk** for cross-platform compatibility
- Packages with `replace` directives are automatically excluded (unless `-include-replaced` is set)
- Progress bar shows live updates with package names aligned to 40 characters

## When Modifying Code

- If changing version detection logic, update `parser.go` (versionSuffixRegex pattern) and `version_checker.go` (semverRegex pattern)
- If adding new component types, update `extractComponent()` in `analyzer.go`
- If modifying report sorting, update `GenerateBasicReport()` in `report.go`
- Breaking change extraction patterns are in `github_client.go:extractBreakingChanges()`
- Vanity import mappings are in `github_client.go:resolveVanityImport()`
