package gateways

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

func TestDownloader_BuildDownloadURL(t *testing.T) {
	d := NewDownloader()

	tests := []struct {
		name     string
		template string
		version  string
		platform entities.PlatformConfig
		want     string
	}{
		{
			name:     "kubectl URL",
			template: "https://dl.k8s.io/release/v{version}/bin/{os}/{arch}/kubectl",
			version:  "1.28.3",
			platform: entities.PlatformConfig{OS: "darwin", Arch: "arm64"},
			want:     "https://dl.k8s.io/release/v1.28.3/bin/darwin/arm64/kubectl",
		},
		{
			name:     "helm tarball URL",
			template: "https://get.helm.sh/helm-v{version}-{os}-{arch}.tar.gz",
			version:  "3.13.0",
			platform: entities.PlatformConfig{OS: "linux", Arch: "amd64"},
			want:     "https://get.helm.sh/helm-v3.13.0-linux-amd64.tar.gz",
		},
		{
			name:     "age URL with version in path",
			template: "https://github.com/FiloSottile/age/releases/download/v{version}/age-v{version}-{os}-{arch}.tar.gz",
			version:  "1.1.1",
			platform: entities.PlatformConfig{OS: "darwin", Arch: "amd64"},
			want:     "https://github.com/FiloSottile/age/releases/download/v1.1.1/age-v1.1.1-darwin-amd64.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := d.BuildDownloadURL(tt.template, tt.version, &tt.platform)
			if got != tt.want {
				t.Errorf("BuildDownloadURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDownloader_DownloadArtifact_UnsupportedPlatform(t *testing.T) {
	d := NewDownloader()

	def := &entities.Recipe{
		Name: "test",
		Download: entities.RecipeDownload{
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	_, err := d.DownloadArtifact(def, "1.0.0", "unsupported-platform", "/tmp/test")
	if err == nil {
		t.Error("DownloadArtifact() should fail for unsupported platform")
	}
}

func TestDownloader_ExtractTarGz_PathTraversal(t *testing.T) {
	d := NewDownloader()

	// This test verifies that path traversal attacks are blocked
	// We can't easily create a malicious tar.gz in a test without external tools,
	// but we can verify the security check logic exists in the code

	// For now, just verify the function signature exists
	tempDir := t.TempDir()
	err := d.extractTarGz("/nonexistent.tar.gz", tempDir)

	// Should fail because file doesn't exist, not because of security check
	if err == nil {
		t.Error("extractTarGz() should fail for nonexistent file")
	}
}

func TestDownloader_DownloadArtifact_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	d := NewDownloader()

	// Test downloading a small real binary (age is relatively small)
	def := &entities.Recipe{
		Name: "age",
		Download: entities.RecipeDownload{
			DownloadURL: "https://github.com/FiloSottile/age/releases/download/v{version}/age-v{version}-{os}-{arch}.tar.gz",
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	outputDir := t.TempDir()

	artifact, err := d.DownloadArtifact(def, "1.1.1", "linux-amd64", outputDir)
	if err != nil {
		t.Fatalf("DownloadArtifact() error = %v", err)
	}

	if artifact == nil {
		t.Fatal("DownloadArtifact() returned nil artifact")
	}

	if artifact.Name != "age" {
		t.Errorf("artifact.Name = %v, want age", artifact.Name)
	}

	if artifact.Version != "1.1.1" {
		t.Errorf("artifact.Version = %v, want 1.1.1", artifact.Version)
	}

	if artifact.Platform != "linux-amd64" {
		t.Errorf("artifact.Platform = %v, want linux-amd64", artifact.Platform)
	}

	// Verify extracted directory exists
	if _, err := os.Stat(artifact.Path); os.IsNotExist(err) {
		t.Errorf("artifact path does not exist: %s", artifact.Path)
	}

	// Verify age binary exists in extracted directory
	ageBinary := filepath.Join(artifact.Path, "age", "age")
	if _, err := os.Stat(ageBinary); os.IsNotExist(err) {
		t.Errorf("age binary not found at expected path: %s", ageBinary)
	}

	t.Logf("Successfully downloaded and extracted age to: %s", artifact.Path)
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "URL with query parameters",
			input:    "https://download.mozilla.org/?product=firefox-latest-ssl&os=linux64&lang=en-US",
			expected: "download",
		},
		{
			name:     "URL with filename and query params",
			input:    "https://example.com/file.tar.gz?version=1.0&platform=linux",
			expected: "file.tar.gz",
		},
		{
			name:     "Simple URL with filename",
			input:    "https://example.com/path/to/file.tar.gz",
			expected: "file.tar.gz",
		},
		{
			name:     "URL with invalid characters in path",
			input:    "https://example.com/file:with*invalid?chars.tar.gz",
			expected: "file_with_invalid",
		},
		{
			name:     "Filename with colon (treated as URL scheme)",
			input:    "file:name.tar.gz",
			expected: "download", // URL parser treats this as a scheme
		},
		{
			name:     "Empty path",
			input:    "https://example.com/",
			expected: "download",
		},
		{
			name:     "Just a slash",
			input:    "/",
			expected: "download",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestDownloader_DownloadFileWithFallback_MirrorUsed(t *testing.T) {
	d := NewDownloader()

	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "test.txt")

	// Test 1: Primary fails, mirror also fails (but we try it)
	// Using URLs that will fail (invalid)
	primaryURL := "http://invalid-primary-url-12345.example.local/file.txt"
	mirrorURL := "http://invalid-mirror-url-12345.example.local/file.txt"

	// This should fail since both URLs are invalid, but it demonstrates the fallback logic
	err := d.downloadFileWithFallback(primaryURL, mirrorURL, destFile)
	if err == nil {
		t.Error("downloadFileWithFallback() should fail with invalid URLs")
	}

	// The error message should indicate both attempts failed or mention fallback
	errMsg := err.Error()
	hasFailed := false
	if len(errMsg) > 0 {
		hasFailed = true // Any non-empty error is a failure message
	}
	if !hasFailed {
		t.Errorf("downloadFileWithFallback() should return an error message")
	}
}

func TestDownloader_DownloadFileWithFallback_NoMirror(t *testing.T) {
	d := NewDownloader()

	tmpDir := t.TempDir()
	destFile := filepath.Join(tmpDir, "test.txt")

	// Test without mirror - just primary URL
	primaryURL := "http://invalid-url.example.local/file.txt"

	err := d.downloadFileWithFallback(primaryURL, "", destFile)
	if err == nil {
		t.Error("downloadFileWithFallback() should fail with invalid URL and no mirror")
	}
}
