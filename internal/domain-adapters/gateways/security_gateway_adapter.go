// Package gateways provides implementations of domain gateway interfaces.
package gateways

import (
	"context"
	"fmt"

	"github.com/ochairo/potions/internal/domain/entities"
	"github.com/ochairo/potions/internal/domain/interfaces"
	"github.com/ochairo/potions/internal/external-adapters/attestation"
	"github.com/ochairo/potions/internal/external-adapters/cosign"
	"github.com/ochairo/potions/internal/external-adapters/gpg"
)

// SecurityGatewayAdapter implements the SecurityGateway interface
type SecurityGatewayAdapter struct {
	logger              interfaces.Logger
	gpgVerifier         *gpg.Verifier
	cosignVerifier      *cosign.Verifier
	attestationVerifier *attestation.Verifier
}

// NewSecurityGatewayAdapter creates a new security gateway adapter
func NewSecurityGatewayAdapter(logger interfaces.Logger) *SecurityGatewayAdapter {
	if logger == nil {
		logger = &interfaces.StdoutLogger{}
	}

	return &SecurityGatewayAdapter{
		logger:              logger,
		gpgVerifier:         gpg.NewVerifier(),
		cosignVerifier:      cosign.NewVerifier(),
		attestationVerifier: attestation.NewVerifier(),
	}
}

// VerifyChecksum verifies file checksum
func (s *SecurityGatewayAdapter) VerifyChecksum(ctx context.Context, filePath, expectedSum string) error {
	s.logger.Info("verifying checksum",
		interfaces.F("file", filePath),
		interfaces.F("expected", expectedSum[:16]+"..."),
	)

	// Implementation would use existing checksum verification logic
	// This would be moved from security_artifacts.go
	return fmt.Errorf("not implemented: use SecurityArtifactsService.VerifyChecksum")
}

// VerifyGPGSignature verifies GPG signature
func (s *SecurityGatewayAdapter) VerifyGPGSignature(ctx context.Context, filePath, sigURL string) error {
	s.logger.Info("verifying GPG signature",
		interfaces.F("file", filePath),
		interfaces.F("signature_url", sigURL),
	)

	return s.gpgVerifier.VerifySignature(ctx, filePath, sigURL)
}

// ImportGPGKeys imports GPG keys from keyservers
func (s *SecurityGatewayAdapter) ImportGPGKeys(ctx context.Context, keyIDs []string) error {
	s.logger.Info("importing GPG keys",
		interfaces.F("key_count", len(keyIDs)),
	)

	return s.gpgVerifier.ImportKeys(ctx, keyIDs)
}

// ImportGPGKeysFromURL imports GPG keys from a URL
func (s *SecurityGatewayAdapter) ImportGPGKeysFromURL(ctx context.Context, keysURL string) error {
	s.logger.Info("importing GPG keys from URL",
		interfaces.F("url", keysURL),
	)

	return s.gpgVerifier.ImportKeysFromURL(ctx, keysURL)
}

// VerifyCosignSignature verifies Cosign/Sigstore signature
func (s *SecurityGatewayAdapter) VerifyCosignSignature(ctx context.Context, filePath, signaturePath, certPath string) error {
	s.logger.Info("verifying Cosign signature",
		interfaces.F("file", filePath),
		interfaces.F("signature", signaturePath),
		interfaces.F("certificate", certPath),
	)

	if !cosign.IsCosignInstalled() {
		return fmt.Errorf("cosign not installed")
	}

	return s.cosignVerifier.VerifySignature(ctx, filePath, signaturePath, certPath)
}

// VerifyGitHubAttestation verifies GitHub attestation
func (s *SecurityGatewayAdapter) VerifyGitHubAttestation(ctx context.Context, filePath, attestationPath string) error {
	s.logger.Info("verifying GitHub attestation",
		interfaces.F("file", filePath),
		interfaces.F("attestation", attestationPath),
	)

	if !attestation.IsGHCLIInstalled() {
		return fmt.Errorf("gh CLI not installed")
	}

	return s.attestationVerifier.VerifyAttestation(ctx, filePath, attestationPath)
}

// VerifyInstalledPackage performs runtime verification of installed package
func (s *SecurityGatewayAdapter) VerifyInstalledPackage(ctx context.Context, packageName, installPath string) error {
	s.logger.Info("verifying installed package",
		interfaces.F("package", packageName),
		interfaces.F("path", installPath),
	)

	// Runtime verification checks:
	// 1. Package exists at install path
	// 2. Checksum matches (if available)
	// 3. Signature is valid (if available)
	// 4. Binary has expected permissions
	// 5. No tampering detected

	// This would integrate with existing verification logic
	return fmt.Errorf("not implemented: runtime package verification")
}

// ScanWithOSV scans artifact with OSV
func (s *SecurityGatewayAdapter) ScanWithOSV(ctx context.Context, artifact *entities.Artifact) (*entities.SecurityReport, error) {
	// Forward to existing implementation
	return nil, fmt.Errorf("not implemented: forward to existing OSV scanner")
}

// GenerateSBOM generates Software Bill of Materials
func (s *SecurityGatewayAdapter) GenerateSBOM(ctx context.Context, artifact *entities.Artifact) (*entities.SBOM, error) {
	// Forward to existing implementation
	return nil, fmt.Errorf("not implemented: forward to existing SBOM generator")
}

// AnalyzeBinaryHardening analyzes binary hardening features
func (s *SecurityGatewayAdapter) AnalyzeBinaryHardening(ctx context.Context, binaryPath, platform string) (*entities.BinaryAnalysis, error) {
	// Forward to existing implementation
	return nil, fmt.Errorf("not implemented: forward to existing binary analyzer")
}
