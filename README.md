# Go Dependency Major Version Upgrade Checker

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Test Coverage](https://img.shields.io/badge/coverage-56.6%25-brightgreen.svg)]()
[![Go Report Card](https://goreportcard.com/badge/github.com/stackrox/go-dep-upgrade-checker)](https://goreportcard.com/report/github.com/stackrox/go-dep-upgrade-checker)

A modern Go tool that automatically discovers major version upgrade opportunities, detects archived/deprecated dependencies, shows release dates and ages, fetches changelogs from GitHub, analyzes codebase impact, and generates actionable reports with live progress tracking.

## ✨ Key Features

- **🔍 Comprehensive Discovery** - Finds all major version upgrades for your Go dependencies
- **🗄️ Archived Detection** - Warns about deprecated/archived repositories (even when no upgrades exist)
- **📅 Release Dates & Ages** - Shows when versions were released and how old they are
- **⚡ Parallel Execution** - Worker pool scaled to 2x CPU cores for maximum speed
- **🌐 Vanity Import Support** - Resolves k8s.io, go.uber.org, golang.org/x, etc.
- **📊 Impact Analysis** - Identifies affected files and components in your codebase
- **💾 Smart Caching** - 24-hour cache for faster repeat runs
- **🔐 GOPROXY/GOPRIVATE** - Supports custom proxies and private modules
- **📈 Live Progress** - Real-time progress bar with package names
- **🔗 Direct Links** - Points to specific release tags, not generic pages

## 📦 Installation

### Quick Run (No Installation)

```bash
go run github.com/stackrox/go-dep-upgrade-checker@latest
```

### Install Binary

```bash
go install github.com/stackrox/go-dep-upgrade-checker@latest
dep-upgrade-checker -output upgrade-report.md
```

### Build from Source

```bash
git clone https://github.com/stackrox/go-dep-upgrade-checker.git
cd go-dep-upgrade-checker
go build -o dep-upgrade-checker .
./dep-upgrade-checker -output report.md
```

## 🚀 Quick Start

### Basic Usage

```bash
# Generate report for your project
dep-upgrade-checker -output upgrade-report.md

# Check specific package
dep-upgrade-checker -package github.com/google/go-github/v50

# Include indirect dependencies
dep-upgrade-checker -include-indirect -output full-report.md

# With GitHub token (recommended - increases rate limit 60/hr → 5000/hr)
GITHUB_TOKEN=ghp_xxx dep-upgrade-checker -output report.md
```

### Custom Proxy

```bash
# Use custom Go proxy
GOPROXY=https://goproxy.io dep-upgrade-checker -output report.md

# Skip private modules
GOPRIVATE=github.com/mycompany/* dep-upgrade-checker -output report.md
```

### Cache Management

```bash
# Clear cache
dep-upgrade-checker -clear-cache

# Show cache statistics
dep-upgrade-checker -cache-stats

# Disable cache
dep-upgrade-checker -no-cache -output report.md
```

## 📊 Example Output

### Command Line Output
```bash
$ dep-upgrade-checker -output upgrade-report.md

Parsing go.mod...
Found 3 versioned dependencies
After filtering: 3 dependencies to check
Cache enabled (24h TTL)
Checking for available upgrades...
Checking go-github/v50                              100% |███████████████| (3/3, 32 it/min)
Found 2 upgrade candidates
Checking for archived dependencies...
Found 1 archived dependencies without upgrades
Analyzing codebase impact...
Generating report...
Report written to upgrade-report.md
```

### Generated Report (upgrade-report.md)
```markdown
# Dependency Major Version Upgrade Analysis

## Summary
- Upgrade candidates: 2
- ⚠️  Archived dependencies: 2

## Available Upgrades

### google/go-github: v50 → v84 (34 major versions)

**Current**: v50.0.0 (released 2023-01-26, 3 years, 1 month old)
**Latest**: v84.0.0 (released 2026-02-27)
**Impact**: 5 files affected (tools)
**Repository**: https://github.com/google/go-github
**Changelog**: https://github.com/google/go-github/releases/tag/v84.0.0
**Breaking Changes**:
- CHANGE: `CreateWorkflowDispatchEventByID` now returns `*WorkflowDispatchRunDetails`
- CHANGE: Split `IssuesService.List` into `ListAllIssues` and `ListUserIssues`

### mitchellh/hashstructure: v1 → v2 (1 major version)

**🗄️  ARCHIVED**: This repository is archived and no longer maintained

**Current**: v1.1.0 (released 2020-11-22, 5 years, 3 months old)
**Latest**: v2.0.2 (released 2021-05-27)
**Impact**: 3 files affected (central)
**Repository**: https://github.com/mitchellh/hashstructure

## Archived Dependencies (No Upgrades Available)

The following dependencies are archived and no longer maintained. You are currently on the latest version, but you should consider migrating to actively maintained alternatives.

### mitchellh/hashstructure: v2.0.2

**🗄️  ARCHIVED**: This repository is archived and no longer maintained

**Current Version**: v2.0.2
**Repository**: https://github.com/mitchellh/hashstructure
```

<details>
<summary>📸 Click to see cached run example (much faster!)</summary>

```bash
$ dep-upgrade-checker -output upgrade-report.md

Parsing go.mod...
Found 3 versioned dependencies
After filtering: 3 dependencies to check
Cache enabled (24h TTL)
Checking for available upgrades...
Checking go-github/v50                              100% |███████████████| (3/3, 1805 it/s)
Cache hits: 3/3
Found 2 upgrade candidates
Checking for archived dependencies...
Analyzing codebase impact...
Generating report...
Report written to upgrade-report.md
```

Notice the speed difference: **32 it/min** → **1805 it/s** (cached)!

</details>

## 🎯 Problem Solved

Modern Go projects have hundreds of dependencies with major version suffixes (v2, v3, v4+). While tools like Dependabot handle patch/minor updates, **no tooling exists to systematically discover major version upgrades and detect archived dependencies**.

### Without This Tool
- 🔍 Manual, tedious discovery of upgrade opportunities
- ⚠️ No warning when dependencies become archived/deprecated
- 📅 No visibility into how old your dependencies are
- 📊 No insight into breaking changes before upgrading
- 🎯 Difficulty prioritizing which upgrades to tackle first

### With This Tool
- ✅ Automatic discovery of all major version upgrades
- ✅ Immediate alerts for archived/deprecated dependencies
- ✅ Clear visibility into dependency ages (e.g., "5 years, 3 months old")
- ✅ Extracted breaking changes from changelogs
- ✅ Impact analysis showing affected files and components
- ✅ Sorted by priority (version jump magnitude)

## 🔧 CLI Options

| Flag | Default | Description |
|------|---------|-------------|
| `-gomod` | `go.mod` | Path to go.mod file |
| `-output` | stdout | Output file path (Markdown) |
| `-package` | - | Filter by specific package path |
| `-include-indirect` | `false` | Include indirect dependencies |
| `-include-replaced` | `false` | Include replaced dependencies |
| `-no-cache` | `false` | Disable caching |
| `-clear-cache` | - | Clear cache and exit |
| `-cache-stats` | - | Show cache statistics and exit |

## ⚙️ How It Works

### 1. Parse go.mod
Uses `golang.org/x/mod/modfile` to extract ALL dependencies:
- Packages with version suffixes (`/v2`, `/v3`, `/v4+`)
- Packages without suffixes (v1.x.x or v0.x.x)
- Infers major version from version string

### 2. Parallel Version Checking
Worker pool scaled to **2x CPU cores** (minimum 4):

**GitHub packages (fast - ~0.5s per package):**
- One API call to fetch all tags
- Parses semantic versions instantly
- No iteration limits

**Other packages (Go proxy - ~1-2s):**
- Fetches from `proxy.golang.org/@v/list`
- Checks all major version paths
- Falls back to sequential queries if needed

**Performance**: With 8 cores, checks up to 16 packages simultaneously!

### 3. Archived Repository Detection
- Checks GitHub API for archived status
- Warns even when on latest version
- Helps identify migration needs early

### 4. Release Date Fetching
- GitHub Releases API for dates
- Go module proxy for timestamps
- Human-readable age calculation
- Downgrade detection (warns if "upgrade" is older)

### 5. Vanity Import Resolution
Automatically resolves to GitHub repositories:
- `k8s.io/*` → `github.com/kubernetes/*`
- `sigs.k8s.io/*` → `github.com/kubernetes-sigs/*`
- `go.uber.org/*` → `github.com/uber-go/*`
- `golang.org/x/*` → `github.com/golang/*`
- `google.golang.org/grpc` → `github.com/grpc/grpc-go`

### 6. Changelog & Breaking Changes
- Fetches `CHANGELOG.md` from main/master
- Falls back to GitHub Releases
- Extracts breaking changes automatically
- Links directly to specific release tags

### 7. Impact Analysis
Native Go file walking to find affected files:
- Counts files importing the package
- Identifies components (central, sensor, etc.)
- No external dependencies

### 8. Smart Caching
- 24-hour TTL for version data
- Stores changelog and impact analysis
- Skips unchanged dependencies
- Cache statistics available

## 🏗️ Architecture

Built with modern Go 1.25 patterns:

```
go-dep-upgrade-checker/
├── main.go                    # CLI entry point
├── parser.go                  # Parse go.mod (golang.org/x/mod)
├── version_checker.go         # Parallel version queries with wg.Go()
├── github_client.go           # GitHub API (go-github)
├── analyzer.go                # Native Go file walking
├── report.go                  # Markdown generation
├── cache.go                   # Smart caching (24h TTL)
└── *_test.go                  # Test suite (56.6% coverage)
```

**Modern Go Features Used:**
- `wg.Go()` for cleaner goroutine management (Go 1.25)
- `max()` builtin for min/max calculations (Go 1.21)
- `for i := range n` for cleaner loops (Go 1.22)
- `strings.CutPrefix` for efficient string ops (Go 1.20)
- `strings.SplitSeq` for memory-efficient iteration (Go 1.24)

## 📈 Performance

- **Parallel execution**: 10-15x faster than sequential
- **Smart caching**: Repeat runs are near-instant
- **Memory efficient**: `SplitSeq` reduces allocations
- **Optimized**: Uses compiler built-ins where possible

## 💡 Use Cases

### Monthly Dependency Audits
```bash
# Add to CI/cron
dep-upgrade-checker -output reports/$(date +%Y-%m)-upgrades.md
```

### Pre-Planning Major Upgrades
```bash
# Analyze specific package before starting work
dep-upgrade-checker -package helm.sh/helm/v3
```

### Detecting Technical Debt
```bash
# Find old, archived dependencies
dep-upgrade-checker -output aged-deps.md
# Review "Archived Dependencies" section
```

### CI Integration
```bash
# Fail CI if archived dependencies found
if dep-upgrade-checker | grep -q "ARCHIVED"; then
  echo "Found archived dependencies!"
  exit 1
fi
```

## 🚧 Limitations

- **GitHub-only changelogs**: Only fetches from GitHub-hosted packages
- **Pattern-based analysis**: Simple string matching for breaking changes
- **Rate limits**: GitHub API (use `GITHUB_TOKEN` for higher limits)
- **No auto-upgrade**: Generates reports only, doesn't modify code

## 🤝 Contributing

Contributions welcome! This project uses modern Go 1.25 patterns.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

**Before submitting:**
- Run `go test ./...` to ensure tests pass
- Run `go build` to ensure it compiles
- Use modern Go patterns (see CLAUDE.md for guidelines)

## 🎯 Future Enhancements

- [ ] GitHub Actions integration example
- [ ] Interactive mode for selecting upgrades
- [ ] Dependency graph visualization
- [ ] Support for GitLab/Bitbucket
- [ ] Historical tracking over time
- [ ] Auto-generate PRs for upgrades
- [ ] Security vulnerability correlation

## 📄 License

Apache License 2.0 - see [LICENSE](LICENSE) file for details.

## 🙏 Credits

Originally developed for [StackRox](https://github.com/stackrox/stackrox) to manage 179 direct and 353 indirect dependencies with modern Go 1.25 patterns.

## 🔗 Related Tools

- [Dependabot](https://github.com/dependabot/dependabot-core) - Automated patch/minor updates
- [Renovate](https://github.com/renovatebot/renovate) - Automated dependency updates
- [go mod](https://go.dev/ref/mod) - Official Go module management
- [govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) - Security vulnerability scanner

---

**⭐ If this tool helps you manage your dependencies, please give it a star!**

*Built with ❤️ using modern Go 1.25 patterns*
