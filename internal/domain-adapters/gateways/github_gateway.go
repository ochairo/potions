package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/interfaces/gateways"
)

const (
	// Max retries for transient errors
	maxRetries = 3
	// Initial backoff duration
	initialBackoff = 1 * time.Second
	// Max backoff duration
	maxBackoff = 32 * time.Second
)

// HTTPGitHubGateway implements GitHubGateway using standard HTTP client
type HTTPGitHubGateway struct {
	client    *http.Client
	token     string
	userAgent string
}

// NewHTTPGitHubGateway creates a new GitHub gateway with HTTP client
func NewHTTPGitHubGateway(token string) *HTTPGitHubGateway {
	return &HTTPGitHubGateway{
		client: &http.Client{
			Timeout: 5 * time.Minute, // Increased for large artifact uploads
		},
		token:     token,
		userAgent: "potions/1.0",
	}
}

// checkRateLimit checks GitHub API rate limit headers and returns error if exhausted
func checkRateLimit(resp *http.Response) error {
	remaining := resp.Header.Get("X-RateLimit-Remaining")
	if remaining == "" {
		return nil // No rate limit header, continue
	}

	remainingInt, err := strconv.Atoi(remaining)
	if err != nil {
		return nil // Invalid header, ignore
	}

	// If exhausted, return error immediately (don't wait in tests/CI)
	if remainingInt == 0 {
		resetTime := resp.Header.Get("X-RateLimit-Reset")
		if resetTime != "" {
			if resetUnix, err := strconv.ParseInt(resetTime, 10, 64); err == nil {
				resetAt := time.Unix(resetUnix, 0)
				return fmt.Errorf("GitHub API rate limit exceeded (0 remaining), resets at %s", resetAt.Format(time.RFC3339))
			}
		}
		return fmt.Errorf("GitHub API rate limit exceeded (0 remaining)")
	}

	// Warn if getting low
	if remainingInt <= 10 {
		// Note: This is adapter layer, direct logging is acceptable here
		// In production, consider injecting logger interface
		fmt.Fprintf(os.Stderr, "⚠️  GitHub API rate limit low: %d remaining\n", remainingInt)
	}

	return nil
}

// isRetryableError checks if an HTTP status code is retryable
func isRetryableError(statusCode int) bool {
	switch statusCode {
	case http.StatusForbidden, // 403 - rate limit
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// calculateBackoff returns the backoff duration for a retry attempt
func calculateBackoff(attempt int) time.Duration {
	backoff := float64(initialBackoff) * math.Pow(2, float64(attempt))
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}
	return time.Duration(backoff)
}

// doWithRetry executes an HTTP request with exponential backoff retry
func (g *HTTPGitHubGateway) doWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := calculateBackoff(attempt - 1)
			time.Sleep(backoff)
		}

		resp, err = g.client.Do(req)
		if err != nil {
			// Network errors are retryable
			if attempt < maxRetries {
				continue
			}
			return nil, err
		}

		// Check rate limit
		if rateLimitErr := checkRateLimit(resp); rateLimitErr != nil {
			//nolint:errcheck,gosec // G104: Best effort close on rate limit error
			resp.Body.Close()
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

// githubRelease represents the GitHub API release format
type githubRelease struct {
	ID          int64  `json:"id,omitempty"`
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	Body        string `json:"body"`
	Draft       bool   `json:"draft"`
	Prerelease  bool   `json:"prerelease"`
	CreatedAt   string `json:"created_at,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	HTMLURL     string `json:"html_url,omitempty"`
	UploadURL   string `json:"upload_url,omitempty"`
}

// githubAsset represents a GitHub release asset
type githubAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	Label              string `json:"label"`
	State              string `json:"state"`
	Size               int64  `json:"size"`
	DownloadCount      int    `json:"download_count"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CreateRelease creates a new GitHub release
func (g *HTTPGitHubGateway) CreateRelease(ctx context.Context, owner, repo string, release *gateways.GitHubRelease) (*gateways.GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", owner, repo)

	apiRelease := githubRelease{
		TagName:    release.TagName,
		Name:       release.Name,
		Body:       release.Body,
		Draft:      release.Draft,
		Prerelease: release.Prerelease,
	}

	body, err := json.Marshal(apiRelease)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal release: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", g.userAgent)
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.doWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create release: %w", err)
	}
	//nolint:errcheck // Defer close on HTTP response body
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create release: status %d (failed to read response)", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to create release: status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &gateways.GitHubRelease{
		ID:          result.ID,
		TagName:     result.TagName,
		Name:        result.Name,
		Body:        result.Body,
		Draft:       result.Draft,
		Prerelease:  result.Prerelease,
		CreatedAt:   result.CreatedAt,
		PublishedAt: result.PublishedAt,
		HTMLURL:     result.HTMLURL,
		UploadURL:   result.UploadURL,
	}, nil
}

