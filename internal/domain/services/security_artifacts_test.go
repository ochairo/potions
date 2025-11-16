package services

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ochairo/potions/internal/domain/interfaces"
)

// Test SHA256 generation
func TestSecurityArtifactsService_GenerateSHA256(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")
	testContent := []byte("test content for sha256")

	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Generate SHA256
	checksumPath, err := service.GenerateSHA256(testFile)
	if err != nil {
		t.Fatalf("GenerateSHA256 failed: %v", err)
	}

	// Verify checksum file exists
	if _, err := os.Stat(checksumPath); os.IsNotExist(err) {
		t.Error("SHA256 file was not created")
	}

	// Verify checksum file content
	//nolint:gosec // G304: checksumPath is test output file
	content, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatalf("Failed to read checksum file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "test.bin") {
		t.Errorf("Checksum file should contain filename, got: %s", contentStr)
	}

	// SHA256 should be 64 hex characters
	parts := strings.Fields(contentStr)
	if len(parts) < 1 || len(parts[0]) != 64 {
		t.Errorf("Invalid SHA256 hash format: %s", contentStr)
	}
}

// Test SHA512 generation
func TestSecurityArtifactsService_GenerateSHA512(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")
	testContent := []byte("test content for sha512")

	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	checksumPath, err := service.GenerateSHA512(testFile)
	if err != nil {
		t.Fatalf("GenerateSHA512 failed: %v", err)
	}

	if _, err := os.Stat(checksumPath); os.IsNotExist(err) {
		t.Error("SHA512 file was not created")
	}

	//nolint:gosec // G304: checksumPath is test output file
	content, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatalf("Failed to read checksum file: %v", err)
	}

	// SHA512 should be 128 hex characters
	parts := strings.Fields(string(content))
	if len(parts) < 1 || len(parts[0]) != 128 {
		t.Errorf("Invalid SHA512 hash format: %s", string(content))
	}
}

// Test SBOM generation
func TestSecurityArtifactsService_GenerateSBOM(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "kubectl-1.28.0.tar.gz")
	testContent := []byte("fake tarball content")

	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	sbomPath, err := service.GenerateSBOM(context.Background(), testFile)
	if err != nil {
		t.Fatalf("GenerateSBOM failed: %v", err)
	}

	// Verify SBOM file exists
	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		t.Error("SBOM file was not created")
	}

	// Verify SBOM content is valid JSON
	//nolint:gosec // G304: sbomPath is test output file
	content, err := os.ReadFile(sbomPath)
	if err != nil {
		t.Fatalf("Failed to read SBOM file: %v", err)
	}

	var sbom map[string]interface{}
	if err := json.Unmarshal(content, &sbom); err != nil {
		t.Fatalf("SBOM is not valid JSON: %v", err)
	}

	// Verify SBOM structure
	if sbom["bomFormat"] != "CycloneDX" {
		t.Errorf("Expected bomFormat=CycloneDX, got: %v", sbom["bomFormat"])
	}

	if sbom["specVersion"] != "1.5" {
		t.Errorf("Expected specVersion=1.5, got: %v", sbom["specVersion"])
	}

	// Verify components exist
	components, ok := sbom["components"].([]interface{})
	if !ok || len(components) == 0 {
		t.Error("SBOM should contain components")
	}
}

