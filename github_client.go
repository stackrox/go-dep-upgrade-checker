package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v60/github"
)

// ChangelogInfo contains changelog data fetched from GitHub
type ChangelogInfo struct {
	Found           bool
	Source          string   // "file" or "releases"
	URL             string   // Link to the changelog
	Content         string   // Full changelog content
	BreakingChanges []string // Extracted breaking changes
}

// Regex patterns for extracting breaking changes (compiled once at package level)
var (
	breakingPattern1 = regexp.MustCompile(`(?im)^[#*\s]*breaking[\s:]+(.+)$`)
	breakingPattern2 = regexp.MustCompile(`(?im)^[#*\s]*⚠️(.+)$`)
	breakingPattern3 = regexp.MustCompile(`(?im)^[#*\s]*removed[\s:]+(.+)$`)
	breakingPattern4 = regexp.MustCompile(`(?im)\[breaking\]\s*(.+)$`)
)

// GitHubClient wraps the GitHub API client
type GitHubClient struct {
	client *github.Client
	ctx    context.Context
}

// RepoStatus contains repository metadata
type RepoStatus struct {
	Archived    bool
	Description string
}

// NewGitHubClient creates a new GitHub client
// If GITHUB_TOKEN is set, it will be used for authentication (higher rate limits)
func NewGitHubClient() *GitHubClient {
	client := github.NewClient(nil)

	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		client = client.WithAuthToken(token)
	}

	return &GitHubClient{
		client: client,
		ctx:    context.Background(),
	}
}

// CheckRepoStatus checks if a repository is archived or has other important metadata
func (gc *GitHubClient) CheckRepoStatus(basePath string) (*RepoStatus, error) {
	// Resolve vanity imports to actual GitHub repositories
	resolvedPath := resolveVanityImport(basePath)

	// Extract owner and repo from GitHub path
	owner, repo, ok := extractGitHubOwnerRepo(resolvedPath)
	if !ok {
		return nil, fmt.Errorf("not a GitHub repository")
	}

	// Fetch repository information
	ctx, cancel := context.WithTimeout(gc.ctx, 10*time.Second)
	defer cancel()

	repository, _, err := gc.client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	return &RepoStatus{
		Archived:    repository.GetArchived(),
		Description: repository.GetDescription(),
	}, nil
}

// FetchChangelog attempts to fetch changelog information for a package
func (gc *GitHubClient) FetchChangelog(basePath string) (*ChangelogInfo, error) {
	// Resolve vanity imports to actual GitHub repositories
	resolvedPath := resolveVanityImport(basePath)

	// Extract owner and repo from GitHub path
	owner, repo, ok := extractGitHubOwnerRepo(resolvedPath)
	if !ok {
		return &ChangelogInfo{Found: false}, nil
	}

	// Try fetching CHANGELOG.md from repository
	info, err := gc.fetchChangelogFile(owner, repo)
	if err == nil && info.Found {
		return info, nil
	}

	// Fallback: try GitHub Releases
	info, err = gc.fetchReleases(owner, repo)
	if err == nil && info.Found {
		return info, nil
	}

	return &ChangelogInfo{Found: false}, nil
}

// fetchChangelogFile tries to fetch CHANGELOG.md from the repository
func (gc *GitHubClient) fetchChangelogFile(owner, repo string) (*ChangelogInfo, error) {
	// Try different common changelog filenames and branches
	filenames := []string{"CHANGELOG.md", "CHANGELOG", "CHANGES.md", "HISTORY.md"}
	branches := []string{"main", "master"}

	for _, branch := range branches {
		for _, filename := range filenames {
			ctx, cancel := context.WithTimeout(gc.ctx, 10*time.Second)
			fileContent, _, _, err := gc.client.Repositories.GetContents(ctx, owner, repo, filename, &github.RepositoryContentGetOptions{
				Ref: branch,
			})
			cancel()

			if err == nil && fileContent != nil {
				content, err := fileContent.GetContent()
				if err == nil && content != "" {
					return &ChangelogInfo{
						Found:           true,
						Source:          "file",
						URL:             fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", owner, repo, branch, filename),
						Content:         content,
						BreakingChanges: extractBreakingChanges(content),
					}, nil
				}
			}
		}
	}

	return &ChangelogInfo{Found: false}, nil
}

// fetchReleases fetches release notes from GitHub Releases
func (gc *GitHubClient) fetchReleases(owner, repo string) (*ChangelogInfo, error) {
	ctx, cancel := context.WithTimeout(gc.ctx, 10*time.Second)
	releases, _, err := gc.client.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{
		PerPage: 10, // Get last 10 releases
	})
	cancel()

	if err != nil {
		return &ChangelogInfo{Found: false}, err
	}

	if len(releases) == 0 {
		return &ChangelogInfo{Found: false}, nil
	}

	// Combine release notes
	var sb strings.Builder
	for _, release := range releases {
		sb.WriteString(fmt.Sprintf("## %s\n\n", release.GetTagName()))
		sb.WriteString(release.GetBody())
		sb.WriteString("\n\n")
	}

	content := sb.String()
	// Link to the latest release specifically instead of generic releases page
	releaseURL := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", owner, repo, releases[0].GetTagName())

	return &ChangelogInfo{
		Found:           true,
		Source:          "releases",
		URL:             releaseURL,
		Content:         content,
		BreakingChanges: extractBreakingChanges(content),
	}, nil
}

