package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-github/v60/github"
	"github.com/schollz/progressbar/v3"
)

var (
	// goProxyURL is determined from GOPROXY environment variable or defaults to official proxy
	goProxyURL = getGoProxyURL()
)

// getGoProxyURL returns the Go proxy URL from environment or default
func getGoProxyURL() string {
	// Check GOPROXY environment variable
	if proxy := os.Getenv("GOPROXY"); proxy != "" {
		// GOPROXY can be a comma-separated list, take the first non-"direct" entry
		proxies := strings.Split(proxy, ",")
		for _, p := range proxies {
			p = strings.TrimSpace(p)
			if p != "direct" && p != "off" && strings.HasPrefix(p, "http") {
				return p
			}
		}
	}
	// Default to official Go proxy
	return "https://proxy.golang.org"
}

// isPrivateModule checks if a module path matches GOPRIVATE patterns
func isPrivateModule(modulePath string) bool {
	goprivate := os.Getenv("GOPRIVATE")
	if goprivate == "" {
		return false
	}

	// GOPRIVATE is a comma-separated list of glob patterns
	patterns := strings.Split(goprivate, ",")
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}

		// Simple glob matching (* matches any characters)
		if matchGlob(modulePath, pattern) {
			return true
		}
	}
	return false
}

// matchGlob performs simple glob pattern matching
func matchGlob(path, pattern string) bool {
	// Convert glob pattern to simple prefix/suffix/contains matching
	if pattern == "*" {
		return true
	}

	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		// *foo* - contains
		substr := strings.Trim(pattern, "*")
		return strings.Contains(path, substr)
	}

	if strings.HasPrefix(pattern, "*") {
		// *foo - suffix
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(path, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		// foo* - prefix
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}

	// Exact match
	return path == pattern
}

var (
	// Regex to parse semantic version tags (v2.3.4, v3.0.0, etc.)
	semverRegex = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)`)
)

// UpgradeCandidate represents a package that has available major version upgrades
type UpgradeCandidate struct {
	Dependency
	AvailableVersions []AvailableVersion
	Changelog         *ChangelogInfo
	Impact            *ImpactAnalysis
	Archived          bool // Whether the repository is archived/deprecated
}

// ArchivedDependency represents a dependency that is archived but has no major version upgrades
type ArchivedDependency struct {
	Dependency
}

// AvailableVersion represents a newer major version that's available
type AvailableVersion struct {
	MajorVer    string     // e.g., "v5"
	FullVersion string     // e.g., "v5.4.0"
	Major       int        // Numeric: 5
	ReleasedAt  *time.Time // When this version was released
}

// moduleInfo is the JSON structure returned by `go list -m -json`
type moduleInfo struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Error   string `json:"Error"`
}

// fetchVersionDateFromProxy fetches the release date for a specific version from the Go module proxy
func fetchVersionDateFromProxy(modulePath, version string) *time.Time {
	// Skip proxy for private modules
	if isPrivateModule(modulePath) {
		return nil
	}

	// The .info endpoint provides timestamp information
	url := fmt.Sprintf("%s/%s/@v/%s.info", goProxyURL, modulePath, version)

	// Use a short timeout to avoid hanging
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	// Parse JSON response
	var info struct {
		Version string    `json:"Version"`
		Time    time.Time `json:"Time"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil
	}

	return &info.Time
}

