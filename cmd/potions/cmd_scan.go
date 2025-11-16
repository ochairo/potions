package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ochairo/potions/internal/domain-adapters/gateways"
	orchestrators "github.com/ochairo/potions/internal/domain-orchestrators"
	"github.com/ochairo/potions/internal/domain/entities"
	"github.com/ochairo/potions/internal/domain/services"
)

func runScan(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	var (
		packageName = fs.String("package", "", "Package name to scan")
		version     = fs.String("version", "", "Package version to scan")
		platform    = fs.String("platform", "", "Platform (e.g., linux-amd64, darwin-arm64)")
		binaryPath  = fs.String("binary", "", "Direct path to binary file to scan")
		verbose     = fs.Bool("verbose", false, "Show detailed scan results")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: potions scan [options]

Run complete security scan on a package or binary.

Performs:
  - Vulnerability scanning (OSV API)
  - Binary hardening analysis
  - SBOM generation
  - Security attestation

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  potions scan --package kubectl --version 1.28.0 --platform linux-amd64
  potions scan --binary /path/to/kubectl
  potions scan --package kubectl --version 1.28.0 --platform linux-amd64 --verbose
`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Validate inputs
	if *packageName == "" && *binaryPath == "" {
		fmt.Fprintf(os.Stderr, "Error: either --package or --binary is required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	if *packageName != "" && (*version == "" || *platform == "") {
		fmt.Fprintf(os.Stderr, "Error: --version and --platform are required when using --package\n\n")
		fs.Usage()
		os.Exit(1)
	}

	// Execute scan following Clean Architecture
	if err := executeScan(ctx, *packageName, *version, *platform, *binaryPath, *verbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func executeScan(ctx context.Context, packageName, version, platform, binaryPath string, verbose bool) error {
	// Layer 1: Create composite gateway (Infrastructure) - handles all gateway creation internally
	securityGateway := gateways.NewCompositeSecurityGateway()

	// Layer 2: Create service (Business Logic)
	securityService := services.NewSecurityService(securityGateway)

	// Layer 3: Create orchestrator (Use Case)
	securityOrch := orchestrators.NewSecurityOrchestrator(securityService)

	// Create artifact entity
	var artifact *entities.Artifact
	if binaryPath != "" {
		// Scan direct binary
		_, err := os.Stat(binaryPath)
		if err != nil {
			return fmt.Errorf("failed to access binary: %w", err)
		}

		artifact = &entities.Artifact{
			Name:     filepath.Base(binaryPath),
			Version:  "unknown",
			Platform: detectPlatform(),
			Path:     binaryPath,
			Type:     "binary",
		}
	} else {
		// Scan package (would need to download first)
		artifact = &entities.Artifact{
			Name:     packageName,
			Version:  version,
			Platform: platform,
			Type:     "package",
		}
	}

	fmt.Printf("🔍 Security Scan: %s@%s (%s)\n\n", artifact.Name, artifact.Version, artifact.Platform)

	// Execute security workflow through orchestrator
	result, err := securityOrch.PerformSecurityWorkflow(ctx, artifact)
	if err != nil {
		return fmt.Errorf("security workflow failed: %w", err)
	}

	// Display results
	displayScanResults(result, verbose)

	// Exit with error if blocked
	if result.Blocked {
		return fmt.Errorf("security scan failed: build blocked")
	}

	return nil
}

func displayScanResults(result *orchestrators.SecurityWorkflowResult, verbose bool) {
	// Security Report
	if result.SecurityReport != nil {
		report := result.SecurityReport
		fmt.Printf("📊 Vulnerability Scan\n")
		fmt.Printf("   Scanner: %s %s\n", report.Metadata.Scanner, report.Metadata.ScannerVersion)
		fmt.Printf("   Total vulnerabilities: %d\n", len(report.Vulnerabilities))
		fmt.Printf("   Security score: %.1f/10.0\n", report.Score)

		if len(report.Vulnerabilities) > 0 {
			// Count by severity
			critical, high, medium, low := 0, 0, 0, 0
			for _, vuln := range report.Vulnerabilities {
				switch vuln.Severity {
				case "CRITICAL":
					critical++
				case "HIGH":
					high++
				case "MEDIUM":
					medium++
				case "LOW":
					low++
				}
			}

			if critical > 0 {
				fmt.Printf("   🔴 CRITICAL: %d\n", critical)
			}
			if high > 0 {
				fmt.Printf("   🟠 HIGH: %d\n", high)
			}
			if medium > 0 {
				fmt.Printf("   🟡 MEDIUM: %d\n", medium)
			}
			if low > 0 {
				fmt.Printf("   🟢 LOW: %d\n", low)
			}

			if verbose {
				fmt.Printf("\n   Vulnerabilities:\n")
				for i, vuln := range report.Vulnerabilities {
					if i >= 10 && !verbose {
						fmt.Printf("   ... and %d more\n", len(report.Vulnerabilities)-10)
						break
					}
					fmt.Printf("   - %s [%s] %s\n", vuln.ID, vuln.Severity, vuln.Description)
				}
			}
		} else {
			fmt.Printf("   ✅ No vulnerabilities found\n")
		}
		fmt.Printf("\n")
	}

	// Binary Analysis
	if result.BinaryAnalysis != nil {
		analysis := result.BinaryAnalysis
		fmt.Printf("🛡️  Binary Hardening Analysis\n")
		fmt.Printf("   Platform: %s\n", analysis.Platform)
		fmt.Printf("   Security score: %.1f/10.0 (%d/%d checks passed)\n",
			analysis.SecurityScore.Score,
			analysis.SecurityScore.Passed,
			analysis.SecurityScore.Total)

		features := analysis.HardeningFeatures
		fmt.Printf("   PIE: %s\n", formatCheck(features.PIEEnabled))
		fmt.Printf("   Stack Canaries: %s\n", formatCheck(features.StackCanaries))
		fmt.Printf("   NX Bit: %s\n", formatCheck(features.NXBit))
		if features.RELRO != "" {
			fmt.Printf("   RELRO: %s\n", features.RELRO)
		}
		if features.FortifySource {
			fmt.Printf("   FORTIFY_SOURCE: ✅\n")
		}
		if features.CodeSigned {
			fmt.Printf("   Code Signed: ✅\n")
		}
		if features.HardenedRuntime {
			fmt.Printf("   Hardened Runtime: ✅\n")
		}
		fmt.Printf("\n")
	}

	// SBOM
	if result.SBOM != nil {
		fmt.Printf("📋 SBOM (Software Bill of Materials)\n")
		fmt.Printf("   Format: %s %s\n", result.SBOM.BOMFormat, result.SBOM.SpecVersion)
		fmt.Printf("   Components: %d\n", len(result.SBOM.Components))
		fmt.Printf("\n")
	}

	// Attestation
	if result.Attestation != nil {
		fmt.Printf("📝 Security Attestation\n")
		fmt.Printf("   Generated: %s\n", result.Attestation.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("\n")
	}

	// Summary
	fmt.Printf("⏱️  Workflow Duration: %v\n\n", result.WorkflowDuration)

	if result.Blocked {
		fmt.Printf("🚫 SCAN RESULT: BLOCKED\n")
		fmt.Printf("   Reason: %s\n", result.BlockReason)
	} else {
		fmt.Printf("✅ SCAN RESULT: PASSED\n")
	}
}

func formatCheck(enabled bool) string {
	if enabled {
		return "✅ Enabled"
	}
	return "❌ Disabled"
}
