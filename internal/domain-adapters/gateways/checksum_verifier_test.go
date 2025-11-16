package gateways

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestVerifyChecksum tests SHA256 checksum verification
func TestVerifyChecksum(t *testing.T) {
	// Create test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("Hello, World! This is a test file for checksum verification.")
	if err := os.WriteFile(testFile, content, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	verifier := NewChecksumVerifier()

	// Calculate checksum first
	actualSum, err := verifier.CalculateChecksum(testFile)
	if err != nil {
		t.Fatalf("CalculateChecksum() error = %v", err)
	}

	if actualSum == "" {
		t.Error("CalculateChecksum() returned empty checksum")
	}

	if len(actualSum) != 64 {
		t.Errorf("CalculateChecksum() returned checksum length = %d, want 64 (SHA256 hex)", len(actualSum))
	}

	// Test valid checksum verification
	t.Run("valid checksum", func(t *testing.T) {
		err := verifier.VerifyChecksum(context.Background(), testFile, actualSum)
		if err != nil {
			t.Errorf("VerifyChecksum() with valid checksum error = %v", err)
		}
	})

	// Test invalid checksum
	t.Run("invalid checksum", func(t *testing.T) {
		invalidSum := "0000000000000000000000000000000000000000000000000000000000000000"
		err := verifier.VerifyChecksum(context.Background(), testFile, invalidSum)
		if err == nil {
			t.Error("VerifyChecksum() with invalid checksum should return error")
		}
	})

	// Test non-existent file
	t.Run("non-existent file", func(t *testing.T) {
		err := verifier.VerifyChecksum(context.Background(), "/nonexistent/file.txt", actualSum)
		if err == nil {
			t.Error("VerifyChecksum() with non-existent file should return error")
		}
	})
}

// TestCalculateChecksum tests SHA256 checksum calculation
func TestCalculateChecksum(t *testing.T) {
	tests := []struct {
		name         string
		content      []byte
		wantChecksum string // Known SHA256 hash
	}{
		{
			name:         "empty file",
			content:      []byte(""),
			wantChecksum: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", // SHA256 of empty string
		},
		{
			name:         "simple content",
			content:      []byte("Hello, World!"),
			wantChecksum: "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test.txt")

			if err := os.WriteFile(testFile, tt.content, 0600); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			verifier := NewChecksumVerifier()
			checksum, err := verifier.CalculateChecksum(testFile)
			if err != nil {
				t.Errorf("CalculateChecksum() error = %v", err)
				return
			}

			if checksum != tt.wantChecksum {
				t.Errorf("CalculateChecksum() = %v, want %v", checksum, tt.wantChecksum)
			}
		})
	}
}

// TestChecksumConsistency tests that checksum calculation is consistent
func TestChecksumConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("Test content for consistency check")
	if err := os.WriteFile(testFile, content, 0600); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	verifier := NewChecksumVerifier()

	// Calculate checksum multiple times
	checksum1, err := verifier.CalculateChecksum(testFile)
	if err != nil {
		t.Fatalf("First CalculateChecksum() error = %v", err)
	}

	checksum2, err := verifier.CalculateChecksum(testFile)
	if err != nil {
		t.Fatalf("Second CalculateChecksum() error = %v", err)
	}

	if checksum1 != checksum2 {
		t.Errorf("Checksum calculation is not consistent: %v != %v", checksum1, checksum2)
	}
}

// TestLargeFileChecksum tests checksum calculation for larger files
func TestLargeFileChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.bin")

	// Create a 1MB file
	size := 1024 * 1024 // 1 MB
	content := make([]byte, size)
	for i := range content {
		content[i] = byte(i % 256)
	}

	if err := os.WriteFile(testFile, content, 0600); err != nil {
		t.Fatalf("Failed to create large test file: %v", err)
	}

	verifier := NewChecksumVerifier()

	// Calculate checksum
	checksum, err := verifier.CalculateChecksum(testFile)
	if err != nil {
		t.Errorf("CalculateChecksum() for large file error = %v", err)
		return
	}

	if checksum == "" {
		t.Error("CalculateChecksum() for large file returned empty checksum")
	}

	// Verify the checksum
	err = verifier.VerifyChecksum(context.Background(), testFile, checksum)
	if err != nil {
		t.Errorf("VerifyChecksum() for large file error = %v", err)
	}
}
