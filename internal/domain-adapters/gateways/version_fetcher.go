package gateways

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

// VersionFetcher handles fetching latest versions from various sources
type VersionFetcher struct {
	httpClient *http.Client
}

// NewVersionFetcher creates a new version fetcher
func NewVersionFetcher() *VersionFetcher {
	return &VersionFetcher{
		httpClient: &http.Client{
			Timeout: 10 * time.Second, // Reasonable timeout for version checks
		},
	}
}

// FetchLatestVersion fetches the latest version based on the version.source field
func (vf *VersionFetcher) FetchLatestVersion(def *entities.Recipe) (string, error) {
	source := def.Version.Source
	if source == "" {
		return "", fmt.Errorf("version.source not specified")
	}

	var rawVersion string
	var err error
	isGitHubTag := false

	// Parse version source type
	//nolint:gocritic // ifElseChain: checking string prefixes with different logic, not suitable for switch
	if strings.HasPrefix(source, "url:") {
		url := strings.TrimPrefix(source, "url:")
		rawVersion, err = vf.fetchFromURL(url)
		if err == nil && def.Version.ExtractPattern != "" {
			// For URL sources, extract and filter all matches to find latest valid version
			rawVersion, err = vf.extractAndFilterVersion(rawVersion, def.Version.ExtractPattern, def.Version.ExcludePatterns)
			if err != nil {
				return "", fmt.Errorf("version extraction failed: %w", err)
			}
			isGitHubTag = true // Mark that filtering was done during extraction
		}
	} else if strings.HasPrefix(source, "github-release:") {
		repo := strings.TrimPrefix(source, "github-release:")
		rawVersion, err = vf.fetchGitHubRelease(repo)
	} else if strings.HasPrefix(source, "github-tag:") {
		repo := strings.TrimPrefix(source, "github-tag:")
		rawVersion, err = vf.fetchGitHubTag(repo, def.Version.ExcludePatterns)
		isGitHubTag = true // Mark that filtering was already done
	} else if strings.HasPrefix(source, "static:") {
		// Static version - just return the value after the colon (e.g., "latest", "6.0")
		rawVersion = strings.TrimPrefix(source, "static:")
	} else {
		return "", fmt.Errorf("unsupported version.source format: %s", source)
	}

	if err != nil {
		return "", err
	}

	// Extract version using regex if specified (skip if already done for URL sources)
	if def.Version.ExtractPattern != "" && !isGitHubTag {
		rawVersion, err = vf.extractVersion(rawVersion, def.Version.ExtractPattern)
		if err != nil {
			return "", fmt.Errorf("version extraction failed: %w", err)
		}
	}

	// Transform version using sed-like pattern if specified
	if def.Version.Cleanup != "" {
		rawVersion, err = vf.transformVersion(rawVersion, def.Version.Cleanup)
		if err != nil {
			return "", fmt.Errorf("version transformation failed: %w", err)
		}
	}

	// Filter out unwanted versions (pre-releases, etc.)
	// Skip filtering for github-tag since it was already done during fetch
	if def.Version.ExcludePatterns != "" && !isGitHubTag {
		if vf.shouldFilterVersion(rawVersion, def.Version.ExcludePatterns) {
			return "", fmt.Errorf("version %s filtered out by regex: %s", rawVersion, def.Version.ExcludePatterns)
		}
	}

	return strings.TrimSpace(rawVersion), nil
}

// doWithRetry executes an HTTP request with exponential backoff retry
func (vf *VersionFetcher) doWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := calculateBackoff(attempt - 1)
			time.Sleep(backoff)
		}

		resp, err = vf.httpClient.Do(req)
		if err != nil {
			// Network errors are retryable
			if attempt < maxRetries {
				continue
			}
			return nil, err
		}

		// Check rate limit before processing response
		if rateLimitErr := checkRateLimit(resp); rateLimitErr != nil {
			//nolint:errcheck,gosec // G104: Best effort close on rate limit error
			resp.Body.Close()
			// If we waited for reset, retry immediately
			if attempt < maxRetries {
				continue
			}
			return nil, rateLimitErr
		}

		// Success or non-retryable error
		if !isRetryableError(resp.StatusCode) {
			return resp, nil
		}

		// Retryable error - close body and retry
		//nolint:errcheck,gosec // G104: Best effort close before retry
		resp.Body.Close()

		if attempt < maxRetries {
			continue
		}

		// Max retries reached
		return resp, nil
	}

	return resp, err
}