// CheckForUpgrades queries for newer major versions using GitHub API if possible,
// falls back to sequential proxy queries for non-GitHub packages
func CheckForUpgrades(dep Dependency) (*UpgradeCandidate, error) {
	// Try to get current version's release date from Go proxy (with timeout)
	// This is done in a non-blocking way
	if dep.CurrentReleasedAt == nil {
		// Only fetch for standard paths, not complex multi-module repos
		if !strings.Contains(dep.Path, "aws-sdk-go") {
			if date := fetchVersionDateFromProxy(dep.Path, dep.CurrentFull); date != nil {
				dep.CurrentReleasedAt = date
			}
		}
	}

	// Try GitHub API first (faster, more accurate)
	resolvedPath := resolveVanityImport(dep.BasePath)
	if strings.HasPrefix(resolvedPath, "github.com/") {
		versions, currentDate, err := checkUpgradesViaGitHub(dep, resolvedPath)
		if err == nil && len(versions) > 0 {
			// Update dependency with current version's release date if we got it from GitHub
			if currentDate != nil && dep.CurrentReleasedAt == nil {
				dep.CurrentReleasedAt = currentDate
			}

			// Validate the latest version exists in Go proxy and get its date
			if len(versions) > 0 {
				latest := &versions[len(versions)-1]

				// Determine the correct module path for this version
				// v0 and v1: use base path
				// v2+: use base path + /vN
				versionPath := dep.BasePath
				if latest.Major >= 2 {
					versionPath = fmt.Sprintf("%s/v%d", dep.BasePath, latest.Major)
				}

				// Try to get the version date from the proxy
				if date := fetchVersionDateFromProxy(versionPath, latest.FullVersion); date != nil {
					latest.ReleasedAt = date

					// Check if the "upgrade" is actually a downgrade (older than current)
					// This happens with packages like k8s.io/client-go that use v0.x.y forever
					if dep.CurrentReleasedAt != nil && latest.ReleasedAt.Before(*dep.CurrentReleasedAt) {
						// This "upgrade" is actually older than current, skip it
						// Fall back to proxy method to find actually newer versions
						goto useFallback
					}
				} else {
					// Latest version doesn't exist in proxy, it's likely invalid
					goto useFallback
				}
			}

			return &UpgradeCandidate{
				Dependency:        dep,
				AvailableVersions: versions,
			}, nil
		}
		// If GitHub API fails, fall through to proxy method
	}

useFallback:
	// Fallback: sequential proxy queries (for non-GitHub packages or if API fails)
	return checkUpgradesViaProxy(dep)
}

// checkUpgradesViaGitHub fetches all releases and tags from GitHub to find available major versions
// Returns (available versions, current version release date, error)
func checkUpgradesViaGitHub(dep Dependency, githubPath string) ([]AvailableVersion, *time.Time, error) {
	parts := strings.Split(githubPath, "/")
	if len(parts) < 3 {
		return nil, nil, fmt.Errorf("invalid GitHub path")
	}
	owner, repo := parts[1], parts[2]

	gc := NewGitHubClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Group versions by major version with release dates
	type versionInfo struct {
		version string
		date    *time.Time
	}
	majorVersions := make(map[int]versionInfo) // major -> latest version info
	var currentVersionDate *time.Time          // Release date of current version

	// First, try fetching releases (which have reliable dates)
	releases, _, err := gc.client.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{
		PerPage: 100,
	})
	if err == nil {
		for _, release := range releases {
			tagName := release.GetTagName()

			// Skip +incompatible versions (pre-module pseudo-versions)
			if strings.Contains(tagName, "+incompatible") {
				continue
			}

			matches := semverRegex.FindStringSubmatch(tagName)
			if len(matches) < 2 {
				continue
			}

			major, err := strconv.Atoi(matches[1])
			if err != nil {
				continue
			}

			// Get release date
			releaseDate := release.PublishedAt.GetTime()

			// Check if this is the current version
			if tagName == dep.CurrentFull && currentVersionDate == nil {
				currentVersionDate = releaseDate
			}

			// Track upgrade versions
			if major <= dep.CurrentMajor {
				continue
			}

			// Keep the latest version for each major
			if existing, exists := majorVersions[major]; !exists || tagName > existing.version {
				majorVersions[major] = versionInfo{
					version: tagName,
					date:    releaseDate,
				}
			}
		}
	}

	// If releases didn't cover all versions, also check tags (but they won't have dates)
	tags, _, err := gc.client.Repositories.ListTags(ctx, owner, repo, &github.ListOptions{
		PerPage: 100,
	})
	if err == nil {
		for _, tag := range tags {
			tagName := tag.GetName()

			// Skip +incompatible versions (pre-module pseudo-versions)
			if strings.Contains(tagName, "+incompatible") {
				continue
			}

			matches := semverRegex.FindStringSubmatch(tagName)
			if len(matches) < 2 {
				continue
			}

			major, err := strconv.Atoi(matches[1])
			if err != nil || major <= dep.CurrentMajor {
				continue
			}

			// Only add if we don't already have this major version from releases
			if _, exists := majorVersions[major]; !exists {
				majorVersions[major] = versionInfo{
					version: tagName,
					date:    nil, // Tags don't have reliable dates without extra API calls
				}
			}
		}
	}

	if len(majorVersions) == 0 {
		return nil, currentVersionDate, fmt.Errorf("no versions found")
	}

	// Convert to sorted list with release dates
	var availableVersions []AvailableVersion
	for major := dep.CurrentMajor + 1; major <= dep.CurrentMajor+100; major++ {
		if info, exists := majorVersions[major]; exists {
			availableVersions = append(availableVersions, AvailableVersion{
				MajorVer:    fmt.Sprintf("v%d", major),
				FullVersion: info.version,
				Major:       major,
				ReleasedAt:  info.date,
			})
		}
	}

	return availableVersions, currentVersionDate, nil
}

