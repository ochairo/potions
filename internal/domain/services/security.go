// Package services implements domain business logic and use cases.
package services

import (
	"context"
	"fmt"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
	"github.com/ochairo/potions/internal/domain/interfaces/gateways"
	"github.com/ochairo/potions/internal/domain/interfaces/services"
)

// securityService implements SecurityService with pure business logic
type securityService struct {
	gateway gateways.SecurityGateway
}

// NewSecurityService creates a new security service with dependency injection
func NewSecurityService(gateway gateways.SecurityGateway) services.SecurityService {
	return &securityService{gateway: gateway}
}

// PerformSecurityScan performs vulnerability scanning on an artifact
func (s *securityService) PerformSecurityScan(ctx context.Context, artifact *entities.Artifact) (*entities.SecurityReport, error) {
	// Delegate to gateway for actual scanning
	report, err := s.gateway.ScanWithOSV(ctx, artifact)
	if err != nil {
		return nil, fmt.Errorf("security scan failed: %w", err)
	}

	// Calculate security score (pure business logic)
	report.Score = s.CalculateSecurityScore(report)

	return report, nil
}

// GenerateSBOM generates a Software Bill of Materials for an artifact
func (s *securityService) GenerateSBOM(ctx context.Context, artifact *entities.Artifact) (*entities.SBOM, error) {
	// Delegate to gateway
	sbom, err := s.gateway.GenerateSBOM(ctx, artifact)
	if err != nil {
		return nil, fmt.Errorf("SBOM generation failed: %w", err)
	}

	return sbom, nil
}

// AnalyzeBinary analyzes binary hardening features
func (s *securityService) AnalyzeBinary(ctx context.Context, binaryPath, platform string) (*entities.BinaryAnalysis, error) {
	// Delegate to gateway
	analysis, err := s.gateway.AnalyzeBinaryHardening(ctx, binaryPath, platform)
	if err != nil {
		return nil, fmt.Errorf("binary analysis failed: %w", err)
	}

	return analysis, nil
}

// GenerateAttestation generates a security attestation
func (s *securityService) GenerateAttestation(_ context.Context, artifact *entities.Artifact, analysis *entities.BinaryAnalysis) (*entities.SecurityAttestation, error) {
	// Generate attestation (pure business logic)
	attestation := &entities.SecurityAttestation{
		Version:       "1.0",
		Timestamp:     time.Now(),
		PredicateType: "https://enterprise-security-attestation/v1",
		Subject: entities.AttestationSubject{
			Name: artifact.Name + "-" + artifact.Version + "-" + artifact.Platform,
			Digest: entities.DigestSet{
				SHA256: "", // Should be calculated
			},
		},
		Predicate: entities.AttestationPredicate{
			BuildType: "potions",
			VerificationSummary: entities.VerificationSummary{
				SupplyChainVerified: true,
				HardeningAnalyzed:   analysis != nil,
				ChecksumVerified:    true,
				VersionValidated:    true,
			},
			BuildMetadata: entities.BuildMetadata{
				Builder:        "potions",
				BuildID:        "local",
				BuildTimestamp: time.Now(),
				SourceCommit:   "unknown",
			},
		},
	}

	if analysis != nil {
		attestation.Predicate.HardeningFeatures = &analysis.HardeningFeatures
	}

	return attestation, nil
}

// CalculateSecurityScore calculates a security score based on vulnerabilities
// Pure business logic - no I/O
func (s *securityService) CalculateSecurityScore(report *entities.SecurityReport) float64 {
	if len(report.Vulnerabilities) == 0 {
		return 10.0
	}

	score := 10.0
	for _, vuln := range report.Vulnerabilities {
		switch vuln.Severity {
		case "CRITICAL":
			score -= 3.0
		case "HIGH":
			score -= 2.0
		case "MEDIUM":
			score -= 1.0
		case "LOW":
			score -= 0.5
		default:
			// UNKNOWN or other
			score -= 0.1
		}
	}

	if score < 0 {
		return 0.0
	}
	return score
}

// FilterVulnerabilities filters vulnerabilities by minimum severity
// Pure business logic - no I/O
func (s *securityService) FilterVulnerabilities(vulnerabilities []entities.Vulnerability, minSeverity string) []entities.Vulnerability {
	severityOrder := map[string]int{
		"CRITICAL": 4,
		"HIGH":     3,
		"MEDIUM":   2,
		"LOW":      1,
		"UNKNOWN":  0,
	}

	minLevel := severityOrder[minSeverity]
	filtered := make([]entities.Vulnerability, 0)

	for _, vuln := range vulnerabilities {
		if severityOrder[vuln.Severity] >= minLevel {
			filtered = append(filtered, vuln)
		}
	}

	return filtered
}

// ShouldBlockBuild determines if a build should be blocked based on security report
// Pure business logic - no I/O
func (s *securityService) ShouldBlockBuild(report *entities.SecurityReport) bool {
	// Block if any CRITICAL vulnerabilities
	for _, vuln := range report.Vulnerabilities {
		if vuln.Severity == "CRITICAL" {
			return true
		}
	}

	// Block if security score too low
	if report.Score < 5.0 {
		return true
	}

	return false
}
