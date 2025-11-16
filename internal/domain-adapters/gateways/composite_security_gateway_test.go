package gateways

import (
	"context"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Test creating composite gateway with custom dependencies
func TestNewCompositeSecurityGatewayWithDeps(t *testing.T) {
	osv := NewOSVGateway()
	sbom := NewSBOMGenerator()
	analyzer := NewBinaryAnalyzerGateway()
	checksum := NewChecksumVerifier()
	gpg := NewGPGVerifier()

	gateway := NewCompositeSecurityGatewayWithDeps(osv, sbom, analyzer, checksum, gpg)

	if gateway == nil {
		t.Fatal("NewCompositeSecurityGatewayWithDeps returned nil")
	}

	composite, ok := gateway.(*compositeSecurityGateway)
	if !ok {
		t.Fatal("Gateway is not of type *compositeSecurityGateway")
	}

	// Verify dependencies are set
	if composite.osvGateway != osv {
		t.Error("osvGateway not set correctly")
	}

	if composite.sbomGenerator != sbom {
		t.Error("sbomGenerator not set correctly")
	}

	if composite.binaryAnalyzer != analyzer {
		t.Error("binaryAnalyzer not set correctly")
	}

	if composite.checksumVerifier != checksum {
		t.Error("checksumVerifier not set correctly")
	}

	if composite.gpgVerifier != gpg {
		t.Error("gpgVerifier not set correctly")
	}
}

// Test OSV scanning through composite gateway
func TestCompositeGateway_ScanWithOSV(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	artifact := &entities.Artifact{
		Name:    "kubectl",
		Version: "1.28.0",
	}

	// This will call the real OSV API, which might fail without network
	_, err := gateway.ScanWithOSV(context.Background(), artifact)

	// We don't assert on error since network might not be available
	// We just verify the method can be called
	if err != nil {
		t.Logf("ScanWithOSV returned error (expected without network): %v", err)
	}
}

// Test SBOM generation through composite gateway
func TestCompositeGateway_GenerateSBOM(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	artifact := &entities.Artifact{
		Name:    "kubectl",
		Version: "1.28.0",
		Path:    "/tmp/kubectl",
	}

	// This will fail because file doesn't exist, but tests the delegation
	_, err := gateway.GenerateSBOM(context.Background(), artifact)

	if err == nil {
		t.Log("GenerateSBOM succeeded (file might exist)")
	} else if !stringContainsSubstr(err.Error(), "failed to") {
		t.Logf("GenerateSBOM error: %v", err)
	}
}

// Test binary analysis through composite gateway
func TestCompositeGateway_AnalyzeBinaryHardening(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	// This will fail for nonexistent file
	_, err := gateway.AnalyzeBinaryHardening(context.Background(), "/nonexistent/binary", "linux-amd64")

	if err == nil {
		t.Fatal("Expected error for nonexistent binary, got nil")
	}
}

// Test checksum verification through composite gateway
func TestCompositeGateway_VerifyChecksum(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	// This will fail for nonexistent file
	err := gateway.VerifyChecksum(context.Background(), "/nonexistent/file", "abc123")

	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}
}

// Test GPG signature verification through composite gateway
func TestCompositeGateway_VerifyGPGSignature(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	// This will fail because no keys are imported
	err := gateway.VerifyGPGSignature(context.Background(), "/tmp/test", "http://example.com/test.sig")

	if err == nil {
		t.Fatal("Expected error when no GPG keys are imported, got nil")
	}

	if !stringContainsSubstr(err.Error(), "no GPG keys imported") {
		t.Errorf("Expected 'no GPG keys imported' error, got: %v", err)
	}
}

// Test GPG key import through composite gateway
func TestCompositeGateway_ImportGPGKeys(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	// This will fail with network error or key not found
	err := gateway.ImportGPGKeys(context.Background(), []string{"TESTKEY123"})

	if err == nil {
		t.Fatal("Expected error for invalid key, got nil")
	}
}

// Test empty key import through composite gateway
func TestCompositeGateway_ImportGPGKeys_EmptyKeys(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	err := gateway.ImportGPGKeys(context.Background(), []string{})

	if err == nil {
		t.Fatal("Expected error for empty key list, got nil")
	}

	if !stringContainsSubstr(err.Error(), "no key IDs provided") {
		t.Errorf("Expected 'no key IDs provided' error, got: %v", err)
	}
}

// Test all security features together (integration-style)
func TestCompositeGateway_IntegrationFlow(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	// Create test artifact
	artifact := &entities.Artifact{
		Name:     "test-package",
		Version:  "1.0.0",
		Platform: "linux-amd64",
		Path:     "/tmp/test",
	}

	// Test OSV scan
	t.Run("OSV Scan", func(t *testing.T) {
		_, err := gateway.ScanWithOSV(context.Background(), artifact)
		if err != nil {
			t.Logf("OSV scan error (expected): %v", err)
		}
	})

	// Test SBOM generation
	t.Run("SBOM Generation", func(t *testing.T) {
		_, err := gateway.GenerateSBOM(context.Background(), artifact)
		if err != nil {
			t.Logf("SBOM generation error (expected): %v", err)
		}
	})

	// Test binary analysis
	t.Run("Binary Analysis", func(t *testing.T) {
		_, err := gateway.AnalyzeBinaryHardening(context.Background(), "/tmp/test", "linux-amd64")
		if err != nil {
			t.Logf("Binary analysis error (expected): %v", err)
		}
	})

	// Test checksum verification
	t.Run("Checksum Verification", func(t *testing.T) {
		err := gateway.VerifyChecksum(context.Background(), "/tmp/test", "abc123")
		if err != nil {
			t.Logf("Checksum verification error (expected): %v", err)
		}
	})

	// Test GPG operations
	t.Run("GPG Operations", func(t *testing.T) {
		err := gateway.ImportGPGKeys(context.Background(), []string{})
		if err == nil {
			t.Error("Should fail with empty key list")
		}

		err = gateway.VerifyGPGSignature(context.Background(), "/tmp/test", "http://example.com/test.sig")
		if err == nil {
			t.Error("Should fail without imported keys")
		}
	})
}

// Test context cancellation propagation
func TestCompositeGateway_ContextCancellation(t *testing.T) {
	gateway := NewCompositeSecurityGateway()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	artifact := &entities.Artifact{
		Name:    "test",
		Version: "1.0.0",
	}

	// All operations should respect context cancellation
	_, err := gateway.ScanWithOSV(ctx, artifact)
	if err == nil {
		t.Log("ScanWithOSV: Context cancellation might not be checked early")
	}
}
