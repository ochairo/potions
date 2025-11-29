// Package cosign provides Cosign/Sigstore signature verification capabilities.
package cosign

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Verifier implements Cosign signature verification
type Verifier struct{}

// NewVerifier creates a new Cosign verifier
func NewVerifier() *Verifier {
	return &Verifier{}
}

// VerifySignature verifies a Cosign signature using keyless verification
func (v *Verifier) VerifySignature(ctx context.Context, filePath, signaturePath, certPath string) error {
	// Check if Cosign is installed
	if _, err := exec.LookPath("cosign"); err != nil {
		return fmt.Errorf("cosign not installed: %w (install from https://github.com/sigstore/cosign)", err)
	}

	// Verify the file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	// Verify signature file exists
	if _, err := os.Stat(signaturePath); err != nil {
		return fmt.Errorf("signature file not found: %w", err)
	}

	// Verify certificate file exists
	if _, err := os.Stat(certPath); err != nil {
		return fmt.Errorf("certificate file not found: %w", err)
	}

	// Build cosign verify-blob command
	// Using keyless verification with provided signature and certificate
	cmd := exec.CommandContext(ctx, "cosign", "verify-blob",
		"--signature", signaturePath,
		"--certificate", certPath,
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		"--certificate-identity-regexp", "^https://github.com/.*/.*/.*@.*$",
		filePath,
	)

	// Capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cosign verification failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// VerifySignatureWithCertIdentity verifies with specific certificate identity
func (v *Verifier) VerifySignatureWithCertIdentity(ctx context.Context, filePath, signaturePath, certPath, certIdentity string) error {
	// Check if Cosign is installed
	if _, err := exec.LookPath("cosign"); err != nil {
		return fmt.Errorf("cosign not installed: %w", err)
	}

	// Build cosign verify-blob command with specific identity
	cmd := exec.CommandContext(ctx, "cosign", "verify-blob",
		"--signature", signaturePath,
		"--certificate", certPath,
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		"--certificate-identity", certIdentity,
		filePath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cosign verification failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// IsCosignInstalled checks if Cosign is available in PATH
func IsCosignInstalled() bool {
	_, err := exec.LookPath("cosign")
	return err == nil
}
