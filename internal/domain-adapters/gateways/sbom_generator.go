package gateways

import (
	"context"
	"crypto/sha256"
	"debug/elf"
	"debug/macho"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

// sbomGenerator implements SBOM generation using pure Go
// Uses debug/elf and debug/macho packages - no syft binary required
type sbomGenerator struct{}

// NewSBOMGenerator creates a new SBOM generator gateway
//
//nolint:revive // unexported-return: Intentionally returns concrete type for testability
func NewSBOMGenerator() *sbomGenerator {
	return &sbomGenerator{}
}

// GenerateSBOM generates a Software Bill of Materials for an artifact
func (g *sbomGenerator) GenerateSBOM(_ context.Context, artifact *entities.Artifact) (*entities.SBOM, error) {
	if artifact == nil {
		return nil, fmt.Errorf("artifact cannot be nil")
	}

	// Validate artifact path
	if artifact.Path == "" {
		return nil, fmt.Errorf("artifact path cannot be empty")
	}

	if _, err := os.Stat(artifact.Path); err != nil {
		return nil, fmt.Errorf("artifact path does not exist: %w", err)
	}

	// Calculate SHA256 hash of the artifact
	hash, err := g.calculateHash(artifact.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate artifact hash: %w", err)
	}

	// Create main component for the artifact itself
	components := []entities.Component{
		{
			Type:    "application",
			Name:    artifact.Name,
			Version: artifact.Version,
			Hashes: []entities.Hash{
				{
					Algorithm: "SHA-256",
					Value:     hash,
				},
			},
		},
	}

	// If binary, extract dependencies
	if artifact.Type == "binary" || g.isBinary(artifact.Path) {
		deps, err := g.extractBinaryDependencies(artifact.Path, artifact.Platform)
		if err != nil {
			// Log warning but don't fail - dependency extraction is best-effort
			// In production, you'd use a proper logger here
			_ = err
		} else {
			components = append(components, deps...)
		}
	}

	return &entities.SBOM{
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.4",
		Version:     1,
		Components:  components,
		Metadata: entities.Metadata{
			Timestamp: time.Now(),
			Tools: []entities.Tool{
				{
					Name:    "potions",
					Version: "1.0.0",
				},
			},
		},
	}, nil
}

// isBinary attempts to determine if a file is a binary
func (g *sbomGenerator) isBinary(path string) bool {
	//nolint:gosec // G304: path is from filepath.Walk for SBOM generation
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	// Read first 512 bytes for magic number detection
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}

	buf = buf[:n]

	// Check for ELF magic number (0x7F 'E' 'L' 'F')
	if len(buf) >= 4 && buf[0] == 0x7F && buf[1] == 'E' && buf[2] == 'L' && buf[3] == 'F' {
		return true
	}

	// Check for Mach-O magic numbers
	if len(buf) >= 4 {
		// 32-bit Mach-O: 0xFEEDFACE (big-endian) or 0xCEFAEDFE (little-endian)
		// 64-bit Mach-O: 0xFEEDFACF (big-endian) or 0xCFFAEDFE (little-endian)
		magic := uint32(buf[0])<<24 | uint32(buf[1])<<16 | uint32(buf[2])<<8 | uint32(buf[3])
		if magic == 0xFEEDFACE || magic == 0xCEFAEDFE || magic == 0xFEEDFACF || magic == 0xCFFAEDFE {
			return true
		}

		// Universal binary: 0xCAFEBABE
		if magic == 0xCAFEBABE {
			return true
		}
	}

	return false
}