// checkUpgradesViaProxy uses Go module proxy list API (for non-GitHub packages)
func checkUpgradesViaProxy(dep Dependency) (*UpgradeCandidate, error) {
	// Try to get all versions from the module proxy first
	versions, err := fetchVersionsFromProxy(dep.BasePath)
	if err == nil && len(versions) > 0 {
		return processProxyVersions(dep, versions), nil
	}

	// If proxy list fails, fall back to sequential queries
	return checkUpgradesViaSequentialQueries(dep)
}

// fetchVersionsFromProxy fetches all versions from the Go module proxy
func fetchVersionsFromProxy(basePath string) ([]string, error) {
	// For packages with major version suffixes, we need to check multiple paths
	// For example, for k8s.io/api/v2, we need to check both:
	// - k8s.io/api/@v/list (for v0, v1)
	// - k8s.io/api/v2/@v/list (for v2)
	// - k8s.io/api/v3/@v/list (for v3), etc.

	allVersions := make(map[string]bool)

	// Try base path first (covers v0, v1, and packages without version suffixes)
	if versions, err := fetchVersionListFromProxy(basePath); err == nil {
		for _, v := range versions {
			allVersions[v] = true
		}
	}

	// Try versioned paths (v2, v3, v4, ... v10)
	// We limit to v10 to avoid too many requests
	for major := 2; major <= 10; major++ {
		versionedPath := fmt.Sprintf("%s/v%d", basePath, major)
		if versions, err := fetchVersionListFromProxy(versionedPath); err == nil {
			for _, v := range versions {
				allVersions[v] = true
			}
		}
	}

	// Convert map to slice
	var result []string
	for v := range allVersions {
		result = append(result, v)
	}

	return result, nil
}

// fetchVersionListFromProxy fetches version list for a specific module path
func fetchVersionListFromProxy(modulePath string) ([]string, error) {
	// Skip proxy for private modules
	if isPrivateModule(modulePath) {
		return nil, fmt.Errorf("private module (GOPRIVATE)")
	}

	url := fmt.Sprintf("%s/%s/@v/list", goProxyURL, modulePath)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy returned status %d", resp.StatusCode)
	}

	var versions []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		version := strings.TrimSpace(scanner.Text())
		if version != "" {
			versions = append(versions, version)
		}
	}

	return versions, scanner.Err()
}

// processProxyVersions parses versions from proxy and finds major upgrades
func processProxyVersions(dep Dependency, versions []string) *UpgradeCandidate {
	majorVersions := make(map[int]string) // major -> latest full version

	for _, version := range versions {
		// Skip +incompatible versions (pre-module pseudo-versions)
		if strings.Contains(version, "+incompatible") {
			continue
		}

		matches := semverRegex.FindStringSubmatch(version)
		if len(matches) < 2 {
			continue
		}

		major, err := strconv.Atoi(matches[1])
		if err != nil || major <= dep.CurrentMajor {
			continue
		}

		// Keep the highest version for each major (simple string comparison works for semver)
		if existing, ok := majorVersions[major]; !ok || version > existing {
			majorVersions[major] = version
		}
	}

	if len(majorVersions) == 0 {
		return nil
	}

	// Convert to list and fetch release dates from proxy
	var availableVersions []AvailableVersion
	for major := dep.CurrentMajor + 1; major <= dep.CurrentMajor+100; major++ {
		if version, exists := majorVersions[major]; exists {
			// Determine the correct module path for this version
			versionPath := dep.BasePath
			if major >= 2 {
				versionPath = fmt.Sprintf("%s/v%d", dep.BasePath, major)
			}

			// Fetch release date from proxy
			releaseDate := fetchVersionDateFromProxy(versionPath, version)

			availableVersions = append(availableVersions, AvailableVersion{
				MajorVer:    fmt.Sprintf("v%d", major),
				FullVersion: version,
				Major:       major,
				ReleasedAt:  releaseDate,
			})
		}
	}

	return &UpgradeCandidate{
		Dependency:        dep,
		AvailableVersions: availableVersions,
	}
}

// checkUpgradesViaSequentialQueries is the old fallback method using go list
func checkUpgradesViaSequentialQueries(dep Dependency) (*UpgradeCandidate, error) {
	var availableVersions []AvailableVersion
	nextMajor := dep.CurrentMajor + 1

	// Limit to 20 sequential queries as fallback
	for i := range 20 {
		checkVer := nextMajor + i
		modulePath := fmt.Sprintf("%s/v%d", dep.BasePath, checkVer)

		version, found, err := queryVersion(modulePath)
		if err != nil || !found {
			break
		}

		availableVersions = append(availableVersions, AvailableVersion{
			MajorVer:    fmt.Sprintf("v%d", checkVer),
			FullVersion: version,
			Major:       checkVer,
		})
	}

	if len(availableVersions) == 0 {
		return nil, nil
	}

	return &UpgradeCandidate{
		Dependency:        dep,
		AvailableVersions: availableVersions,
	}, nil
}