// GetRelease retrieves a release by tag name
func (g *HTTPGitHubGateway) GetRelease(ctx context.Context, owner, repo, tag string) (*gateways.GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/tags/%s", owner, repo, tag)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", g.userAgent)

	resp, err := g.doWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get release: %w", err)
	}
	//nolint:errcheck // Defer close on HTTP response body
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("release not found: %s", tag)
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("HTTP %d: failed to read error response", resp.StatusCode)
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &gateways.GitHubRelease{
		ID:          result.ID,
		TagName:     result.TagName,
		Name:        result.Name,
		Body:        result.Body,
		Draft:       result.Draft,
		Prerelease:  result.Prerelease,
		CreatedAt:   result.CreatedAt,
		PublishedAt: result.PublishedAt,
		HTMLURL:     result.HTMLURL,
		UploadURL:   result.UploadURL,
	}, nil
}

// UploadAsset uploads a file to a release
func (g *HTTPGitHubGateway) UploadAsset(ctx context.Context, uploadURL, filename string, content io.Reader) (*gateways.GitHubAsset, error) {
	// Remove template suffix BEFORE any processing (e.g., {?name,label})
	// GitHub returns URLs like: https://uploads.github.com/.../assets{?name,label}
	baseURL := strings.Split(uploadURL, "{")[0]

	// Validate the URL is well-formed
	if _, err := url.Parse(baseURL); err != nil {
		return nil, fmt.Errorf("invalid upload URL: %w", err)
	}

	// GitHub's upload URLs should use uploads.github.com, not api.github.com
	// Verify we have the correct domain
	if !strings.Contains(baseURL, "uploads.github.com") && strings.Contains(baseURL, "api.github.com") {
		// Fix incorrect URL domain
		baseURL = strings.Replace(baseURL, "api.github.com", "uploads.github.com", 1)
	}

	// Add filename as query parameter
	uploadURLWithName := fmt.Sprintf("%s?name=%s", baseURL, url.QueryEscape(filename))

	// Read content into buffer to get size
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, content); err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURLWithName, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", g.userAgent)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(buf.Len())

	resp, err := g.doWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("failed to upload asset: %w", err)
	}
	//nolint:errcheck // Defer close on HTTP response body
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to upload asset: status %d (failed to read response)", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to upload asset: status %d: %s (URL: %s)", resp.StatusCode, string(bodyBytes), uploadURLWithName)
	}

	var result githubAsset
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &gateways.GitHubAsset{
		ID:                 result.ID,
		Name:               result.Name,
		Label:              result.Label,
		State:              result.State,
		Size:               result.Size,
		DownloadCount:      result.DownloadCount,
		BrowserDownloadURL: result.BrowserDownloadURL,
	}, nil
}

// ListReleaseAssets lists all assets for a release
func (g *HTTPGitHubGateway) ListReleaseAssets(ctx context.Context, owner, repo string, releaseID int64) ([]*gateways.GitHubAsset, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/%d/assets", owner, repo, releaseID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", g.userAgent)

	resp, err := g.doWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list assets: %w", err)
	}
	//nolint:errcheck // Defer close on HTTP response body
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to list assets: status %d (failed to read response)", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to list assets: status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var results []githubAsset
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	assets := make([]*gateways.GitHubAsset, len(results))
	for i, a := range results {
		assets[i] = &gateways.GitHubAsset{
			ID:                 a.ID,
			Name:               a.Name,
			Label:              a.Label,
			State:              a.State,
			Size:               a.Size,
			DownloadCount:      a.DownloadCount,
			BrowserDownloadURL: a.BrowserDownloadURL,
		}
	}

	return assets, nil
}

// ListReleases lists all releases in a repository
func (g *HTTPGitHubGateway) ListReleases(ctx context.Context, owner, repo string) ([]*gateways.GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", owner, repo)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "token "+g.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", g.userAgent)

	resp, err := g.doWithRetry(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}
	//nolint:errcheck // Defer close on HTTP response body
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to list releases: status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiReleases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&apiReleases); err != nil {
		return nil, fmt.Errorf("failed to decode releases: %w", err)
	}

	releases := make([]*gateways.GitHubRelease, len(apiReleases))
	for i, r := range apiReleases {
		releases[i] = &gateways.GitHubRelease{
			ID:          r.ID,
			TagName:     r.TagName,
			Name:        r.Name,
			Body:        r.Body,
			Draft:       r.Draft,
			Prerelease:  r.Prerelease,
			CreatedAt:   r.CreatedAt,
			PublishedAt: r.PublishedAt,
			HTMLURL:     r.HTMLURL,
			UploadURL:   r.UploadURL,
		}
	}

	return releases, nil
}
