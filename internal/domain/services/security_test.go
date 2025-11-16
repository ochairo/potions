package services

import (
	"context"
	"testing"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

// mockSecurityGateway is a mock implementation for testing
type mockSecurityGateway struct {
	scanResult     *entities.SecurityReport
	scanError      error
	sbomResult     *entities.SBOM
	sbomError      error
	analysisResult *entities.BinaryAnalysis
	analysisError  error
}

func (m *mockSecurityGateway) ScanWithOSV(_ context.Context, _ *entities.Artifact) (*entities.SecurityReport, error) {
	return m.scanResult, m.scanError
}

func (m *mockSecurityGateway) GenerateSBOM(_ context.Context, _ *entities.Artifact) (*entities.SBOM, error) {
	return m.sbomResult, m.sbomError
}

func (m *mockSecurityGateway) AnalyzeBinaryHardening(_ context.Context, _, _ string) (*entities.BinaryAnalysis, error) {
	return m.analysisResult, m.analysisError
}

func (m *mockSecurityGateway) VerifyChecksum(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockSecurityGateway) VerifyGPGSignature(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockSecurityGateway) ImportGPGKeys(_ context.Context, _ []string) error {
	return nil
}

// TestCalculateSecurityScore tests the security score calculation logic
func TestCalculateSecurityScore(t *testing.T) {
	tests := []struct {
		name            string
		vulnerabilities []entities.Vulnerability
		expectedScore   float64
	}{
		{
			name:            "no vulnerabilities",
			vulnerabilities: []entities.Vulnerability{},
			expectedScore:   10.0,
		},
		{
			name: "one critical vulnerability",
			vulnerabilities: []entities.Vulnerability{
				{Severity: "CRITICAL"},
			},
			expectedScore: 7.0, // 10.0 - 3.0
		},
		{
			name: "one high vulnerability",
			vulnerabilities: []entities.Vulnerability{
				{Severity: "HIGH"},
			},
			expectedScore: 8.0, // 10.0 - 2.0
		},
		{
			name: "one medium vulnerability",
			vulnerabilities: []entities.Vulnerability{
				{Severity: "MEDIUM"},
			},
			expectedScore: 9.0, // 10.0 - 1.0
		},
		{
			name: "one low vulnerability",
			vulnerabilities: []entities.Vulnerability{
				{Severity: "LOW"},
			},
			expectedScore: 9.5, // 10.0 - 0.5
		},
		{
			name: "mixed severity vulnerabilities",
			vulnerabilities: []entities.Vulnerability{
				{Severity: "CRITICAL"},
				{Severity: "HIGH"},
				{Severity: "MEDIUM"},
				{Severity: "LOW"},
			},
			expectedScore: 3.5, // 10.0 - 3.0 - 2.0 - 1.0 - 0.5
		},
		{
			name: "score cannot go below zero",
			vulnerabilities: []entities.Vulnerability{
				{Severity: "CRITICAL"},
				{Severity: "CRITICAL"},
				{Severity: "CRITICAL"},
				{Severity: "CRITICAL"},
			},
			expectedScore: 0.0, // Would be -2.0, but clamped to 0
		},
		{
			name: "unknown severity",
			vulnerabilities: []entities.Vulnerability{
				{Severity: "UNKNOWN"},
			},
			expectedScore: 9.9, // 10.0 - 0.1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSecurityService(nil) // nil gateway OK for pure logic
			report := &entities.SecurityReport{
				Vulnerabilities: tt.vulnerabilities,
			}

			score := svc.CalculateSecurityScore(report)
			if score != tt.expectedScore {
				t.Errorf("CalculateSecurityScore() = %v, want %v", score, tt.expectedScore)
			}
		})
	}
}

