package gateways

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Test packaging a single file
func TestPackager_PackageArtifact_SingleFile(t *testing.T) {
	packager := NewPackager()
	tmpDir := t.TempDir()

	// Create a single binary file
	binaryPath := filepath.Join(tmpDir, "kubectl")
	content := []byte("fake kubectl binary")
	//nolint:gosec // G306: Test executable binary needs 0700 permissions
	if err := os.WriteFile(binaryPath, content, 0700); err != nil {
		t.Fatalf("Failed to create test binary: %v", err)
	}

	recipe := &entities.Recipe{Name: "kubectl"}
	artifact := &entities.Artifact{
		Path: binaryPath,
	}

	// Create dist directory
	distDir := filepath.Join(tmpDir, "dist")
	if err := os.MkdirAll(distDir, 0750); err != nil {
		t.Fatalf("Failed to create dist dir: %v", err)
	}

	// Change to tmpDir so dist/ is created there
	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	//nolint:errcheck // Test cleanup - not critical
	defer os.Chdir(oldWd)

	result, err := packager.PackageArtifact(
		context.Background(),
		recipe,
		artifact,
		"1.28.0",
		"linux-amd64",
		tmpDir,
	)

	if err != nil {
		t.Fatalf("PackageArtifact failed: %v", err)
	}

	if result.Type != "archive" {
		t.Errorf("Expected type=archive, got: %s", result.Type)
	}

	// Verify tarball was created
	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Errorf("Tarball was not created at: %s", result.Path)
	}

	// Verify tarball contains the file
	verifyTarballContents(t, result.Path, "kubectl")
}

// Test packaging a directory
func TestPackager_PackageArtifact_Directory(t *testing.T) {
	packager := NewPackager()
	tmpDir := t.TempDir()

	// Create a directory with multiple files
	extractedDir := filepath.Join(tmpDir, "kubectl-extracted")
	if err := os.MkdirAll(extractedDir, 0750); err != nil {
		t.Fatalf("Failed to create extracted dir: %v", err)
	}

	// Create some files
	files := []string{"kubectl", "README.md", "LICENSE"}
	for _, name := range files {
		path := filepath.Join(extractedDir, name)
		if err := os.WriteFile(path, []byte("content of "+name), 0600); err != nil {
			t.Fatalf("Failed to create file %s: %v", name, err)
		}
	}

	recipe := &entities.Recipe{Name: "kubectl"}
	artifact := &entities.Artifact{
		Path: extractedDir,
	}

	// Create dist directory
	distDir := filepath.Join(tmpDir, "dist")
	if err := os.MkdirAll(distDir, 0750); err != nil {
		t.Fatalf("Failed to create dist dir: %v", err)
	}
	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	//nolint:errcheck // Test cleanup - not critical
	defer os.Chdir(oldWd)

	result, err := packager.PackageArtifact(
		context.Background(),
		recipe,
		artifact,
		"v1.28.0",
		"darwin-arm64",
		tmpDir,
	)

	if err != nil {
		t.Fatalf("PackageArtifact failed: %v", err)
	}

	// Verify version 'v' prefix is removed in filename
	expectedFilename := "kubectl-1.28.0-darwin-arm64.tar.gz"
	if !stringContains(result.Path, expectedFilename) {
		t.Errorf("Expected filename to contain %s, got: %s", expectedFilename, result.Path)
	}

	// Verify tarball exists
	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Errorf("Tarball was not created at: %s", result.Path)
	}
}

// Test packaging with bin directory preference
func TestPackager_PackageArtifact_BinDirectory(t *testing.T) {
	packager := NewPackager()
	tmpDir := t.TempDir()

	// Create extracted directory structure
	extractedDir := filepath.Join(tmpDir, "output", "helm-extracted")
	binDir := filepath.Join(tmpDir, "bin")

	if err := os.MkdirAll(extractedDir, 0750); err != nil {
		t.Fatalf("Failed to create extracted dir: %v", err)
	}

	if err := os.MkdirAll(binDir, 0750); err != nil {
		t.Fatalf("Failed to create bin dir: %v", err)
	}

	// Create files in both directories
	if err := os.WriteFile(filepath.Join(extractedDir, "source.go"), []byte("source"), 0600); err != nil {
		t.Fatal(err)
	}

	//nolint:gosec // G306: Test executable binary needs 0700 permissions
	if err := os.WriteFile(filepath.Join(binDir, "helm"), []byte("binary"), 0700); err != nil {
		t.Fatal(err)
	}

	recipe := &entities.Recipe{Name: "helm"}
	artifact := &entities.Artifact{
		Path: extractedDir,
	}

	distDir := filepath.Join(tmpDir, "dist")
	if err := os.MkdirAll(distDir, 0750); err != nil {
		t.Fatal(err)
	}
	oldWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	//nolint:errcheck // Test cleanup - not critical
	defer os.Chdir(oldWd)

	result, err := packager.PackageArtifact(
		context.Background(),
		recipe,
		artifact,
		"3.12.0",
		"linux-amd64",
		tmpDir,
	)

	if err != nil {
		t.Fatalf("PackageArtifact failed: %v", err)
	}

	// Verify tarball was created
	if _, err := os.Stat(result.Path); os.IsNotExist(err) {
		t.Error("Tarball was not created")
	}
}