// fetchFromURL fetches version from a plain URL
func (vf *VersionFetcher) fetchFromURL(url string) (string, error) {
	resp, err := vf.httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	//nolint:errcheck // Defer close
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(body), nil
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// fetchGitHubRelease fetches the latest release from GitHub
func (vf *VersionFetcher) fetchGitHubRelease(repo string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add Accept header for GitHub API
	req.Header.Set("Accept", "application/vnd.github+json")

	// Add GitHub token if available (required for higher rate limits)
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := vf.doWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	//nolint:errcheck // Defer close
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("GitHub API error %d (failed to read response)", resp.StatusCode)
		}
		return "", fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	if release.Draft {
		return "", fmt.Errorf("latest release is a draft")
	}

	return release.TagName, nil
}

// GitHubTag represents a GitHub tag
type GitHubTag struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

// fetchGitHubTag fetches the latest tag from GitHub, optionally filtering unwanted tags
func (vf *VersionFetcher) fetchGitHubTag(repo string, filterRegex string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/tags", repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	// Add GitHub token if available (required for higher rate limits)
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		token = os.Getenv("GH_TOKEN")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := vf.doWithRetry(req)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	//nolint:errcheck // Defer close
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("GitHub API error %d (failed to read response)", resp.StatusCode)
		}
		return "", fmt.Errorf("GitHub API error %d: %s", resp.StatusCode, string(body))
	}

	var tags []GitHubTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", fmt.Errorf("failed to parse GitHub response: %w", err)
	}

	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found")
	}

	// If filter regex is provided, find first tag that doesn't match filter
	if filterRegex != "" {
		for _, tag := range tags {
			if !vf.shouldFilterVersion(tag.Name, filterRegex) {
				return tag.Name, nil
			}
		}
		return "", fmt.Errorf("all tags filtered out by regex: %s", filterRegex)
	}

	// Return the first (most recent) tag
	return tags[0].Name, nil
}

// extractAndFilterVersion extracts ALL version matches and returns the latest valid one
func (vf *VersionFetcher) extractAndFilterVersion(input, pattern, excludePatterns string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	allMatches := re.FindAllStringSubmatch(input, -1)
	if len(allMatches) == 0 {
		return "", fmt.Errorf("no match found for pattern: %s", pattern)
	}

	// Collect all valid versions with their full matches
	type versionMatch struct {
		version   string
		fullMatch string
	}
	var validVersions []versionMatch

	// Extract versions and filter
	for _, matches := range allMatches {
		fullMatch := matches[0]

		// Use capture group if exists and not empty, otherwise use full match
		var version string
		if len(matches) > 1 && matches[1] != "" {
			version = matches[1]
		} else {
			version = matches[0]
		}

		// Check if this version should be filtered (check against full match)
		if excludePatterns == "" || !vf.shouldFilterVersion(fullMatch, excludePatterns) {
			validVersions = append(validVersions, versionMatch{version: version, fullMatch: fullMatch})
		}
	}

	if len(validVersions) == 0 {
		return "", fmt.Errorf("all versions filtered out by regex: %s", excludePatterns)
	}

	// Find the highest version using semantic version comparison
	latestVersion := validVersions[0].version
	for i := 1; i < len(validVersions); i++ {
		if vf.compareVersions(validVersions[i].version, latestVersion) > 0 {
			latestVersion = validVersions[i].version
		}
	}

	return latestVersion, nil
}