// TestShouldBlockBuild tests the build blocking logic
func TestShouldBlockBuild(t *testing.T) {
	tests := []struct {
		name        string
		report      *entities.SecurityReport
		shouldBlock bool
	}{
		{
			name: "no vulnerabilities - allow build",
			report: &entities.SecurityReport{
				Vulnerabilities: []entities.Vulnerability{},
				Score:           10.0,
			},
			shouldBlock: false,
		},
		{
			name: "critical vulnerability - block build",
			report: &entities.SecurityReport{
				Vulnerabilities: []entities.Vulnerability{
					{Severity: "CRITICAL"},
				},
				Score: 7.0,
			},
			shouldBlock: true,
		},
		{
			name: "high vulnerabilities but no critical - allow",
			report: &entities.SecurityReport{
				Vulnerabilities: []entities.Vulnerability{
					{Severity: "HIGH"},
					{Severity: "HIGH"},
				},
				Score: 6.0,
			},
			shouldBlock: false,
		},
		{
			name: "low score but no critical - block",
			report: &entities.SecurityReport{
				Vulnerabilities: []entities.Vulnerability{
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
				},
				Score: 4.0, // Below threshold
			},
			shouldBlock: true,
		},
		{
			name: "score exactly at threshold - allow",
			report: &entities.SecurityReport{
				Vulnerabilities: []entities.Vulnerability{
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
					{Severity: "MEDIUM"},
				},
				Score: 5.0, // Exactly at threshold
			},
			shouldBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSecurityService(nil)
			blocked := svc.ShouldBlockBuild(tt.report)
			if blocked != tt.shouldBlock {
				t.Errorf("ShouldBlockBuild() = %v, want %v", blocked, tt.shouldBlock)
			}
		})
	}
}

// TestFilterVulnerabilities tests vulnerability filtering by severity
func TestFilterVulnerabilities(t *testing.T) {
	vulnerabilities := []entities.Vulnerability{
		{ID: "CVE-1", Severity: "CRITICAL"},
		{ID: "CVE-2", Severity: "HIGH"},
		{ID: "CVE-3", Severity: "MEDIUM"},
		{ID: "CVE-4", Severity: "LOW"},
		{ID: "CVE-5", Severity: "UNKNOWN"},
	}

	tests := []struct {
		name        string
		minSeverity string
		expected    []string // IDs
	}{
		{
			name:        "filter critical only",
			minSeverity: "CRITICAL",
			expected:    []string{"CVE-1"},
		},
		{
			name:        "filter high and above",
			minSeverity: "HIGH",
			expected:    []string{"CVE-1", "CVE-2"},
		},
		{
			name:        "filter medium and above",
			minSeverity: "MEDIUM",
			expected:    []string{"CVE-1", "CVE-2", "CVE-3"},
		},
		{
			name:        "filter low and above",
			minSeverity: "LOW",
			expected:    []string{"CVE-1", "CVE-2", "CVE-3", "CVE-4"},
		},
		{
			name:        "filter all including unknown",
			minSeverity: "UNKNOWN",
			expected:    []string{"CVE-1", "CVE-2", "CVE-3", "CVE-4", "CVE-5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewSecurityService(nil)
			filtered := svc.FilterVulnerabilities(vulnerabilities, tt.minSeverity)

			if len(filtered) != len(tt.expected) {
				t.Errorf("FilterVulnerabilities() returned %d vulnerabilities, want %d", len(filtered), len(tt.expected))
			}

			for i, vuln := range filtered {
				if vuln.ID != tt.expected[i] {
					t.Errorf("FilterVulnerabilities()[%d].ID = %v, want %v", i, vuln.ID, tt.expected[i])
				}
			}
		})
	}
}

// TestPerformSecurityScan tests the complete security scan workflow with mocks
func TestPerformSecurityScan(t *testing.T) {
	tests := []struct {
		name           string
		mockScanResult *entities.SecurityReport
		mockScanError  error
		wantError      bool
	}{
		{
			name: "successful scan with no vulnerabilities",
			mockScanResult: &entities.SecurityReport{
				Vulnerabilities: []entities.Vulnerability{},
				ScanDate:        time.Now().Format(time.RFC3339),
			},
			mockScanError: nil,
			wantError:     false,
		},
		{
			name: "successful scan with vulnerabilities",
			mockScanResult: &entities.SecurityReport{
				Vulnerabilities: []entities.Vulnerability{
					{ID: "CVE-2024-1", Severity: "HIGH"},
					{ID: "CVE-2024-2", Severity: "MEDIUM"},
				},
				ScanDate: time.Now().Format(time.RFC3339),
			},
			mockScanError: nil,
			wantError:     false,
		},
		{
			name:           "scan fails",
			mockScanResult: nil,
			mockScanError:  context.DeadlineExceeded,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGW := &mockSecurityGateway{
				scanResult: tt.mockScanResult,
				scanError:  tt.mockScanError,
			}
			svc := NewSecurityService(mockGW)

			artifact := &entities.Artifact{
				Name:    "test-package",
				Version: "1.0.0",
			}

			report, err := svc.PerformSecurityScan(context.Background(), artifact)

			if (err != nil) != tt.wantError {
				t.Errorf("PerformSecurityScan() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if report == nil {
					t.Error("PerformSecurityScan() returned nil report")
					return
				}

				// Verify score was calculated
				if report.Score == 0 && len(report.Vulnerabilities) == 0 {
					if report.Score != 10.0 {
						t.Errorf("PerformSecurityScan() score = %v, want 10.0 for no vulnerabilities", report.Score)
					}
				}
			}
		})
	}
}

