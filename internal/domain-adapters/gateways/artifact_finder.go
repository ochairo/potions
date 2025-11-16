package gateways

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ArtifactFinder provides utilities for locating build artifacts
type ArtifactFinder struct{}

// NewArtifactFinder creates a new artifact finder
func NewArtifactFinder() *ArtifactFinder {
	return &ArtifactFinder{}
}

// FindRecursive searches recursively for package artifacts
// Finds: .tar.gz, .sha256, .sha512, .sbom.json, .provenance.json
func (f *ArtifactFinder) FindRecursive(artifactsDir, packageName, version string) ([]string, error) {
	// Check if directory exists
	if _, err := os.Stat(artifactsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("artifacts directory does not exist: %s", artifactsDir)
	}

	versionClean := strings.TrimPrefix(version, "v")
	var artifacts []string

	err := filepath.Walk(artifactsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		basename := filepath.Base(path)
		expectedPrefix := fmt.Sprintf("%s-%s-", packageName, versionClean)

		// Check if file matches package-version pattern
		if strings.HasPrefix(basename, expectedPrefix) {
			// Accept artifact files
			if strings.HasSuffix(basename, ".tar.gz") ||
				strings.HasSuffix(basename, ".sha256") ||
				strings.HasSuffix(basename, ".sha512") ||
				strings.HasSuffix(basename, ".sbom.json") ||
				strings.HasSuffix(basename, ".provenance.json") {
				artifacts = append(artifacts, path)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return artifacts, nil
}

// FindByGlob searches using glob patterns for package artifacts
func (f *ArtifactFinder) FindByGlob(binariesDir, packageName, version string) ([]string, error) {
	var artifacts []string

	// Remove 'v' prefix from version for file matching
	versionClean := strings.TrimPrefix(version, "v")

	// Pattern: packageName-version-platform.tar.gz{,.sha256,.sha512,.sbom.json,.provenance.json}
	patterns := []string{
		fmt.Sprintf("%s-%s-*.tar.gz", packageName, versionClean),
		fmt.Sprintf("%s-%s-*.tar.gz.sha256", packageName, versionClean),
		fmt.Sprintf("%s-%s-*.tar.gz.sha512", packageName, versionClean),
		fmt.Sprintf("%s-%s-*.tar.gz.sbom.json", packageName, versionClean),
		fmt.Sprintf("%s-%s-*.tar.gz.provenance.json", packageName, versionClean),
	}

	for _, pattern := range patterns {
		fullPattern := filepath.Join(binariesDir, pattern)
		matches, err := filepath.Glob(fullPattern)
		if err != nil {
			return nil, fmt.Errorf("failed to glob pattern %s: %w", pattern, err)
		}
		artifacts = append(artifacts, matches...)
	}

	return artifacts, nil
}