// compareVersions compares two version strings semantically
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func (vf *VersionFetcher) compareVersions(v1, v2 string) int {
	// Split versions by dots
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	// Compare each part numerically
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int

		if i < len(parts1) {
			// Extract numeric part (handle cases like "1rc1" -> 1)
			numStr := ""
			for _, ch := range parts1[i] {
				if ch >= '0' && ch <= '9' {
					numStr += string(ch)
				} else {
					break
				}
			}
			if numStr != "" {
				if n, err := strconv.Atoi(numStr); err == nil {
					num1 = n
				}
			}
		}

		if i < len(parts2) {
			numStr := ""
			for _, ch := range parts2[i] {
				if ch >= '0' && ch <= '9' {
					numStr += string(ch)
				} else {
					break
				}
			}
			if numStr != "" {
				if n, err := strconv.Atoi(numStr); err == nil {
					num2 = n
				}
			}
		}

		if num1 > num2 {
			return 1
		} else if num1 < num2 {
			return -1
		}
	}

	return 0
}

// extractVersion extracts version using regex
func (vf *VersionFetcher) extractVersion(input, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	matches := re.FindStringSubmatch(input)
	if len(matches) == 0 {
		return "", fmt.Errorf("no match found for pattern: %s", pattern)
	}

	// Return the first capture group if it exists AND is not empty
	// This handles patterns like: 'package-([0-9.]+)' where we want just the version
	// But also handles patterns like: '[0-9]+\.[0-9]+(\.[0-9]+)?' where we want the full match
	if len(matches) > 1 && matches[1] != "" {
		return matches[1], nil
	}

	return matches[0], nil
} // transformVersion applies sed-like transformations or simple string replacements
func (vf *VersionFetcher) transformVersion(input, sedPattern string) (string, error) {
	// Support simple "find:replace" syntax (e.g., "v:" to remove "v", "_:." to replace "_" with ".")
	if !strings.HasPrefix(sedPattern, "s") && strings.Contains(sedPattern, ":") {
		parts := strings.SplitN(sedPattern, ":", 2)
		if len(parts) == 2 {
			find := parts[0]
			replace := parts[1]
			return strings.ReplaceAll(input, find, replace), nil
		}
	}

	// Support sed with different separators: s/.../ or s|...| or s;...;
	if !strings.HasPrefix(sedPattern, "s") || len(sedPattern) < 4 {
		return "", fmt.Errorf("unsupported sed pattern (must start with s or use find:replace syntax): %s", sedPattern)
	}

	// Detect the separator (character after 's')
	separator := rune(sedPattern[1])

	// Split by separator, accounting for escaped separators
	parts := splitBySeparator(sedPattern[2:], separator)
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid sed pattern format: %s", sedPattern)
	}

	pattern := parts[0]
	replacement := parts[1]

	// Check for 'g' flag (global replacement)
	globalReplace := len(parts) > 2 && strings.Contains(parts[2], "g")

	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex in sed pattern: %w", err)
	}

	if globalReplace {
		return re.ReplaceAllString(input, replacement), nil
	}

	// Replace only first match
	loc := re.FindStringIndex(input)
	if loc == nil {
		return input, nil
	}
	return input[:loc[0]] + re.ReplaceAllString(input[loc[0]:loc[1]], replacement) + input[loc[1]:], nil
}

// splitBySeparator splits a string by a separator character
func splitBySeparator(s string, sep rune) []string {
	var parts []string
	var current strings.Builder
	escaped := false

	for _, ch := range s {
		if escaped {
			// Preserve backslash for regex patterns
			current.WriteRune('\\')
			current.WriteRune(ch)
			escaped = false
			continue
		}

		if ch == '\\' {
			escaped = true
			continue
		}

		if ch == sep {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}

		current.WriteRune(ch)
	}

	// Add the last part
	parts = append(parts, current.String())
	return parts
}

// shouldFilterVersion checks if version should be filtered out
func (vf *VersionFetcher) shouldFilterVersion(version, filterPattern string) bool {
	re, err := regexp.Compile(filterPattern)
	if err != nil {
		return false // Don't filter if regex is invalid
	}

	return re.MatchString(version)
}