// TestGenerateSBOM tests SBOM generation
func TestGenerateSBOM(t *testing.T) {
	mockGW := &mockSecurityGateway{
		sbomResult: &entities.SBOM{
			BOMFormat:   "CycloneDX",
			SpecVersion: "1.4",
			Components: []entities.Component{
				{Type: "application", Name: "test", Version: "1.0.0"},
			},
		},
		sbomError: nil,
	}
	svc := NewSecurityService(mockGW)

	artifact := &entities.Artifact{
		Name:    "test-package",
		Version: "1.0.0",
		Path:    "/tmp/test",
	}

	sbom, err := svc.GenerateSBOM(context.Background(), artifact)
	if err != nil {
		t.Errorf("GenerateSBOM() error = %v", err)
		return
	}

	if sbom == nil {
		t.Error("GenerateSBOM() returned nil SBOM")
		return
	}

	if sbom.BOMFormat != "CycloneDX" {
		t.Errorf("GenerateSBOM() BOMFormat = %v, want CycloneDX", sbom.BOMFormat)
	}
}

// TestAnalyzeBinary tests binary analysis
func TestAnalyzeBinary(t *testing.T) {
	mockGW := &mockSecurityGateway{
		analysisResult: &entities.BinaryAnalysis{
			Platform: "linux",
			HardeningFeatures: entities.HardeningFeatures{
				PIEEnabled:    true,
				StackCanaries: true,
				RELRO:         "full",
				NXBit:         true,
			},
			SecurityScore: entities.SecurityScore{
				Score:      8.0,
				Total:      10,
				Passed:     8,
				Percentage: 80,
			},
		},
		analysisError: nil,
	}
	svc := NewSecurityService(mockGW)

	analysis, err := svc.AnalyzeBinary(context.Background(), "/bin/test", "linux-amd64")
	if err != nil {
		t.Errorf("AnalyzeBinary() error = %v", err)
		return
	}

	if analysis == nil {
		t.Error("AnalyzeBinary() returned nil analysis")
		return
	}

	if analysis.Platform != "linux" {
		t.Errorf("AnalyzeBinary() Platform = %v, want linux", analysis.Platform)
	}

	if !analysis.HardeningFeatures.PIEEnabled {
		t.Error("AnalyzeBinary() PIEEnabled = false, want true")
	}
}

// TestGenerateAttestation tests attestation generation
func TestGenerateAttestation(t *testing.T) {
	svc := NewSecurityService(nil)

	artifact := &entities.Artifact{
		Name:     "test-package",
		Version:  "1.0.0",
		Platform: "linux-amd64",
	}

	analysis := &entities.BinaryAnalysis{
		Platform: "linux",
		HardeningFeatures: entities.HardeningFeatures{
			PIEEnabled: true,
		},
	}

	attestation, err := svc.GenerateAttestation(context.Background(), artifact, analysis)
	if err != nil {
		t.Errorf("GenerateAttestation() error = %v", err)
		return
	}

	if attestation == nil {
		t.Error("GenerateAttestation() returned nil attestation")
		return
	}

	if attestation.Version != "1.0" {
		t.Errorf("GenerateAttestation() Version = %v, want 1.0", attestation.Version)
	}

	if attestation.Predicate.HardeningFeatures == nil {
		t.Error("GenerateAttestation() HardeningFeatures not included in predicate")
	}
}
