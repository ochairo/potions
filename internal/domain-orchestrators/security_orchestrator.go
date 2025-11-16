package orchestrators

import (
	"context"
	"fmt"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
	"github.com/ochairo/potions/internal/domain/interfaces/services"
)

// SecurityOrchestrator coordinates the complete security workflow
// Following Clean Architecture: orchestrators coordinate services for complex use cases
type SecurityOrchestrator struct {
	securityService services.SecurityService
}

// NewSecurityOrchestrator creates a new security orchestrator
func NewSecurityOrchestrator(securityService services.SecurityService) *SecurityOrchestrator {
	return &SecurityOrchestrator{
		securityService: securityService,
	}
}

// SecurityWorkflowResult contains the complete security analysis results
type SecurityWorkflowResult struct {
	Artifact         *entities.Artifact
	SecurityReport   *entities.SecurityReport
	BinaryAnalysis   *entities.BinaryAnalysis
	SBOM             *entities.SBOM
	Attestation      *entities.SecurityAttestation
	WorkflowDuration time.Duration
	Blocked          bool
	BlockReason      string
}

// PerformSecurityWorkflow executes the complete security workflow for an artifact
// This is the main use case that coordinates all security operations
func (o *SecurityOrchestrator) PerformSecurityWorkflow(ctx context.Context, artifact *entities.Artifact) (*SecurityWorkflowResult, error) {
	startTime := time.Now()

	result := &SecurityWorkflowResult{
		Artifact: artifact,
	}

	// Step 1: Vulnerability scanning
	securityReport, err := o.securityService.PerformSecurityScan(ctx, artifact)
	if err != nil {
		return nil, fmt.Errorf("vulnerability scan failed: %w", err)
	}
	result.SecurityReport = securityReport

	// Step 2: Check if build should be blocked
	if o.securityService.ShouldBlockBuild(securityReport) {
		result.Blocked = true
		result.BlockReason = o.determineBlockReason(securityReport)
		result.WorkflowDuration = time.Since(startTime)
		return result, nil
	}

	// Step 3: Binary analysis (if artifact is a binary)
	if artifact.Type == "binary" && artifact.Path != "" {
		binaryAnalysis, err := o.securityService.AnalyzeBinary(ctx, artifact.Path, artifact.Platform)
		if err != nil {
			// Log warning but don't fail - binary analysis is best-effort
			// In production, use proper logger
			_ = err
		} else {
			result.BinaryAnalysis = binaryAnalysis
		}
	}

	// Step 4: Generate SBOM
	sbom, err := o.securityService.GenerateSBOM(ctx, artifact)
	if err != nil {
		// Log warning but don't fail - SBOM is nice-to-have
		_ = err
	} else {
		result.SBOM = sbom
	}

	// Step 5: Generate security attestation
	attestation, err := o.securityService.GenerateAttestation(ctx, artifact, result.BinaryAnalysis)
	if err != nil {
		// Log warning but don't fail - attestation is nice-to-have
		_ = err
	} else {
		result.Attestation = attestation
	}

	result.WorkflowDuration = time.Since(startTime)
	return result, nil
}

// determineBlockReason analyzes the security report to determine why the build was blocked
func (o *SecurityOrchestrator) determineBlockReason(report *entities.SecurityReport) string {
	// Check for critical vulnerabilities
	criticalCount := 0
	for _, vuln := range report.Vulnerabilities {
		if vuln.Severity == "CRITICAL" {
			criticalCount++
		}
	}

	if criticalCount > 0 {
		return fmt.Sprintf("Build blocked: %d CRITICAL vulnerabilities found", criticalCount)
	}

	// Check for low security score
	if report.Score < 5.0 {
		return fmt.Sprintf("Build blocked: Security score %.1f/10.0 below threshold (5.0)", report.Score)
	}

	return "Build blocked: Security requirements not met"
}

// GetHighSeverityVulnerabilities returns vulnerabilities of HIGH or CRITICAL severity
func (o *SecurityOrchestrator) GetHighSeverityVulnerabilities(report *entities.SecurityReport) []entities.Vulnerability {
	return o.securityService.FilterVulnerabilities(report.Vulnerabilities, "HIGH")
}

// GetSecuritySummary generates a human-readable security summary
func (o *SecurityOrchestrator) GetSecuritySummary(result *SecurityWorkflowResult) string {
	if result.Blocked {
		return fmt.Sprintf("🚫 BLOCKED: %s", result.BlockReason)
	}

	summary := fmt.Sprintf("✅ PASSED: Security score %.1f/10.0\n", result.SecurityReport.Score)
	summary += fmt.Sprintf("   Vulnerabilities: %d total\n", len(result.SecurityReport.Vulnerabilities))

	if result.BinaryAnalysis != nil {
		summary += fmt.Sprintf("   Binary hardening: %d/%d checks passed\n",
			result.BinaryAnalysis.SecurityScore.Passed,
			result.BinaryAnalysis.SecurityScore.Total)
	}

	if result.SBOM != nil {
		summary += fmt.Sprintf("   SBOM: %d components\n", len(result.SBOM.Components))
	}

	summary += fmt.Sprintf("   Duration: %v", result.WorkflowDuration)

	return summary
}
