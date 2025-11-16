package gateways

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Test creating a new OSV gateway
func TestNewOSVGateway(t *testing.T) {
	gateway := NewOSVGateway()

	if gateway == nil {
		t.Fatal("NewOSVGateway returned nil")
	}

	if gateway.apiURL != "https://api.osv.dev/v1/query" {
		t.Errorf("API URL = %s, want https://api.osv.dev/v1/query", gateway.apiURL)
	}
}

// Test scanning with vulnerabilities found
func TestOSVGateway_ScanWithOSV_VulnerabilitiesFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Method = %s, want POST", r.Method)
		}

		w.WriteHeader(http.StatusOK)
		response := OSVQueryResponse{
			Vulns: []OSVVulnerability{
				{
					ID:      "CVE-2023-1234",
					Summary: "Critical vulnerability in kubectl",
					Details: "Remote code execution possible",
					Severity: []OSVSeverity{
						{Type: "CVSS_V3", Score: "9.8"},
					},
				},
				{
					ID:      "CVE-2023-5678",
					Summary: "High severity issue",
					Details: "Denial of service",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	gateway := NewOSVGateway()
	gateway.apiURL = server.URL

	artifact := &entities.Artifact{
		Name:    "kubectl",
		Version: "1.28.0",
	}

	report, err := gateway.ScanWithOSV(context.Background(), artifact)

	if err != nil {
		t.Fatalf("ScanWithOSV failed: %v", err)
	}

	if len(report.Vulnerabilities) != 2 {
		t.Errorf("Expected 2 vulnerabilities, got: %d", len(report.Vulnerabilities))
	}

	if report.Vulnerabilities[0].ID != "CVE-2023-1234" {
		t.Errorf("First vulnerability ID = %s, want CVE-2023-1234", report.Vulnerabilities[0].ID)
	}

	if report.Metadata.Scanner != "OSV API" {
		t.Errorf("Scanner = %s, want OSV API", report.Metadata.Scanner)
	}
}

// Test scanning with no vulnerabilities
func TestOSVGateway_ScanWithOSV_NoVulnerabilities(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		response := OSVQueryResponse{
			Vulns: []OSVVulnerability{},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	gateway := NewOSVGateway()
	gateway.apiURL = server.URL

	artifact := &entities.Artifact{
		Name:    "safe-package",
		Version: "1.0.0",
	}

	report, err := gateway.ScanWithOSV(context.Background(), artifact)

	if err != nil {
		t.Fatalf("ScanWithOSV failed: %v", err)
	}

	if len(report.Vulnerabilities) != 0 {
		t.Errorf("Expected 0 vulnerabilities, got: %d", len(report.Vulnerabilities))
	}
}

// Test scanning with API error (404 = no vulnerabilities)
func TestOSVGateway_ScanWithOSV_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	gateway := NewOSVGateway()
	gateway.apiURL = server.URL

	artifact := &entities.Artifact{
		Name:    "unknown-package",
		Version: "1.0.0",
	}

	report, err := gateway.ScanWithOSV(context.Background(), artifact)

	if err != nil {
		t.Fatalf("ScanWithOSV failed: %v", err)
	}

	// 404 should return empty report, not error
	if len(report.Vulnerabilities) != 0 {
		t.Errorf("Expected 0 vulnerabilities for 404, got: %d", len(report.Vulnerabilities))
	}
}

// Test scanning with network error
func TestOSVGateway_ScanWithOSV_NetworkError(t *testing.T) {
	gateway := NewOSVGateway()
	gateway.apiURL = "http://invalid-url-that-does-not-exist.local:9999"

	artifact := &entities.Artifact{
		Name:    "test",
		Version: "1.0.0",
	}

	_, err := gateway.ScanWithOSV(context.Background(), artifact)

	if err == nil {
		t.Fatal("Expected error for network failure, got nil")
	}
}

// Test ecosystem detection
func TestOSVGateway_DetectEcosystem(t *testing.T) {
	gateway := NewOSVGateway()

	tests := []struct {
		name     string
		artifact *entities.Artifact
		want     string
	}{
		{
			name:     "kubectl",
			artifact: &entities.Artifact{Name: "kubectl"},
			want:     "Go",
		},
		{
			name:     "kubernetes",
			artifact: &entities.Artifact{Name: "kubernetes"},
			want:     "Go",
		},
		{
			name:     "node",
			artifact: &entities.Artifact{Name: "node"},
			want:     "npm",
		},
		{
			name:     "python",
			artifact: &entities.Artifact{Name: "python"},
			want:     "PyPI",
		},
		{
			name:     "unknown",
			artifact: &entities.Artifact{Name: "some-random-tool"},
			want:     "Generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ecosystem := gateway.detectEcosystem(tt.artifact)

			if ecosystem != tt.want {
				t.Errorf("detectEcosystem(%s) = %s, want %s", tt.artifact.Name, ecosystem, tt.want)
			}
		})
	}
}

// Test severity extraction
func TestOSVGateway_ExtractSeverity(t *testing.T) {
	gateway := NewOSVGateway()

	tests := []struct {
		name string
		vuln OSVVulnerability
		want string
	}{
		{
			name: "with severity field",
			vuln: OSVVulnerability{
				Severity: []OSVSeverity{{Type: "CRITICAL"}},
			},
			want: "CRITICAL",
		},
		{
			name: "no severity",
			vuln: OSVVulnerability{},
			want: "UNKNOWN",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			severity := gateway.extractSeverity(tt.vuln)

			if severity != tt.want {
				t.Errorf("extractSeverity() = %s, want %s", severity, tt.want)
			}
		})
	}
}

// Test scanning with invalid JSON response
func TestOSVGateway_ScanWithOSV_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	gateway := NewOSVGateway()
	gateway.apiURL = server.URL

	artifact := &entities.Artifact{
		Name:    "test",
		Version: "1.0.0",
	}

	_, err := gateway.ScanWithOSV(context.Background(), artifact)

	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

// Test context cancellation
func TestOSVGateway_ScanWithOSV_ContextCanceled(t *testing.T) {
	gateway := NewOSVGateway()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	artifact := &entities.Artifact{
		Name:    "test",
		Version: "1.0.0",
	}

	_, err := gateway.ScanWithOSV(ctx, artifact)

	if err == nil {
		t.Fatal("Expected error for canceled context, got nil")
	}
}