// Test error handling for nonexistent artifact
func TestPackager_PackageArtifact_NonexistentArtifact(t *testing.T) {
	packager := NewPackager()

	recipe := &entities.Recipe{Name: "kubectl"}
	artifact := &entities.Artifact{
		Path: "/nonexistent/path",
	}

	_, err := packager.PackageArtifact(
		context.Background(),
		recipe,
		artifact,
		"1.0.0",
		"linux-amd64",
		"/tmp",
	)

	if err == nil {
		t.Error("Expected error for nonexistent artifact, got nil")
	}
}

// Test createTarballFromFile directly
func TestPackager_CreateTarballFromFile(t *testing.T) {
	packager := NewPackager()
	tmpDir := t.TempDir()

	sourceFile := filepath.Join(tmpDir, "binary")
	content := []byte("binary content")
	//nolint:gosec // G306: Test executable binary needs 0700 permissions
	if err := os.WriteFile(sourceFile, content, 0700); err != nil {
		t.Fatal(err)
	}

	tarballPath := filepath.Join(tmpDir, "output.tar.gz")

	err := packager.createTarballFromFile(sourceFile, tarballPath, "myapp")
	if err != nil {
		t.Fatalf("createTarballFromFile failed: %v", err)
	}

	// Verify tarball exists
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		t.Error("Tarball was not created")
	}

	// Verify contents
	verifyTarballContents(t, tarballPath, "myapp")
}

// Test createTarball with symlinks
func TestPackager_CreateTarball_WithSymlinks(t *testing.T) {
	packager := NewPackager()
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(sourceDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create a file and a symlink to it
	realFile := filepath.Join(sourceDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(sourceDir, "link.txt")
	if err := os.Symlink("real.txt", symlinkPath); err != nil {
		t.Skipf("Symlink creation not supported on this system: %v", err)
	}

	tarballPath := filepath.Join(tmpDir, "output.tar.gz")

	err := packager.createTarball(sourceDir, tarballPath)
	if err != nil {
		t.Fatalf("createTarball failed: %v", err)
	}

	// Verify tarball was created
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		t.Error("Tarball was not created")
	}
}

// Test createTarball with nested directories
func TestPackager_CreateTarball_NestedDirectories(t *testing.T) {
	packager := NewPackager()
	tmpDir := t.TempDir()

	sourceDir := filepath.Join(tmpDir, "source")
	nestedDir := filepath.Join(sourceDir, "subdir", "deep")
	if err := os.MkdirAll(nestedDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create files at different levels
	files := map[string]string{
		filepath.Join(sourceDir, "root.txt"):          "root",
		filepath.Join(sourceDir, "subdir", "mid.txt"): "mid",
		filepath.Join(nestedDir, "deep.txt"):          "deep",
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}

	tarballPath := filepath.Join(tmpDir, "nested.tar.gz")

	err := packager.createTarball(sourceDir, tarballPath)
	if err != nil {
		t.Fatalf("createTarball failed: %v", err)
	}

	// Verify tarball exists and contains all files
	if _, err := os.Stat(tarballPath); os.IsNotExist(err) {
		t.Error("Tarball was not created")
	}

	// Verify structure
	entries := extractTarballEntries(t, tarballPath)
	if len(entries) < 3 {
		t.Errorf("Expected at least 3 entries in tarball, got: %d", len(entries))
	}
}

// Helper to verify tarball contains expected file
func verifyTarballContents(t *testing.T, tarballPath, expectedFile string) {
	t.Helper()

	entries := extractTarballEntries(t, tarballPath)

	found := false
	for _, name := range entries {
		if name == expectedFile {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Tarball does not contain %s. Contents: %v", expectedFile, entries)
	}
}

// Helper to extract tarball entry names
func extractTarballEntries(t *testing.T, tarballPath string) []string {
	t.Helper()

	//nolint:gosec // G304: tarballPath is test fixture path
	file, err := os.Open(tarballPath)
	if err != nil {
		t.Fatalf("Failed to open tarball: %v", err)
		return nil
	}
	//nolint:errcheck // Defer close in test
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
		return nil
	}
	//nolint:errcheck // Defer close in test
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	var entries []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Failed to read tar entry: %v", err)
			return nil
		}
		entries = append(entries, header.Name)
	}

	return entries
}

// Helper function for string contains
func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && stringContainsAt(s, substr))
}

func stringContainsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
