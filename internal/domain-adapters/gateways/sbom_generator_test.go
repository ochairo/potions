package gateways

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

// TestGenerateSBOM tests basic SBOM generation
func TestGenerateSBOM(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-binary")

	// Create a simple test file
	content := []byte("test binary content")
	//nolint:gosec // G306: Test executable needs 0700 permissions
	if err := os.WriteFile(testFile, content, 0700); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	generator := NewSBOMGenerator()

	artifact := &entities.Artifact{
		Name:    "test-package",
		Version: "1.0.0",
		Path:    testFile,
		Type:    "binary",
	}

	sbom, err := generator.GenerateSBOM(context.Background(), artifact)
	if err != nil {
		t.Fatalf("GenerateSBOM() error = %v", err)
	}

	if sbom == nil {
		t.Fatal("GenerateSBOM() returned nil")
	}

	// Verify SBOM format
	if sbom.BOMFormat != "CycloneDX" {
		t.Errorf("BOMFormat = %v, want CycloneDX", sbom.BOMFormat)
	}

	if sbom.SpecVersion != "1.4" {
		t.Errorf("SpecVersion = %v, want 1.4", sbom.SpecVersion)
	}

	// Verify components
	if len(sbom.Components) == 0 {
		t.Error("Components is empty, expected at least the main artifact")
	}

	// Verify main component
	mainComponent := sbom.Components[0]
	if mainComponent.Type != "application" {
		t.Errorf("Main component type = %v, want application", mainComponent.Type)
	}

	if mainComponent.Name != "test-package" {
		t.Errorf("Main component name = %v, want test-package", mainComponent.Name)
	}

	if mainComponent.Version != "1.0.0" {
		t.Errorf("Main component version = %v, want 1.0.0", mainComponent.Version)
	}

	// Verify hash
	if len(mainComponent.Hashes) == 0 {
		t.Error("Main component hashes is empty, expected SHA256 hash")
	} else {
		hash := mainComponent.Hashes[0]
		if hash.Algorithm != "SHA-256" {
			t.Errorf("Hash algorithm = %v, want SHA-256", hash.Algorithm)
		}
		if hash.Value == "" {
			t.Error("Hash value is empty")
		}
	}

	// Verify metadata
	if sbom.Metadata.Timestamp.IsZero() {
		t.Error("Metadata timestamp is zero")
	}

	if len(sbom.Metadata.Tools) == 0 {
		t.Error("Metadata tools is empty")
	}
}

// TestGenerateSBOM_NilArtifact tests error handling for nil artifact
func TestGenerateSBOM_NilArtifact(t *testing.T) {
	generator := NewSBOMGenerator()

	_, err := generator.GenerateSBOM(context.Background(), nil)
	if err == nil {
		t.Error("GenerateSBOM() with nil artifact should return error")
	}
}

// TestGenerateSBOM_EmptyPath tests error handling for empty path
func TestGenerateSBOM_EmptyPath(t *testing.T) {
	generator := NewSBOMGenerator()

	artifact := &entities.Artifact{
		Name:    "test",
		Version: "1.0.0",
		Path:    "",
	}

	_, err := generator.GenerateSBOM(context.Background(), artifact)
	if err == nil {
		t.Error("GenerateSBOM() with empty path should return error")
	}
}

// TestGenerateSBOM_NonExistentPath tests error handling for non-existent file
func TestGenerateSBOM_NonExistentPath(t *testing.T) {
	generator := NewSBOMGenerator()

	artifact := &entities.Artifact{
		Name:    "test",
		Version: "1.0.0",
		Path:    "/nonexistent/file",
	}

	_, err := generator.GenerateSBOM(context.Background(), artifact)
	if err == nil {
		t.Error("GenerateSBOM() with non-existent path should return error")
	}
}

// TestParseLibraryNameVersion tests library name and version parsing
func TestParseLibraryNameVersion(t *testing.T) {
	generator := NewSBOMGenerator()

	tests := []struct {
		libPath     string
		wantName    string
		wantVersion string
	}{
		{
			libPath:     "libssl.so.1.1",
			wantName:    "ssl",
			wantVersion: "1.1",
		},
		{
			libPath:     "libcrypto.so.3",
			wantName:    "crypto",
			wantVersion: "3",
		},
		{
			libPath:     "/usr/lib/libz.1.dylib",
			wantName:    "z",
			wantVersion: "1",
		},
		{
			libPath:     "libSystem.B.dylib",
			wantName:    "System.B",
			wantVersion: "unknown",
		},
		{
			libPath:     "libpthread.so",
			wantName:    "pthread",
			wantVersion: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.libPath, func(t *testing.T) {
			name, version := generator.parseLibraryNameVersion(tt.libPath)

			if name != tt.wantName {
				t.Errorf("parseLibraryNameVersion(%q) name = %v, want %v", tt.libPath, name, tt.wantName)
			}

			if version != tt.wantVersion {
				t.Errorf("parseLibraryNameVersion(%q) version = %v, want %v", tt.libPath, version, tt.wantVersion)
			}
		})
	}
}

// TestIsBinary tests binary detection
func TestIsBinary(t *testing.T) {
	generator := NewSBOMGenerator()
	tmpDir := t.TempDir()

	tests := []struct {
		name       string
		content    []byte
		wantBinary bool
	}{
		{
			name:       "ELF binary",
			content:    []byte{0x7F, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00},
			wantBinary: true,
		},
		{
			name:       "Mach-O 64-bit",
			content:    []byte{0xCF, 0xFA, 0xED, 0xFE},
			wantBinary: true,
		},
		{
			name:       "text file",
			content:    []byte("This is a text file\n"),
			wantBinary: false,
		},
		{
			name:       "empty file",
			content:    []byte{},
			wantBinary: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(testFile, tt.content, 0600); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			isBinary := generator.isBinary(testFile)
			if isBinary != tt.wantBinary {
				t.Errorf("isBinary(%q) = %v, want %v", tt.name, isBinary, tt.wantBinary)
			}
		})
	}
}

// TestIsNumeric tests numeric string detection
func TestIsNumeric(t *testing.T) {
	generator := NewSBOMGenerator()

	tests := []struct {
		input string
		want  bool
	}{
		{"123", true},
		{"0", true},
		{"1", true},
		{"abc", false},
		{"", false},
		{"1.2", false},
		{"12a", false},
		{"a12", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := generator.isNumeric(tt.input)
			if got != tt.want {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
