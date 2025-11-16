package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/interfaces/gateways"
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
			Timeout: 30 * time.Second,
		},
		token:     token,
		userAgent: "potions/1.0",
	}
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

	resp, err := g.client.Do(req)
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

	resp, err := g.client.Do(req)
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
	// Parse upload URL and replace template
	u, err := url.Parse(uploadURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upload URL: %w", err)
	}

	// Remove template suffix (e.g., {?name,label})
	baseURL := strings.Split(u.String(), "{")[0]

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

	resp, err := g.client.Do(req)
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
		return nil, fmt.Errorf("failed to upload asset: status %d: %s", resp.StatusCode, string(bodyBytes))
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

	resp, err := g.client.Do(req)
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
