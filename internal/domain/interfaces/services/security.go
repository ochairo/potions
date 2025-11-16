// Package services defines interfaces for domain service contracts.
package services

import (
	"context"

	"github.com/ochairo/potions/internal/domain/entities"
)

// SecurityService defines the interface for high-level security operations
// Contains business logic for security decisions
type SecurityService interface {
	// High-level security operations
	PerformSecurityScan(ctx context.Context, artifact *entities.Artifact) (*entities.SecurityReport, error)
	GenerateSBOM(ctx context.Context, artifact *entities.Artifact) (*entities.SBOM, error)
	AnalyzeBinary(ctx context.Context, binaryPath, platform string) (*entities.BinaryAnalysis, error)
	GenerateAttestation(ctx context.Context, artifact *entities.Artifact, analysis *entities.BinaryAnalysis) (*entities.SecurityAttestation, error)

	// Business logic
	CalculateSecurityScore(report *entities.SecurityReport) float64
	FilterVulnerabilities(vulnerabilities []entities.Vulnerability, minSeverity string) []entities.Vulnerability
	ShouldBlockBuild(report *entities.SecurityReport) bool
}
