package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ochairo/potions/internal/domain-adapters/gateways"
	"github.com/ochairo/potions/internal/external-adapters/attestation"
	"github.com/ochairo/potions/internal/external-adapters/cosign"
	"github.com/ochairo/potions/internal/external-adapters/gpg"
)

func runVerify(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	var (
		checksumFile   = fs.String("checksum", "", "Checksum file to verify against (.sha256 or .sha512)")
		gpgSig         = fs.String("gpg-sig", "", "GPG signature file (.asc)")
		gpgKeyIDs      = fs.String("gpg-key-ids", "", "Comma-separated GPG key IDs to import")
		gpgKeysURL     = fs.String("gpg-keys-url", "", "URL to KEYS file for GPG verification")
		cosignSig      = fs.String("cosign-sig", "", "Cosign signature file (.sig)")
		cosignCert     = fs.String("cosign-cert", "", "Cosign certificate file (.pem)")
		cosignIdentity = fs.String("cosign-identity", "", "Expected certificate identity")
		attestFile     = fs.String("attest-file", "", "Attestation file (.attestation.jsonl)")
		attestOwner    = fs.String("owner", "", "GitHub repository owner (for attestations)")
		attestRepo     = fs.String("repo", "", "GitHub repository name (for attestations)")
		verifyAll      = fs.Bool("all", false, "Verify all available signatures automatically")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: potions verify <file> [options]

Verify checksums, signatures, and attestations for build artifacts.

Supports multiple verification methods:
  - Checksums: SHA256 and SHA512 verification
  - GPG: PGP signature verification
  - Cosign: Sigstore keyless signature verification
  - GitHub Attestations: SLSA provenance verification

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  # Verify checksum
  potions verify mypackage.tar.gz --checksum mypackage.tar.gz.sha256

  # Verify GPG signature
  potions verify kubectl.tar.gz --gpg-sig kubectl.tar.gz.asc --gpg-key-ids 7F92E05B31093BEF

  # Verify Cosign signature
  potions verify helm.tar.gz --cosign-sig helm.tar.gz.sig --cosign-cert helm.tar.gz.pem

  # Verify all available signatures
  potions verify package.tar.gz --all
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
	if err := executeVerify(ctx, filePath, *checksumFile, *gpgSig, *gpgKeyIDs, *gpgKeysURL,
		*cosignSig, *cosignCert, *cosignIdentity, *attestFile, *attestOwner, *attestRepo, *verifyAll); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func executeVerify(ctx context.Context, filePath, checksumFile, gpgSig, gpgKeyIDs, gpgKeysURL,
	cosignSig, cosignCert, cosignIdentity, attestFile, attestOwner, attestRepo string, verifyAll bool) error {

	verified := 0
	failed := 0

	// Auto-detect files if --all is specified
	if verifyAll {
		if checksumFile == "" {
			if fileExists(filePath + ".sha256") {
				checksumFile = filePath + ".sha256"
			} else if fileExists(filePath + ".sha512") {
				checksumFile = filePath + ".sha512"
			}
		}
		if gpgSig == "" && fileExists(filePath+".asc") {
			gpgSig = filePath + ".asc"
		}
		if cosignSig == "" && fileExists(filePath+".sig") && fileExists(filePath+".pem") {
			cosignSig = filePath + ".sig"
			cosignCert = filePath + ".pem"
		}
		if attestFile == "" && fileExists(filePath+".attestation.jsonl") {
			attestFile = filePath + ".attestation.jsonl"
		}
	}

	fmt.Printf("🔍 Verifying %s\n\n", filepath.Base(filePath))

	// Verify checksum
	if checksumFile != "" {
		fmt.Printf("📋 Verifying checksum...\n")
		if err := verifyChecksum(ctx, filePath, checksumFile); err != nil {
			fmt.Printf("❌ Checksum verification FAILED: %v\n\n", err)
			failed++
		} else {
			fmt.Printf("✅ Checksum verified\n\n")
			verified++
		}
	}

	// Verify GPG signature
	if gpgSig != "" {
		fmt.Printf("🔐 Verifying GPG signature...\n")
		if err := verifyGPGSignature(ctx, filePath, gpgSig, gpgKeyIDs, gpgKeysURL); err != nil {
			fmt.Printf("❌ GPG signature verification FAILED: %v\n\n", err)
			failed++
		} else {
			fmt.Printf("✅ GPG signature verified\n\n")
			verified++
		}
	}

	// Verify Cosign signature
	if cosignSig != "" {
		fmt.Printf("🔏 Verifying Cosign signature...\n")
		if err := verifyCosignSignature(ctx, filePath, cosignSig, cosignCert, cosignIdentity); err != nil {
			fmt.Printf("❌ Cosign signature verification FAILED: %v\n\n", err)
			failed++
		} else {
			fmt.Printf("✅ Cosign signature verified\n\n")
			verified++
		}
	}

	// Verify GitHub attestation
	if attestFile != "" {
		fmt.Printf("📜 Verifying GitHub attestation...\n")
		if err := verifyAttestation(ctx, filePath, attestFile, attestOwner, attestRepo); err != nil {
			fmt.Printf("❌ Attestation verification FAILED: %v\n\n", err)
			failed++
		} else {
			fmt.Printf("✅ Attestation verified\n\n")
			verified++
		}
	}

	// Print summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("✅ Verified: %d checks\n", verified)
	if failed > 0 {
		fmt.Printf("❌ Failed: %d checks\n", failed)
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if failed > 0 {
		return fmt.Errorf("%d verification checks failed", failed)
	}

	if verified == 0 {
		return fmt.Errorf("no verification checks performed (specify --checksum, --gpg-sig, --cosign-sig, or --attest-file)")
	}

	return nil
}

func verifyChecksum(ctx context.Context, filePath, checksumFile string) error {
	// Layer 1: Create gateway (Infrastructure)
	verifier := gateways.NewChecksumVerifier()

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
		return err
	}

	return nil
}

func verifyGPGSignature(ctx context.Context, filePath, gpgSig, gpgKeyIDs, gpgKeysURL string) error {
	gpgVerifier := gpg.NewVerifier()

	// Import keys if specified
	if gpgKeyIDs != "" {
		keyIDList := strings.Split(gpgKeyIDs, ",")
		if err := gpgVerifier.ImportKeys(ctx, keyIDList); err != nil {
			return fmt.Errorf("failed to import GPG keys: %w", err)
		}
	} else if gpgKeysURL != "" {
		if err := gpgVerifier.ImportKeysFromURL(ctx, gpgKeysURL); err != nil {
			return fmt.Errorf("failed to import GPG keys from URL: %w", err)
		}
	}

	if gpgVerifier.GetKeyringSize() == 0 {
		return fmt.Errorf("no GPG keys imported for verification (use --gpg-key-ids or --gpg-keys-url)")
	}

	if err := gpgVerifier.VerifySignatureFromFile(filePath, gpgSig); err != nil {
		return err
	}

	return nil
}

func verifyCosignSignature(ctx context.Context, filePath, cosignSig, cosignCert, cosignIdentity string) error {
	if !cosign.IsCosignInstalled() {
		return fmt.Errorf("cosign not installed (install from https://docs.sigstore.dev/cosign/installation/)")
	}

	cosignVerifier := cosign.NewVerifier()

	if cosignCert == "" {
		return fmt.Errorf("cosign certificate required (use --cosign-cert)")
	}

	var err error
	if cosignIdentity != "" {
		err = cosignVerifier.VerifySignatureWithCertIdentity(ctx, filePath, cosignSig, cosignCert, cosignIdentity)
	} else {
		err = cosignVerifier.VerifySignature(ctx, filePath, cosignSig, cosignCert)
	}

	if err != nil {
		return err
	}

	return nil
}

func verifyAttestation(ctx context.Context, filePath, attestFile, attestOwner, attestRepo string) error {
	if !attestation.IsGHCLIInstalled() {
		return fmt.Errorf("gh CLI not installed (install from https://cli.github.com)")
	}

	attestVerifier := attestation.NewVerifier()

	var err error
	if attestOwner != "" && attestRepo != "" {
		err = attestVerifier.VerifyAttestationWithGH(ctx, filePath, attestOwner, attestRepo)
	} else {
		err = attestVerifier.VerifyAttestation(ctx, filePath, attestFile)
	}

	if err != nil {
		return err
	}

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
