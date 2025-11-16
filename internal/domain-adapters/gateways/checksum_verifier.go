package gateways

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// checksumVerifier implements checksum verification using pure Go
type checksumVerifier struct{}

// NewChecksumVerifier creates a new checksum verifier
//
//nolint:revive // unexported-return: Intentionally returns concrete type for testability
func NewChecksumVerifier() *checksumVerifier {
	return &checksumVerifier{}
}

// VerifyChecksum verifies a file's SHA256 checksum
// Pure Go implementation - no external sha256sum binary needed
func (v *checksumVerifier) VerifyChecksum(_ context.Context, filePath, expectedSum string) error {
	//nolint:gosec // G304: File path is user-provided for checksum verification
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	//nolint:errcheck // Defer close on read-only file
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}

	actualSum := hex.EncodeToString(h.Sum(nil))

	if actualSum != expectedSum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedSum, actualSum)
	}

	return nil
}

// CalculateChecksum calculates the SHA256 checksum of a file
func (v *checksumVerifier) CalculateChecksum(filePath string) (string, error) {
	//nolint:gosec // G304: File path is user-provided for checksum calculation
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	//nolint:errcheck // Defer close on read-only file
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