// queryVersion queries the Go module proxy for the latest version of a module
// Returns (version, found, error)
func queryVersion(modulePath string) (string, bool, error) {
	cmd := exec.Command("go", "list", "-m", "-json", modulePath+"@latest")
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if it's a "not found" error
		outputStr := string(output)
		if strings.Contains(outputStr, "404") ||
			strings.Contains(outputStr, "not found") ||
			strings.Contains(outputStr, "no matching versions") {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to query %s: %w (output: %s)", modulePath, err, outputStr)
	}

	var info moduleInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return "", false, fmt.Errorf("failed to parse JSON for %s: %w", modulePath, err)
	}

	if info.Error != "" {
		// Module exists but has an error - treat as not found
		return "", false, nil
	}

	return info.Version, true, nil
}

// CheckAllUpgrades finds upgrade candidates for all dependencies in parallel
func CheckAllUpgrades(deps []Dependency, showProgress bool) ([]UpgradeCandidate, error) {
	var bar *progressbar.ProgressBar

	// Show progress bar when requested
	if showProgress {
		bar = progressbar.NewOptions(len(deps),
			progressbar.OptionSetDescription("Checking for upgrades"),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(15),
			progressbar.OptionThrottle(100),
			progressbar.OptionShowIts(),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
		)
	}

	// Use worker pool pattern for parallel processing
	// Scale workers based on CPU cores (2x cores for I/O bound operations)
	maxWorkers := max(runtime.NumCPU()*2, 4)

	type result struct {
		candidate *UpgradeCandidate
		err       error
		dep       Dependency
	}

	results := make(chan result, len(deps))
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	// Launch workers for each dependency
	for _, dep := range deps {
		wg.Go(func() {
			// Acquire semaphore (limit concurrent workers)
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Update progress bar (thread-safe)
			if bar != nil {
				parts := strings.Split(dep.Path, "/")
				pkgName := dep.Path
				if len(parts) >= 2 {
					pkgName = parts[len(parts)-2] + "/" + parts[len(parts)-1]
				}
				const maxWidth = 40
				if len(pkgName) > maxWidth {
					pkgName = pkgName[:maxWidth-3] + "..."
				} else {
					pkgName = fmt.Sprintf("%-*s", maxWidth, pkgName)
				}
				bar.Describe(fmt.Sprintf("Checking %s", pkgName))
			}

			// Check for upgrades
			candidate, err := CheckForUpgrades(dep)

			// Send result
			results <- result{candidate: candidate, err: err, dep: dep}

			// Update progress
			if bar != nil {
				_ = bar.Add(1)
			}
		})
	}

	// Close results channel when all workers complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var candidates []UpgradeCandidate
	for res := range results {
		if res.err != nil {
			// Log error but continue with other dependencies
			if bar == nil {
				fmt.Printf("Warning: failed to check upgrades for %s: %v\n", res.dep.Path, res.err)
			}
			continue
		}

		if res.candidate != nil {
			candidates = append(candidates, *res.candidate)
		}
	}

	return candidates, nil
}

