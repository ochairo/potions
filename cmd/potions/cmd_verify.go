package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ochairo/potions/internal/domain-adapters/gateways"
)

func runVerify(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	var (
		checksumFile = fs.String("checksum", "", "Checksum file to verify against (.sha256 or .sha512)")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: potions verify <file> [options]

Verify checksums for a file using pure Go implementation.

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  potions verify kubectl-1.28.0-darwin-arm64.tar.gz
  potions verify kubectl-1.28.0-darwin-arm64.tar.gz --checksum kubectl-1.28.0-darwin-arm64.tar.gz.sha256
`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Error: file path is required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	filePath := fs.Arg(0)

	// Execute verification following Clean Architecture
	if err := executeVerify(ctx, filePath, *checksumFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func executeVerify(ctx context.Context, filePath, checksumFile string) error {
	// Layer 1: Create gateway (Infrastructure)
	verifier := gateways.NewChecksumVerifier()

	// Auto-detect checksum file if not specified
	if checksumFile == "" {
		if _, err := os.Stat(filePath + ".sha256"); err == nil {
			checksumFile = filePath + ".sha256"
		} else if _, err := os.Stat(filePath + ".sha512"); err == nil {
			checksumFile = filePath + ".sha512"
		} else {
			return fmt.Errorf("no checksum file found (tried %s.sha256 and %s.sha512)", filePath, filePath)
		}
	}

	fmt.Printf("🔍 Verifying %s\n", filepath.Base(filePath))
	fmt.Printf("📋 Using checksum: %s\n", filepath.Base(checksumFile))

	// Read expected checksum from file
	//nolint:gosec // G304: checksumFile is user-provided path for verification
	data, err := os.ReadFile(checksumFile)
	if err != nil {
		return fmt.Errorf("failed to read checksum file: %w", err)
	}

	// Parse checksum file (format: "hash  filename")
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return fmt.Errorf("invalid checksum file format")
	}
	expectedChecksum := parts[0]

	// Verify using the gateway (pure Go crypto/sha256)
	if err := verifier.VerifyChecksum(ctx, filePath, expectedChecksum); err != nil {
		fmt.Printf("❌ Verification FAILED: %v\n", err)
		return err
	}

	fmt.Printf("✅ Verification successful!\n")
	return nil
}
