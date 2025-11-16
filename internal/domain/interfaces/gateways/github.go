// Package gateways defines interfaces for external service adapters.
package gateways

import (
	"context"
	"io"
)

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	ID          int64
	TagName     string
	Name        string
	Body        string
	Draft       bool
	Prerelease  bool
	CreatedAt   string
	PublishedAt string
	HTMLURL     string
	UploadURL   string
}

// GitHubAsset represents an uploaded release asset
type GitHubAsset struct {
	ID                 int64
	Name               string
	Label              string
	State              string
	Size               int64
	DownloadCount      int
	BrowserDownloadURL string
}

// GitHubGateway defines operations for GitHub API interactions
type GitHubGateway interface {
	// CreateRelease creates a new GitHub release
	CreateRelease(ctx context.Context, owner, repo string, release *GitHubRelease) (*GitHubRelease, error)

	// GetRelease retrieves a release by tag name
	GetRelease(ctx context.Context, owner, repo, tag string) (*GitHubRelease, error)

	// UploadAsset uploads a file to a release
	UploadAsset(ctx context.Context, uploadURL, filename string, content io.Reader) (*GitHubAsset, error)

	// ListReleaseAssets lists all assets for a release
	ListReleaseAssets(ctx context.Context, owner, repo string, releaseID int64) ([]*GitHubAsset, error)
}
