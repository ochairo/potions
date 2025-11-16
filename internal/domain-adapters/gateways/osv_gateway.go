package gateways

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
)

// osvGateway implements OSV vulnerability scanning using pure Go HTTP API
// No osv-scanner binary required
type osvGateway struct {
	apiURL     string
	httpClient *http.Client
}

// NewOSVGateway creates a new OSV gateway
//
//nolint:revive // unexported-return: Intentionally returns concrete type for testability
func NewOSVGateway() *osvGateway {
	return &osvGateway{
		apiURL: "https://api.osv.dev/v1/query",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ScanWithOSV scans an artifact for vulnerabilities using OSV API
func (g *osvGateway) ScanWithOSV(ctx context.Context, artifact *entities.Artifact) (*entities.SecurityReport, error) {
	// Detect ecosystem from artifact
	ecosystem := g.detectEcosystem(artifact)

	// Query OSV API directly (no osv-scanner binary needed)
	payload := OSVQueryRequest{
		Package: OSVPackage{
			Name:      artifact.Name,
			Ecosystem: ecosystem,
		},
		Version: artifact.Version,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", g.apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSV API request failed: %w", err)
	}
	//nolint:errcheck // Defer close on HTTP response body
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &entities.SecurityReport{
			Vulnerabilities: []entities.Vulnerability{},
			ScanDate:        time.Now().Format(time.RFC3339),
			Metadata: entities.ScanMetadata{
				Scanner:        "OSV API",
				ScannerVersion: "v1",
			},
		}, nil
	}

	var osvResp OSVQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&osvResp); err != nil {
		return nil, fmt.Errorf("failed to parse OSV response: %w", err)
	}

	// Convert to domain entities
	vulnerabilities := make([]entities.Vulnerability, 0)
	for _, vuln := range osvResp.Vulns {
		vulnerabilities = append(vulnerabilities, entities.Vulnerability{
			ID:          vuln.ID,
			Severity:    g.extractSeverity(vuln),
			Description: vuln.Summary,
			Score:       g.extractCVSS(vuln),
			Component:   artifact.Name + "@" + artifact.Version,
		})
	}

	return &entities.SecurityReport{
		Vulnerabilities: vulnerabilities,
		ScanDate:        time.Now().Format(time.RFC3339),
		Metadata: entities.ScanMetadata{
			Scanner:        "OSV API",
			ScannerVersion: "v1",
		},
	}, nil
}

// detectEcosystem tries to detect the package ecosystem
func (g *osvGateway) detectEcosystem(artifact *entities.Artifact) string {
	// Simple heuristics - could be improved
	name := artifact.Name

	switch {
	case contains(name, []string{"kubectl", "kube", "kubernetes"}):
		return "Go"
	case contains(name, []string{"node", "npm", "yarn"}):
		return "npm"
	case contains(name, []string{"python", "pip"}):
		return "PyPI"
	default:
		return "Generic"
	}
}

// extractSeverity extracts severity from OSV vulnerability
func (g *osvGateway) extractSeverity(vuln OSVVulnerability) string {
	// OSV doesn't always provide severity, try to extract from various fields
	if len(vuln.Severity) > 0 {
		return vuln.Severity[0].Type
	}

	// Try to infer from CVSS score if available
	score := g.extractCVSS(vuln)
	switch {
	case score >= 9.0:
		return "CRITICAL"
	case score >= 7.0:
		return "HIGH"
	case score >= 4.0:
		return "MEDIUM"
	case score > 0:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// extractCVSS extracts CVSS score from OSV vulnerability
//
//nolint:unparam // Placeholder implementation - will parse CVSS from vuln.Severity in future
func (g *osvGateway) extractCVSS(vuln OSVVulnerability) float64 {
	if len(vuln.Severity) > 0 {
		// Try to parse CVSS score from severity
		// OSV stores severity info, but for now return placeholder
		_ = vuln.Severity // Use parameter
		return 0.0
	}
	return 0.0
}

// Helper function
func contains(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// OSV API request/response types

// OSVQueryRequest represents a query to the OSV API for vulnerability information.
type OSVQueryRequest struct {
	Package OSVPackage `json:"package"`
	Version string     `json:"version"`
}

// OSVPackage identifies a software package in a specific ecosystem.
type OSVPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem"`
}

// OSVQueryResponse contains the vulnerability results from the OSV API.
type OSVQueryResponse struct {
	Vulns []OSVVulnerability `json:"vulns"`
}

// OSVVulnerability represents a single vulnerability from the OSV database.
type OSVVulnerability struct {
	ID       string        `json:"id"`
	Summary  string        `json:"summary"`
	Details  string        `json:"details"`
	Severity []OSVSeverity `json:"severity,omitempty"`
}

// OSVSeverity contains severity scoring information for a vulnerability.
type OSVSeverity struct {
	Type  string `json:"type"`
	Score string `json:"score"`
}
