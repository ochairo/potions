package gateways

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Test unsupported platform
func TestBinaryAnalyzer_UnsupportedPlatform(t *testing.T) {
	analyzer := NewBinaryAnalyzerGateway()

	_, err := analyzer.AnalyzeBinaryHardening(context.Background(), "/tmp/test", "windows-amd64")

	if err == nil {
		t.Fatal("Expected error for unsupported platform, got nil")
	}

	if !stringContainsSubstr(err.Error(), "unsupported platform") {
		t.Errorf("Expected 'unsupported platform' error, got: %v", err)
	}
}

// Test analyzing nonexistent file
func TestBinaryAnalyzer_NonexistentFile(t *testing.T) {
	analyzer := NewBinaryAnalyzerGateway()

	_, err := analyzer.AnalyzeBinaryHardening(context.Background(), "/nonexistent/binary", "linux-amd64")

	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}
}

// Test analyzing non-binary file
func TestBinaryAnalyzer_InvalidBinaryFile(t *testing.T) {
	analyzer := NewBinaryAnalyzerGateway()
	tmpDir := t.TempDir()

	textFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(textFile, []byte("not a binary"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := analyzer.AnalyzeBinaryHardening(context.Background(), textFile, "linux-amd64")

	if err == nil {
		t.Fatal("Expected error for invalid binary file, got nil")
	}
}

// Test hardening score calculation
func TestBinaryAnalyzer_CalculateHardeningScore(t *testing.T) {
	analyzer := NewBinaryAnalyzerGateway()

	tests := []struct {
		name     string
		features entities.HardeningFeatures
		wantMin  float64
		wantMax  float64
	}{
		{
			name: "all features enabled",
			features: entities.HardeningFeatures{
				PIEEnabled:      true,
				StackCanaries:   true,
				NXBit:           true,
				RELRO:           "full",
				FortifySource:   true,
				CodeSigned:      true,
				HardenedRuntime: true,
			},
			wantMin: 9.0,
			wantMax: 10.0,
		},
		{
			name: "no features enabled",
			features: entities.HardeningFeatures{
				PIEEnabled:      false,
				StackCanaries:   false,
				NXBit:           false,
				RELRO:           "disabled",
				FortifySource:   false,
				CodeSigned:      false,
				HardenedRuntime: false,
			},
			wantMin: 0.0,
			wantMax: 2.0,
		},
		{
			name: "partial features",
			features: entities.HardeningFeatures{
				PIEEnabled:    true,
				StackCanaries: true,
				NXBit:         true,
				RELRO:         "partial",
			},
			wantMin: 4.0,
			wantMax: 7.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := analyzer.calculateHardeningScore(tt.features)

			if score.Score < tt.wantMin || score.Score > tt.wantMax {
				t.Errorf("Score = %.1f, want between %.1f and %.1f", score.Score, tt.wantMin, tt.wantMax)
			}

			if score.Total <= 0 {
				t.Error("Total checks should be > 0")
			}

			if score.Passed < 0 || score.Passed > score.Total {
				t.Errorf("Passed = %d should be between 0 and %d", score.Passed, score.Total)
			}

			if score.Percentage < 0 || score.Percentage > 100 {
				t.Errorf("Percentage = %d should be between 0 and 100", score.Percentage)
			}
		})
	}
}

// Test Linux binary analysis with mock ELF (we can't create real ELF without external tools)
// This tests the error path when file doesn't exist or is invalid
func TestBinaryAnalyzer_AnalyzeLinuxBinary_InvalidFile(t *testing.T) {
	analyzer := NewBinaryAnalyzerGateway()
	tmpDir := t.TempDir()

	// Create invalid "binary"
	fakeBinary := filepath.Join(tmpDir, "fake")
	if err := os.WriteFile(fakeBinary, []byte("not an ELF"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := analyzer.analyzeLinuxBinary(fakeBinary)

	if err == nil {
		t.Fatal("Expected error for invalid ELF file, got nil")
	}

	if !stringContainsSubstr(err.Error(), "failed to open ELF file") {
		t.Errorf("Expected ELF error, got: %v", err)
	}
}

// Test Darwin binary analysis with mock Mach-O
func TestBinaryAnalyzer_AnalyzeDarwinBinary_InvalidFile(t *testing.T) {
	analyzer := NewBinaryAnalyzerGateway()
	tmpDir := t.TempDir()

	// Create invalid "binary"
	fakeBinary := filepath.Join(tmpDir, "fake")
	if err := os.WriteFile(fakeBinary, []byte("not a Mach-O"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := analyzer.analyzeDarwinBinary(fakeBinary)

	if err == nil {
		t.Fatal("Expected error for invalid Mach-O file, got nil")
	}

	if !stringContainsSubstr(err.Error(), "failed to open Mach-O file") {
		t.Errorf("Expected Mach-O error, got: %v", err)
	}
}

// Test platform detection from path
func TestBinaryAnalyzer_PlatformDetection(t *testing.T) {
	analyzer := NewBinaryAnalyzerGateway()

	tests := []struct {
		platform    string
		expectError bool
		errorText   string
	}{
		{"linux-amd64", true, "failed to open ELF file"},
		{"linux-arm64", true, "failed to open ELF file"},
		{"darwin-amd64", true, "failed to open Mach-O file"},
		{"darwin-arm64", true, "failed to open Mach-O file"},
		{"windows-amd64", true, "unsupported platform"},
		{"freebsd-amd64", true, "unsupported platform"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			_, err := analyzer.AnalyzeBinaryHardening(context.Background(), "/nonexistent", tt.platform)

			if tt.expectError && err == nil {
				t.Error("Expected error, got nil")
			}

			if err != nil && tt.errorText != "" {
				if !stringContainsSubstr(err.Error(), tt.errorText) {
					t.Errorf("Error = %v, want to contain %s", err, tt.errorText)
				}
			}
		})
	}
}

// Helper function
func stringContainsSubstr(s, substr string) bool {
	return len(s) >= len(substr) && stringIndexOf(s, substr) >= 0
}

func stringIndexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