// resolveVanityImport resolves common vanity import paths to their GitHub equivalents
func resolveVanityImport(path string) string {
	// Common vanity import patterns
	vanityMappings := map[string]string{
		"k8s.io/":           "github.com/kubernetes/",
		"sigs.k8s.io/":      "github.com/kubernetes-sigs/",
		"go.uber.org/":      "github.com/uber-go/",
		"golang.org/x/":     "github.com/golang/",
		"google.golang.org/": "github.com/",
		"gocloud.dev/":      "github.com/google/go-cloud/",
	}

	for prefix, replacement := range vanityMappings {
		suffix, ok := strings.CutPrefix(path, prefix)
		if !ok {
			continue
		}

		// Handle special case for google.golang.org where repo name might differ
		if prefix == "google.golang.org/" {
			// google.golang.org/grpc -> github.com/grpc/grpc-go
			// google.golang.org/protobuf -> github.com/protocolbuffers/protobuf-go
			parts := strings.Split(suffix, "/")
			if len(parts) > 0 {
				switch parts[0] {
				case "grpc":
					return "github.com/grpc/grpc-go"
				case "protobuf":
					return "github.com/protocolbuffers/protobuf-go"
				}
			}
		}
		return replacement + suffix
	}

	return path
}

// extractGitHubOwnerRepo extracts owner and repo from a GitHub module path
// Returns (owner, repo, ok)
func extractGitHubOwnerRepo(path string) (string, string, bool) {
	if !strings.HasPrefix(path, "github.com/") {
		return "", "", false
	}

	parts := strings.Split(path, "/")
	if len(parts) < 3 {
		return "", "", false
	}

	return parts[1], parts[2], true
}

// extractBreakingChanges finds breaking changes in changelog content
func extractBreakingChanges(content string) []string {
	var changes []string

	// Generic section headers to skip (these aren't actual breaking changes)
	skipPatterns := []string{
		"breaking changes",
		"changes",
		"breaking",
		"removed",
		"important",
		"notes",
		"⚠️",
	}

	// Use package-level compiled patterns
	patterns := []*regexp.Regexp{
		breakingPattern1,
		breakingPattern2,
		breakingPattern3,
		breakingPattern4,
	}

	for line := range strings.SplitSeq(content, "\n") {
		for _, pattern := range patterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
				change := strings.TrimSpace(matches[1])

				// Skip if empty or too short (likely a header)
				if len(change) < 10 {
					continue
				}

				// Skip if it ends with ":**" (markdown link labels)
				if strings.HasSuffix(change, ":**") {
					continue
				}

				// Skip if it starts with emoji (likely a heading)
				if strings.HasPrefix(change, "⚠️") || strings.HasPrefix(change, "⚠") {
					continue
				}

				// Skip if it's just "Changes" followed by punctuation
				if rest, ok := strings.CutPrefix(change, "Changes:"); ok && len(rest) < 5 {
					continue
				}

				// Skip generic section headers
				changeLower := strings.ToLower(strings.TrimSpace(strings.Trim(change, "⚠️ ")))
				isGenericHeader := false
				for _, skip := range skipPatterns {
					if changeLower == skip ||
					   strings.HasPrefix(changeLower, skip+":") ||
					   strings.HasSuffix(changeLower, skip) {
						isGenericHeader = true
						break
					}
				}
				if isGenericHeader {
					continue
				}

				// Skip if it looks like a markdown header (starts with #)
				if strings.HasPrefix(change, "#") {
					continue
				}

				// Skip if it's ONLY punctuation and emojis
				trimmedChange := strings.Trim(change, " ⚠️⚠*#:-")
				if len(trimmedChange) < 5 {
					continue
				}

				changes = append(changes, change)
			}
		}
	}

	// Limit to top 5 most relevant breaking changes
	if len(changes) > 5 {
		changes = changes[:5]
	}

	return changes
}

// ExtractVersionSection extracts a specific version section from changelog
func ExtractVersionSection(content, version string) string {
	// Try to find the version header
	versionPattern := regexp.MustCompile(fmt.Sprintf(`(?mi)^#+\s*\[?%s\]?`, regexp.QuoteMeta(version)))
	lines := strings.Split(content, "\n")

	var sectionLines []string
	inSection := false
	headerLevel := 0

	for _, line := range lines {
		if versionPattern.MatchString(line) {
			inSection = true
			// Count header level (number of # characters)
			headerLevel = strings.Count(strings.SplitN(line, " ", 2)[0], "#")
			sectionLines = append(sectionLines, line)
			continue
		}

		if inSection {
			// Check if we've hit the next section header at the same or higher level
			if strings.HasPrefix(strings.TrimSpace(line), "#") {
				currentLevel := strings.Count(strings.SplitN(line, " ", 2)[0], "#")
				if currentLevel <= headerLevel {
					break
				}
			}
			sectionLines = append(sectionLines, line)
		}
	}

	if len(sectionLines) > 0 {
		result := strings.Join(sectionLines, "\n")
		// Truncate if too long
		if len(result) > 1000 {
			result = result[:1000] + "\n\n_(truncated)_"
		}
		return result
	}

	return ""
}