// CheckAllUpgradesWithCache finds upgrade candidates for all dependencies with caching support
func CheckAllUpgradesWithCache(deps []Dependency, cache *Cache, showProgress bool) ([]UpgradeCandidate, error) {
	var bar *progressbar.ProgressBar

	// Show progress bar when requested
	if showProgress {
		bar = progressbar.NewOptions(len(deps),
			progressbar.OptionSetDescription("Checking for upgrades"),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(15),
			progressbar.OptionThrottle(100),
			progressbar.OptionShowIts(),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
		)
	}

	// Use worker pool pattern for parallel processing
	maxWorkers := max(runtime.NumCPU()*2, 4)

	type result struct {
		candidate *UpgradeCandidate
		err       error
		dep       Dependency
		cached    bool
	}

	results := make(chan result, len(deps))
	semaphore := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	githubClient := NewGitHubClient()

	// Launch workers for each dependency
	for _, dep := range deps {
		wg.Go(func() {
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Update progress bar
			if bar != nil {
				parts := strings.Split(dep.Path, "/")
				pkgName := dep.Path
				if len(parts) >= 2 {
					pkgName = parts[len(parts)-2] + "/" + parts[len(parts)-1]
				}
				const maxWidth = 40
				if len(pkgName) > maxWidth {
					pkgName = pkgName[:maxWidth-3] + "..."
				} else {
					pkgName = fmt.Sprintf("%-*s", maxWidth, pkgName)
				}
				bar.Describe(fmt.Sprintf("Checking %s", pkgName))
			}

			var candidate *UpgradeCandidate
			cached := false

			// Try cache first
			if cache != nil && cache.enabled {
				entry, found, err := cache.Get(dep)
				if err == nil && found {
					// Restore from cache
					candidate = &UpgradeCandidate{
						Dependency:        entry.Dependency,
						AvailableVersions: entry.Versions,
						Changelog:         entry.ChangelogInfo,
						Impact:            entry.ImpactAnalysis,
						Archived:          entry.Archived,
					}
					cached = true
				}
			}

			// If not in cache, fetch fresh data
			if !cached {
				var err error
				candidate, err = CheckForUpgrades(dep)
				if err != nil {
					results <- result{candidate: nil, err: err, dep: dep, cached: false}
					if bar != nil {
						_ = bar.Add(1)
					}
					return
				}

				// If we found upgrades, fetch changelog and check if archived
				if candidate != nil {
					// Check if repository is archived
					repoStatus, err := githubClient.CheckRepoStatus(candidate.BasePath)
					if err == nil && repoStatus != nil {
						candidate.Archived = repoStatus.Archived
					}

					// Fetch changelog
					changelog, err := githubClient.FetchChangelog(candidate.BasePath)
					if err == nil {
						candidate.Changelog = changelog
					}

					// Analyze impact (simplified - just mark as not analyzed for now)
					// The main function will do full analysis if needed
					candidate.Impact = &ImpactAnalysis{
						FilesAffected: 0,
						Components:    []string{},
					}

					// Store in cache
					if cache != nil && cache.enabled {
						_ = cache.Set(dep, candidate.AvailableVersions, candidate.Changelog, candidate.Impact, candidate.Archived)
					}
				}
			}

			// Send result
			results <- result{candidate: candidate, err: nil, dep: dep, cached: cached}

			// Update progress
			if bar != nil {
				_ = bar.Add(1)
			}
		})
	}

	// Close results channel when all workers complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var candidates []UpgradeCandidate
	cacheHits := 0
	for res := range results {
		if res.err != nil {
			// Log error but continue
			if bar == nil {
				fmt.Printf("Warning: failed to check upgrades for %s: %v\n", res.dep.Path, res.err)
			}
			continue
		}

		if res.candidate != nil {
			candidates = append(candidates, *res.candidate)
			if res.cached {
				cacheHits++
			}
		}
	}

	if cache != nil && cache.enabled && cacheHits > 0 {
		fmt.Fprintf(os.Stderr, "Cache hits: %d/%d\n", cacheHits, len(deps))
	}

	return candidates, nil
}

// VersionJump returns the number of major versions between current and latest available
func (uc UpgradeCandidate) VersionJump() int {
	if len(uc.AvailableVersions) == 0 {
		return 0
	}
	latest := uc.AvailableVersions[len(uc.AvailableVersions)-1]
	return latest.Major - uc.CurrentMajor
}

// LatestAvailable returns the newest available version
func (uc UpgradeCandidate) LatestAvailable() AvailableVersion {
	if len(uc.AvailableVersions) == 0 {
		return AvailableVersion{}
	}
	return uc.AvailableVersions[len(uc.AvailableVersions)-1]
}

// CheckArchivedDependencies checks all dependencies for archived status
// Returns only dependencies that are archived but have no upgrades available
func CheckArchivedDependencies(allDeps []Dependency, candidates []UpgradeCandidate) []ArchivedDependency {
	// Create a map of dependencies that already have upgrades (use full Path)
	hasUpgrades := make(map[string]bool)
	for _, c := range candidates {
		hasUpgrades[c.Path] = true
	}

	var archivedDeps []ArchivedDependency
	githubClient := NewGitHubClient()

	// Check each dependency that doesn't have upgrades
	for _, dep := range allDeps {
		// Skip if this specific dependency version already has an upgrade candidate
		if hasUpgrades[dep.Path] {
			continue
		}

		// Check if repository is archived
		status, err := githubClient.CheckRepoStatus(dep.BasePath)
		if err != nil {
			// Not a GitHub repo or error - skip
			continue
		}

		if status.Archived {
			archivedDeps = append(archivedDeps, ArchivedDependency{
				Dependency: dep,
			})
		}
	}

	return archivedDeps
}