// extractBinaryDependencies extracts dependencies from a binary file
func (g *sbomGenerator) extractBinaryDependencies(binaryPath, platform string) ([]entities.Component, error) {
	// Determine platform if not provided
	if platform == "" {
		platform = g.detectPlatform(binaryPath)
	}

	switch {
	case strings.HasPrefix(platform, "linux"):
		return g.extractELFDependencies(binaryPath)
	case strings.HasPrefix(platform, "darwin"):
		return g.extractMachODependencies(binaryPath)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

// detectPlatform attempts to detect the platform from the binary
func (g *sbomGenerator) detectPlatform(binaryPath string) string {
	// Try ELF first
	if _, err := elf.Open(binaryPath); err == nil {
		return "linux"
	}

	// Try Mach-O
	if _, err := macho.Open(binaryPath); err == nil {
		return "darwin"
	}

	return "unknown"
}

// extractELFDependencies extracts dependencies from an ELF binary
func (g *sbomGenerator) extractELFDependencies(binaryPath string) ([]entities.Component, error) {
	f, err := elf.Open(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ELF file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	components := make([]entities.Component, 0)
	seen := make(map[string]bool)

	// Extract imported libraries
	libs, err := f.ImportedLibraries()
	if err != nil {
		return nil, fmt.Errorf("failed to extract imported libraries: %w", err)
	}

	for _, lib := range libs {
		if lib == "" || seen[lib] {
			continue
		}
		seen[lib] = true

		// Parse library name and version if possible
		name, version := g.parseLibraryNameVersion(lib)

		components = append(components, entities.Component{
			Type:    "library",
			Name:    name,
			Version: version,
			Hashes:  []entities.Hash{}, // Could resolve library path and hash it
		})
	}

	// Extract imported symbols (for additional context)
	symbols, err := f.ImportedSymbols()
	if err == nil && len(symbols) > 0 {
		// Add symbol information to metadata if needed
		// For now, just count them for completeness
		_ = symbols
	}

	return components, nil
}

// extractMachODependencies extracts dependencies from a Mach-O binary
func (g *sbomGenerator) extractMachODependencies(binaryPath string) ([]entities.Component, error) {
	f, err := macho.Open(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Mach-O file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	components := make([]entities.Component, 0)
	seen := make(map[string]bool)

	// Extract imported libraries
	libs, err := f.ImportedLibraries()
	if err != nil {
		return nil, fmt.Errorf("failed to extract imported libraries: %w", err)
	}

	for _, lib := range libs {
		if lib == "" || seen[lib] {
			continue
		}
		seen[lib] = true

		// Parse library name and version
		name, version := g.parseLibraryNameVersion(lib)

		components = append(components, entities.Component{
			Type:    "library",
			Name:    name,
			Version: version,
			Hashes:  []entities.Hash{},
		})
	}

	return components, nil
}

// parseLibraryNameVersion attempts to parse library name and version
func (g *sbomGenerator) parseLibraryNameVersion(libPath string) (name, version string) {
	// Get basename
	base := filepath.Base(libPath)

	// Remove common library prefixes
	base = strings.TrimPrefix(base, "lib")

	// Try to extract version from common patterns
	// Examples:
	//   libssl.so.1.1 -> ssl, 1.1
	//   libcrypto.so.3 -> crypto, 3
	//   libSystem.B.dylib -> System.B, unknown
	//   /usr/lib/libz.1.dylib -> z, 1

	// Split by dots
	parts := strings.Split(base, ".")

	if len(parts) >= 2 {
		// Check if last parts are version numbers
		versionParts := make([]string, 0)
		nameParts := make([]string, 0)

		foundVersion := false
		for i := len(parts) - 1; i >= 0; i-- {
			part := parts[i]

			// Skip common extensions
			if part == "so" || part == "dylib" || part == "dll" {
				continue
			}

			// Check if it's a number (version)
			if g.isNumeric(part) && !foundVersion {
				versionParts = append([]string{part}, versionParts...)
			} else {
				foundVersion = true
				nameParts = append([]string{part}, nameParts...)
			}
		}

		if len(nameParts) > 0 {
			name = strings.Join(nameParts, ".")
		} else {
			name = base
		}

		if len(versionParts) > 0 {
			version = strings.Join(versionParts, ".")
		} else {
			version = "unknown"
		}
	} else {
		name = base
		version = "unknown"
	}

	return name, version
}

// isNumeric checks if a string is numeric
func (g *sbomGenerator) isNumeric(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// calculateHash calculates SHA256 hash of a file
func (g *sbomGenerator) calculateHash(filePath string) (string, error) {
	//nolint:gosec // G304: filePath is from filepath.Walk for SBOM generation
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	//nolint:errcheck // Defer close
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
