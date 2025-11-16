package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ochairo/potions/internal/domain/interfaces/gateways"
)

// Test creating a new GitHub gateway
func TestNewHTTPGitHubGateway(t *testing.T) {
	gateway := NewHTTPGitHubGateway("test-token")

	if gateway == nil {
		t.Fatal("NewHTTPGitHubGateway returned nil")
	}

	if gateway.token != "test-token" {
		t.Errorf("Token = %s, want test-token", gateway.token)
	}
}

// Test create release with API error
func TestGitHubGateway_CreateRelease_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"id": 123, "tag_name": "v1.0.0", "name": "Release 1.0.0"}`))
	}))
	defer server.Close()

	gateway := NewHTTPGitHubGateway("test-token")

	release := &gateways.GitHubRelease{
		TagName: "v1.0.0",
		Name:    "Release v1.0.0",
	}

	_, err := gateway.CreateRelease(context.Background(), "test", "repo", release)

	if err == nil {
		t.Fatal("Expected error for API failure, got nil")
	}
}

// Test get release not found
func TestGitHubGateway_GetRelease_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer server.Close()

	gateway := NewHTTPGitHubGateway("test-token")

	_, err := gateway.GetRelease(context.Background(), "test", "repo", "nonexistent")

	if err == nil {
		t.Fatal("Expected error for 404, got nil")
	}
}

// Test upload asset
func TestGitHubGateway_UploadAsset_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		w.WriteHeader(http.StatusCreated)
		response := githubAsset{
			ID:                 456,
			Name:               "test.tar.gz",
			State:              "uploaded",
			Size:               1024,
			BrowserDownloadURL: "https://github.com/test/repo/releases/download/v1.0.0/test.tar.gz",
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	gateway := NewHTTPGitHubGateway("test-token")

	// Don't include template suffix - just the base URL
	uploadURL := server.URL
	content := bytes.NewReader([]byte("test content"))

	result, err := gateway.UploadAsset(context.Background(), uploadURL, "test.tar.gz", content)

	if err != nil {
		t.Fatalf("UploadAsset failed: %v", err)
	}

	if result.Name != "test.tar.gz" {
		t.Errorf("Asset name = %s, want test.tar.gz", result.Name)
	}

	if result.State != "uploaded" {
		t.Errorf("Asset state = %s, want uploaded", result.State)
	}
} // Test upload asset with invalid URL
func TestGitHubGateway_UploadAsset_InvalidURL(t *testing.T) {
	gateway := NewHTTPGitHubGateway("test-token")

	_, err := gateway.UploadAsset(context.Background(), "://invalid-url", "test.tar.gz", strings.NewReader("test"))

	if err == nil {
		t.Fatal("Expected error for invalid URL, got nil")
	}

	if !strings.Contains(err.Error(), "invalid upload URL") {
		t.Errorf("Expected 'invalid upload URL' error, got: %v", err)
	}
}

// Test upload asset with API error
func TestGitHubGateway_UploadAsset_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message": "Invalid asset"}`))
	}))
	defer server.Close()

	gateway := NewHTTPGitHubGateway("test-token")

	uploadURL := server.URL

	_, err := gateway.UploadAsset(context.Background(), uploadURL, "test.tar.gz", strings.NewReader("test"))

	if err == nil {
		t.Fatal("Expected error for API failure, got nil")
	}

	if !strings.Contains(err.Error(), "failed to upload asset") {
		t.Errorf("Expected upload error, got: %v", err)
	}
} // Test list assets with API error
func TestGitHubGateway_ListReleaseAssets_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message": "Server Error"}`))
	}))
	defer server.Close()

	gateway := NewHTTPGitHubGateway("test-token")

	_, err := gateway.ListReleaseAssets(context.Background(), "test", "repo", 123)

	if err == nil {
		t.Fatal("Expected error for API failure, got nil")
	}
}

// Test context cancellation
func TestGitHubGateway_ContextCancellation(t *testing.T) {
	gateway := NewHTTPGitHubGateway("test-token")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	release := &gateways.GitHubRelease{
		TagName: "v1.0.0",
		Name:    "Test",
	}

	_, err := gateway.CreateRelease(ctx, "test", "repo", release)

	if err == nil {
		t.Fatal("Expected error for canceled context, got nil")
	}
}

// Test upload asset with empty content
func TestGitHubGateway_UploadAsset_EmptyContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read body
		body, _ := io.ReadAll(r.Body)

		if len(body) != 0 {
			t.Errorf("Expected empty body, got %d bytes", len(body))
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(githubAsset{ID: 1, Name: "empty.tar.gz"})
	}))
	defer server.Close()

	gateway := NewHTTPGitHubGateway("test-token")

	uploadURL := server.URL

	result, err := gateway.UploadAsset(context.Background(), uploadURL, "empty.tar.gz", bytes.NewReader([]byte{}))

	if err != nil {
		t.Fatalf("UploadAsset failed: %v", err)
	}

	if result.Name != "empty.tar.gz" {
		t.Errorf("Asset name = %s, want empty.tar.gz", result.Name)
	}
}
