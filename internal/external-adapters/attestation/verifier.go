// Package attestation provides GitHub Attestation verification capabilities.
package attestation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Verifier implements GitHub Attestation verification
type Verifier struct{}

// NewVerifier creates a new attestation verifier
func NewVerifier() *Verifier {
	return &Verifier{}
}

// VerifyAttestation verifies a GitHub Attestation
func (v *Verifier) VerifyAttestation(ctx context.Context, filePath, attestationPath string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not installed: %w (install from https://cli.github.com)", err)
	}

	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	if _, err := os.Stat(attestationPath); err != nil {
		return fmt.Errorf("attestation file not found: %w", err)
	}

	attestationData, err := os.ReadFile(attestationPath)
	if err != nil {
		return fmt.Errorf("failed to read attestation: %w", err)
	}

	var attestation map[string]interface{}
	if err := json.Unmarshal(attestationData, &attestation); err != nil {
		return fmt.Errorf("invalid attestation format: %w", err)
	}

	if _, ok := attestation["_type"]; !ok {
		return fmt.Errorf("attestation missing _type field")
	}

	if _, ok := attestation["subject"]; !ok {
		return fmt.Errorf("attestation missing subject field")
	}

	return nil
}

// VerifyAttestationWithGH verifies using gh CLI
func (v *Verifier) VerifyAttestationWithGH(ctx context.Context, filePath, owner, repo string) error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not installed: %w", err)
	}

	cmd := exec.CommandContext(ctx, "gh", "attestation", "verify",
		filePath,
		"--owner", owner,
		"--repo", repo,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("attestation verification failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// IsGHCLIInstalled checks if gh CLI is available in PATH
func IsGHCLIInstalled() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}