// Test provenance generation
func TestSecurityArtifactsService_GenerateProvenance(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "kubectl-1.28.0.tar.gz")
	testContent := []byte("fake tarball for provenance")

	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	provenancePath, err := service.GenerateProvenance(context.Background(), testFile)
	if err != nil {
		t.Fatalf("GenerateProvenance failed: %v", err)
	}

	// Verify provenance file exists
	if _, err := os.Stat(provenancePath); os.IsNotExist(err) {
		t.Error("Provenance file was not created")
	}

	// Verify provenance is valid JSON
	//nolint:gosec // G304: provenancePath is test output file
	content, err := os.ReadFile(provenancePath)
	if err != nil {
		t.Fatalf("Failed to read provenance file: %v", err)
	}

	var provenance map[string]interface{}
	if err := json.Unmarshal(content, &provenance); err != nil {
		t.Fatalf("Provenance is not valid JSON: %v", err)
	}

	// Verify SLSA provenance structure
	if provenance["_type"] != "https://in-toto.io/Statement/v0.1" {
		t.Errorf("Invalid provenance _type: %v", provenance["_type"])
	}

	if provenance["predicateType"] != "https://slsa.dev/provenance/v0.2" {
		t.Errorf("Invalid predicateType: %v", provenance["predicateType"])
	}

	// Verify subject exists with digest
	subject, ok := provenance["subject"].([]interface{})
	if !ok || len(subject) == 0 {
		t.Fatal("Provenance should contain subject")
	}

	firstSubject, ok := subject[0].(map[string]interface{})
	if !ok {
		t.Fatal("Subject should be a map")
	}

	digest, ok := firstSubject["digest"].(map[string]interface{})
	if !ok {
		t.Fatal("Subject should contain digest")
	}

	if _, hasSHA256 := digest["sha256"]; !hasSHA256 {
		t.Error("Digest should contain sha256")
	}

	if _, hasSHA512 := digest["sha512"]; !hasSHA512 {
		t.Error("Digest should contain sha512")
	}
}

// Test GenerateAllArtifacts
func TestSecurityArtifactsService_GenerateAllArtifacts(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "kubectl-1.28.0.tar.gz")
	testContent := []byte("comprehensive test content")

	if err := os.WriteFile(testFile, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	artifacts, err := service.GenerateAllArtifacts(context.Background(), testFile)
	if err != nil {
		t.Fatalf("GenerateAllArtifacts failed: %v", err)
	}

	// Verify all artifacts were generated
	if artifacts.SHA256Path == "" {
		t.Error("SHA256Path should be set")
	}

	if artifacts.SHA512Path == "" {
		t.Error("SHA512Path should be set")
	}

	if artifacts.SBOMPath == "" {
		t.Error("SBOMPath should be set")
	}

	if artifacts.ProvenancePath == "" {
		t.Error("ProvenancePath should be set")
	}

	// Verify all files exist
	for _, path := range []string{
		artifacts.SHA256Path,
		artifacts.SHA512Path,
		artifacts.SBOMPath,
		artifacts.ProvenancePath,
	} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("Artifact file does not exist: %s", path)
		}
	}
}

// Test error handling for nonexistent file
func TestSecurityArtifactsService_NonexistentFile(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	_, err := service.GenerateSHA256("/nonexistent/file.tar.gz")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

// Test hash computation error handling
func TestSecurityArtifactsService_ComputeSHA256Error(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	hash, err := service.computeSHA256("/nonexistent/path/file.bin")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}

	if hash != "" {
		t.Errorf("Hash should be empty on error, got: %s", hash)
	}
}

// Test hash determinism (same content should produce same hash)
func TestSecurityArtifactsService_HashDeterminism(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	tmpDir := t.TempDir()
	testFile1 := filepath.Join(tmpDir, "file1.bin")
	testFile2 := filepath.Join(tmpDir, "file2.bin")
	testContent := []byte("identical content for both files")

	if err := os.WriteFile(testFile1, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test file 1: %v", err)
	}

	if err := os.WriteFile(testFile2, testContent, 0600); err != nil {
		t.Fatalf("Failed to create test file 2: %v", err)
	}

	hash1, err := service.computeSHA256(testFile1)
	if err != nil {
		t.Fatalf("Failed to compute hash 1: %v", err)
	}

	hash2, err := service.computeSHA256(testFile2)
	if err != nil {
		t.Fatalf("Failed to compute hash 2: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Identical files should produce identical hashes: %s != %s", hash1, hash2)
	}
}

// Test mustCompute helpers return empty string on error
func TestSecurityArtifactsService_MustComputeHelpers(t *testing.T) {
	service := NewSecurityArtifactsService(&interfaces.NoOpLogger{})

	hash256 := service.mustComputeSHA256("/nonexistent/file")
	if hash256 != "" {
		t.Errorf("mustComputeSHA256 should return empty string on error, got: %s", hash256)
	}

	hash512 := service.mustComputeSHA512("/nonexistent/file")
	if hash512 != "" {
		t.Errorf("mustComputeSHA512 should return empty string on error, got: %s", hash512)
	}
}
