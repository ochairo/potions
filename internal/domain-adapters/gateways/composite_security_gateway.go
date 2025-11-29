package gateways

import (
	"context"

	"github.com/ochairo/potions/internal/domain/entities"
	"github.com/ochairo/potions/internal/domain/interfaces/gateways"
)

// compositeSecurityGateway implements the SecurityGateway interface by composing
// all individual security gateways together
type compositeSecurityGateway struct {
	osvGateway       *osvGateway
	sbomGenerator    *sbomGenerator
	binaryAnalyzer   *binaryAnalyzerGateway
	checksumVerifier *checksumVerifier
	gpgVerifier      *gpgVerifier
}

// NewCompositeSecurityGateway creates a new composite security gateway with all dependencies
func NewCompositeSecurityGateway() gateways.SecurityGateway {
	return &compositeSecurityGateway{
		osvGateway:       NewOSVGateway(),
		sbomGenerator:    NewSBOMGenerator(),
		binaryAnalyzer:   NewBinaryAnalyzerGateway(),
		checksumVerifier: NewChecksumVerifier(),
		gpgVerifier:      NewGPGVerifier(),
	}
}

// NewCompositeSecurityGatewayWithDeps creates a composite gateway with custom dependencies
// This is useful for testing or when you want to inject specific implementations
func NewCompositeSecurityGatewayWithDeps(
	osv *osvGateway,
	sbom *sbomGenerator,
	analyzer *binaryAnalyzerGateway,
	checksum *checksumVerifier,
	gpg *gpgVerifier,
) gateways.SecurityGateway {
	return &compositeSecurityGateway{
		osvGateway:       osv,
		sbomGenerator:    sbom,
		binaryAnalyzer:   analyzer,
		checksumVerifier: checksum,
		gpgVerifier:      gpg,
	}
}

// ScanWithOSV performs vulnerability scanning using OSV API
func (c *compositeSecurityGateway) ScanWithOSV(ctx context.Context, artifact *entities.Artifact) (*entities.SecurityReport, error) {
	return c.osvGateway.ScanWithOSV(ctx, artifact)
}

// GenerateSBOM generates a Software Bill of Materials
func (c *compositeSecurityGateway) GenerateSBOM(ctx context.Context, artifact *entities.Artifact) (*entities.SBOM, error) {
	return c.sbomGenerator.GenerateSBOM(ctx, artifact)
}

// AnalyzeBinaryHardening analyzes binary security hardening features
func (c *compositeSecurityGateway) AnalyzeBinaryHardening(ctx context.Context, binaryPath, platform string) (*entities.BinaryAnalysis, error) {
	return c.binaryAnalyzer.AnalyzeBinaryHardening(ctx, binaryPath, platform)
}

// VerifyChecksum verifies a file's SHA256 checksum
func (c *compositeSecurityGateway) VerifyChecksum(ctx context.Context, filePath, expectedSum string) error {
	return c.checksumVerifier.VerifyChecksum(ctx, filePath, expectedSum)
}

// VerifyGPGSignature verifies a detached GPG signature
func (c *compositeSecurityGateway) VerifyGPGSignature(ctx context.Context, filePath, sigURL string) error {
	return c.gpgVerifier.VerifyGPGSignature(ctx, filePath, sigURL)
}

// ImportGPGKeys imports GPG keys from keyservers
func (c *compositeSecurityGateway) ImportGPGKeys(ctx context.Context, keyIDs []string) error {
	return c.gpgVerifier.ImportGPGKeys(ctx, keyIDs)
}

func (c *compositeSecurityGateway) ImportGPGKeysFromURL(ctx context.Context, keysURL string) error {
	return c.gpgVerifier.ImportGPGKeysFromURL(ctx, keysURL)
}

// VerifyCosignSignature verifies Cosign/Sigstore signature (not yet fully implemented)
func (c *compositeSecurityGateway) VerifyCosignSignature(_ context.Context, _, _, _ string) error {
	// TODO: Implement Cosign verification when needed
	// For now, this would be handled by external tools (cosign CLI)
	return nil
}

// VerifyGitHubAttestation verifies GitHub attestation (not yet fully implemented)
func (c *compositeSecurityGateway) VerifyGitHubAttestation(_ context.Context, _, _ string) error {
	// TODO: Implement GitHub attestation verification when needed
	// For now, this would be handled by external tools (gh CLI)
	return nil
}

// VerifyInstalledPackage performs runtime verification of installed package (not yet fully implemented)
func (c *compositeSecurityGateway) VerifyInstalledPackage(_ context.Context, _, _ string) error {
	// TODO: Implement runtime package verification when needed
	// This would check checksums, signatures, and permissions
	return nil
}
