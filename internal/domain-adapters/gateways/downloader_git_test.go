package gateways

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

func TestDownloader_GitClone(t *testing.T) {
	downloader := NewDownloader()

	// Create temp directory
	tmpDir := t.TempDir()

	// Define a minimal recipe using git method
	recipe := &entities.Recipe{
		Name: "age",
		Download: entities.RecipeDownload{
			Method:       "git",
			GitURL:       "https://github.com/FiloSottile/age.git",
			GitTagPrefix: "v",
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {},
			},
		},
	}

	version := "1.2.1" // This tag exists
	platform := "linux-amd64"

	// Call DownloadArtifact which should use git clone
	artifact, err := downloader.DownloadArtifact(recipe, version, platform, tmpDir)
	if err != nil {
		t.Fatalf("DownloadArtifact with git method failed: %v", err)
	}

	// Verify the clone directory exists
	if _, err := os.Stat(artifact.Path); os.IsNotExist(err) {
		t.Errorf("Cloned directory does not exist: %s", artifact.Path)
	}

	// Verify it contains expected files (age should have a go.mod file)
	goModPath := filepath.Join(artifact.Path, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		t.Errorf("Expected go.mod file not found in cloned repo: %s", goModPath)
	}

	t.Logf("✅ Successfully cloned age via git to: %s", artifact.Path)
}

func TestDownloader_GitClone_InvalidTag(t *testing.T) {
	downloader := NewDownloader()
	tmpDir := t.TempDir()

	recipe := &entities.Recipe{
		Name: "age",
		Download: entities.RecipeDownload{
			Method:       "git",
			GitURL:       "https://github.com/FiloSottile/age.git",
			GitTagPrefix: "v",
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {},
			},
		},
	}

	version := "999.999.999" // This tag does not exist
	platform := "linux-amd64"

	// Should fail with invalid tag
	_, err := downloader.DownloadArtifact(recipe, version, platform, tmpDir)
	if err == nil {
		t.Fatal("Expected error for invalid git tag, got nil")
	}

	if err.Error() == "" {
		t.Error("Error message is empty")
	}

	t.Logf("✅ Correctly failed with error: %v", err)
}
