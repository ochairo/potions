package gateways

import (
	"context"

	"github.com/ochairo/potions/internal/domain/entities"
)

// SecurityGateway defines the interface for security operations
// Implementations should use pure Go (zero external dependencies)
type SecurityGateway interface {
	// Vulnerability scanning
	ScanWithOSV(ctx context.Context, artifact *entities.Artifact) (*entities.SecurityReport, error)

	// SBOM generation
	GenerateSBOM(ctx context.Context, artifact *entities.Artifact) (*entities.SBOM, error)

	// Binary analysis
	AnalyzeBinaryHardening(ctx context.Context, binaryPath, platform string) (*entities.BinaryAnalysis, error)

	// Verification
	VerifyChecksum(ctx context.Context, filePath, expectedSum string) error
	VerifyGPGSignature(ctx context.Context, filePath, sigURL string) error
	ImportGPGKeys(ctx context.Context, keyIDs []string) error
	ImportGPGKeysFromURL(ctx context.Context, keysURL string) error

	// Cosign/Sigstore verification
	VerifyCosignSignature(ctx context.Context, filePath, signaturePath, certPath string) error

	// GitHub Attestation verification
	VerifyGitHubAttestation(ctx context.Context, filePath, attestationPath string) error

	// Runtime verification (post-install checks)
	VerifyInstalledPackage(ctx context.Context, packageName, installPath string) error
}
