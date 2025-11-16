package services

import (
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// SecurityArtifactsService handles generation of security artifacts
type SecurityArtifactsService struct{}

// NewSecurityArtifactsService creates a new security artifacts service
func NewSecurityArtifactsService() *SecurityArtifactsService {
	return &SecurityArtifactsService{}
}

// SecurityArtifacts represents all security artifacts for a binary
type SecurityArtifacts struct {
	SHA256Path     string
	SHA512Path     string
	SBOMPath       string
	ProvenancePath string
}

// GenerateAllArtifacts generates all security artifacts for a tarball
func (s *SecurityArtifactsService) GenerateAllArtifacts(ctx context.Context, tarballPath string) (*SecurityArtifacts, error) {
	artifacts := &SecurityArtifacts{}

	// Generate checksums
	fmt.Printf("  🔐 Generating checksums...\n")
	sha256Path, err := s.GenerateSHA256(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SHA256: %w", err)
	}
	artifacts.SHA256Path = sha256Path

	sha512Path, err := s.GenerateSHA512(tarballPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate SHA512: %w", err)
	}
	artifacts.SHA512Path = sha512Path

	// Generate SBOM (simple implementation)
	fmt.Printf("  📋 Generating SBOM...\n")
	sbomPath, err := s.GenerateSBOM(ctx, tarballPath)
	if err != nil {
		fmt.Printf("  ⚠️  SBOM generation failed: %v (continuing...)\n", err)
	} else {
		artifacts.SBOMPath = sbomPath
	}

	// Generate provenance
	fmt.Printf("  📝 Generating provenance...\n")
	provenancePath, err := s.GenerateProvenance(ctx, tarballPath)
	if err != nil {
		fmt.Printf("  ⚠️  Provenance generation failed: %v (continuing...)\n", err)
	} else {
		artifacts.ProvenancePath = provenancePath
	}

	return artifacts, nil
}

// GenerateSHA256 generates SHA256 checksum file
func (s *SecurityArtifactsService) GenerateSHA256(filePath string) (string, error) {
	hash, err := s.computeSHA256(filePath)
	if err != nil {
		return "", err
	}

	checksumPath := filePath + ".sha256"
	content := fmt.Sprintf("%s  %s\n", hash, filepath.Base(filePath))

	if err := os.WriteFile(checksumPath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("failed to write SHA256 file: %w", err)
	}

	return checksumPath, nil
}

// GenerateSHA512 generates SHA512 checksum file
func (s *SecurityArtifactsService) GenerateSHA512(filePath string) (string, error) {
	hash, err := s.computeSHA512(filePath)
	if err != nil {
		return "", err
	}

	checksumPath := filePath + ".sha512"
	content := fmt.Sprintf("%s  %s\n", hash, filepath.Base(filePath))

	if err := os.WriteFile(checksumPath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("failed to write SHA512 file: %w", err)
	}

	return checksumPath, nil
}

// GenerateSBOM generates a simple Software Bill of Materials
func (s *SecurityArtifactsService) GenerateSBOM(_ context.Context, filePath string) (string, error) {
	sbomPath := filePath + ".sbom.json"

	// Simple SBOM structure
	sbom := map[string]interface{}{
		"bomFormat":   "CycloneDX",
		"specVersion": "1.5",
		"version":     1,
		"metadata": map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"component": map[string]interface{}{
				"type": "application",
				"name": filepath.Base(filePath),
			},
		},
		"components": []map[string]interface{}{
			{
				"type":    "file",
				"name":    filepath.Base(filePath),
				"version": "unknown",
				"hashes": []map[string]string{
					{
						"alg":     "SHA-256",
						"content": s.mustComputeSHA256(filePath),
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(sbom, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal SBOM: %w", err)
	}

	if err := os.WriteFile(sbomPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write SBOM file: %w", err)
	}

	return sbomPath, nil
}

// GenerateProvenance generates SLSA provenance attestation
func (s *SecurityArtifactsService) GenerateProvenance(_ context.Context, filePath string) (string, error) {
	provenancePath := filePath + ".provenance.json"

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	// Simple SLSA provenance structure
	provenance := map[string]interface{}{
		"_type": "https://in-toto.io/Statement/v0.1",
		"subject": []map[string]interface{}{
			{
				"name": filepath.Base(filePath),
				"digest": map[string]string{
					"sha256": s.mustComputeSHA256(filePath),
					"sha512": s.mustComputeSHA512(filePath),
				},
			},
		},
		"predicateType": "https://slsa.dev/provenance/v0.2",
		"predicate": map[string]interface{}{
			"builder": map[string]string{
				"id": "https://github.com/ochairo/potions",
			},
			"buildType": "https://github.com/ochairo/potions@v1",
			"metadata": map[string]interface{}{
				"buildStartedOn":  time.Now().UTC().Format(time.RFC3339),
				"buildFinishedOn": time.Now().UTC().Format(time.RFC3339),
				"completeness": map[string]bool{
					"parameters":  true,
					"environment": false,
					"materials":   false,
				},
				"reproducible": false,
			},
			"materials": []map[string]interface{}{
				{
					"uri": "pkg:generic/" + filepath.Base(filePath),
					"digest": map[string]string{
						"sha256": s.mustComputeSHA256(filePath),
					},
				},
			},
		},
	}

	// Add file size
	if fileInfo != nil {
		if subject, ok := provenance["subject"].([]map[string]interface{}); ok && len(subject) > 0 {
			subject[0]["size"] = fileInfo.Size()
		}
	}

	data, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal provenance: %w", err)
	}

	if err := os.WriteFile(provenancePath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write provenance file: %w", err)
	}

	return provenancePath, nil
}

// computeSHA256 computes SHA256 hash of a file
func (s *SecurityArtifactsService) computeSHA256(filePath string) (string, error) {
	//nolint:gosec // G304: filePath is function parameter for checksum generation
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// computeSHA512 computes SHA512 hash of a file
func (s *SecurityArtifactsService) computeSHA512(filePath string) (string, error) {
	//nolint:gosec // G304: filePath is function parameter for checksum generation
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// mustComputeSHA256 computes SHA256 or returns empty string on error
func (s *SecurityArtifactsService) mustComputeSHA256(filePath string) string {
	hash, err := s.computeSHA256(filePath)
	if err != nil {
		return ""
	}
	return hash
}

// mustComputeSHA512 computes SHA512 or returns empty string on error
func (s *SecurityArtifactsService) mustComputeSHA512(filePath string) string {
	hash, err := s.computeSHA512(filePath)
	if err != nil {
		return ""
	}
	return hash
}
