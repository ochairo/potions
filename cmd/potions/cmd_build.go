// Package main provides the potions CLI for building prebuilt software binaries.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain-adapters/gateways"
	orchestrators "github.com/ochairo/potions/internal/domain-orchestrators"
	"github.com/ochairo/potions/internal/domain/entities"
	"github.com/ochairo/potions/internal/domain/services"
	"github.com/ochairo/potions/internal/external-adapters/yaml"
)

// PackageBuildInput represents a package to build
type PackageBuildInput struct {
	Package string `json:"package"`
	Version string `json:"version"`
}

// BuildReport represents the output of building packages
type BuildReport struct {
	TotalPackages     int            `json:"total_packages"`
	SuccessfulBuilds  int            `json:"successful_builds"`
	FailedBuilds      int            `json:"failed_builds"`
	TimeoutBuilds     int            `json:"timeout_builds"`
	SuccessDetails    []BuildResult  `json:"success_details"`
	FailureDetails    []BuildResult  `json:"failure_details"`
	TimeoutDetails    []BuildResult  `json:"timeout_details"`
	PlatformBreakdown map[string]int `json:"platform_breakdown"`
	DurationSeconds   float64        `json:"duration_seconds"`
}

// BuildResult represents the outcome of a single build
type BuildResult struct {
	Package  string `json:"package"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

func runBuild(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	var (
		// Common flags
		platform       = fs.String("platform", "", "Target platform (e.g., darwin-arm64)")
		enableSecurity = fs.Bool("enable-security-scan", true, "Enable security vulnerability scanning (default: true)")
		recipesDir     = fs.String("recipes-dir", "recipes", "Path to recipes directory")
		outputDir      = fs.String("output-dir", "dist", "Output directory for built binaries")

		// Single package flags
		allPlatforms = fs.Bool("all-platforms", false, "Build for all platforms defined in recipe")

		// Multiple packages flags
		packages       = fs.String("packages", "", "JSON array of packages to build")
		timeoutMinutes = fs.Int("timeout", 20, "Timeout per package build in minutes")
		successFile    = fs.String("successes", "build-successes.txt", "File to write successful builds")
		failureFile    = fs.String("failures", "build-failures.txt", "File to write failed builds")
		timeoutFile    = fs.String("timeouts", "build-failures-timeout.txt", "File to write timeout builds")
		errorFile      = fs.String("errors", "build-failures-error.txt", "File to write error builds")
		jsonOutput     = fs.String("json-output", "", "Optional JSON file for detailed report")
		quiet          = fs.Bool("quiet", false, "Quiet mode - minimal output")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: potions build <package> [version] [options]
       potions build --packages <json> --platform <platform> [options]

Build binaries for packages.

Examples:
  # Single package
  potions build kubectl                                # Build latest version, auto-detect platform
  potions build kubectl v1.28.0                        # Build specific version
  potions build kubectl v1.28.0 --platform darwin-arm64
  potions build kubectl v1.28.0 --all-platforms        # Build for all platforms

  # Multiple packages from JSON
  potions build --packages '[{"package":"curl","version":"8.11.1"}]' --platform linux-x86_64
  potions build --packages @packages.json --platform darwin-arm64
  potions build --packages "$PACKAGES" --platform linux-arm64 --quiet

Options:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Build multiple packages from JSON input
	if *packages != "" {
		if *platform == "" {
			fmt.Fprintf(os.Stderr, "Error: --platform is required when using --packages\n")
			fs.Usage()
			os.Exit(1)
		}
		buildFromPackageList(ctx, *packages, *platform, *recipesDir, *outputDir, *enableSecurity,
			*timeoutMinutes, *successFile, *failureFile, *timeoutFile, *errorFile, *jsonOutput, *quiet)
		return
	}

	// Build single package from CLI args
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Error: package name is required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	packageName := fs.Arg(0)
	version := ""
	if fs.NArg() >= 2 {
		version = fs.Arg(1)
	}

	buildPackage(ctx, packageName, version, *platform, *allPlatforms, *recipesDir, *outputDir, *enableSecurity)
}

func buildPackage(ctx context.Context, packageName, version, platform string, allPlatforms bool, recipesDir, outputDir string, enableSecurity bool) {
	// Initialize repository
	defRepo := yaml.NewRecipeRepository(recipesDir)

	// Load package recipe
	def, err := defRepo.GetRecipe(ctx, packageName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Determine platforms to build
	var platforms []string
	//nolint:gocritic // ifElseChain: checking different boolean conditions, not suitable for switch
	if allPlatforms {
		// Build for all platforms in recipe
		for p := range def.Download.Platforms {
			platforms = append(platforms, p)
		}
		fmt.Printf("Building for all platforms: %v\n", platforms)
	} else if platform != "" {
		// Build for specified platform
		if _, exists := def.Download.Platforms[platform]; !exists {
			fmt.Fprintf(os.Stderr, "Error: platform %s not supported by %s\n", platform, packageName)
			fmt.Fprintf(os.Stderr, "Available platforms: ")
			for p := range def.Download.Platforms {
				fmt.Fprintf(os.Stderr, "%s ", p)
			}
			fmt.Fprintf(os.Stderr, "\n")
			os.Exit(1)
		}
		platforms = []string{platform}
	} else {
		// Auto-detect current platform
		platforms = []string{detectPlatform()}
		fmt.Printf("Auto-detected platform: %s\n", platforms[0])
	}

	// Initialize security components
	securityGateway := gateways.NewCompositeSecurityGateway()
	var securityOrch *orchestrators.SecurityOrchestrator
	if enableSecurity && def.Security.ScanVulnerabilities {
		securityService := services.NewSecurityService(securityGateway)
		securityOrch = orchestrators.NewSecurityOrchestrator(securityService)
	}

	// Initialize version fetcher and downloader
	versionFetcher := gateways.NewVersionFetcher()
	downloader := gateways.NewDownloader()
	scriptExecutor := gateways.NewScriptExecutor()
	packager := gateways.NewPackager()

	// Initialize build orchestrator
	buildOrch := orchestrators.NewBuildOrchestrator(
		defRepo,
		securityOrch,
		securityGateway,
		versionFetcher,
		downloader,
		scriptExecutor,
		packager,
		orchestrators.BuildOrchestratorConfig{
			EnableSecurityScan: enableSecurity,
			OutputDir:          outputDir,
		},
	)

	// Build for each platform
	fmt.Printf("\nBuilding %s", packageName)
	if version != "" {
		fmt.Printf(" version %s", version)
	}
	fmt.Println()
	if enableSecurity {
		fmt.Println("🔒 Security scanning: enabled")
	}
	fmt.Println()

	// Initialize security artifacts service
	securityArtifactsService := services.NewSecurityArtifactsService()

	successCount := 0
	for _, plat := range platforms {
		fmt.Printf("=== Building for %s ===\n", plat)

		result, err := buildOrch.BuildPackage(ctx, packageName, version, plat)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Build failed for %s: %v\n\n", plat, err)
			continue
		}

		fmt.Println(result.GetBuildSummary())

		// Generate security artifacts if enabled
		if enableSecurity && result.Artifact != nil && result.Artifact.Path != "" {
			fmt.Printf("\n🔒 Generating security artifacts for %s...\n", filepath.Base(result.Artifact.Path))

			artifacts, err := securityArtifactsService.GenerateAllArtifacts(ctx, result.Artifact.Path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  Security artifacts generation failed: %v\n", err)
			} else {
				fmt.Printf("✅ Security artifacts generated:\n")
				if artifacts.SHA256Path != "" {
					fmt.Printf("  - %s\n", filepath.Base(artifacts.SHA256Path))
				}
				if artifacts.SHA512Path != "" {
					fmt.Printf("  - %s\n", filepath.Base(artifacts.SHA512Path))
				}
				if artifacts.SBOMPath != "" {
					fmt.Printf("  - %s\n", filepath.Base(artifacts.SBOMPath))
				}
				if artifacts.ProvenancePath != "" {
					fmt.Printf("  - %s\n", filepath.Base(artifacts.ProvenancePath))
				}
			}
		}

		fmt.Println()
		successCount++
	}

	// Summary
	fmt.Printf("\n✅ Build complete: %d/%d platforms successful\n", successCount, len(platforms))
	if successCount < len(platforms) {
		os.Exit(1)
	}
}

func buildFromPackageList(ctx context.Context, packagesInput, targetPlatform, recipesDir, outputDir string,
	enableSecurity bool, timeoutMinutes int, successFile, failureFile, timeoutFile, errorFile, jsonOutput string, quiet bool) {

	// Parse packages input
	var packagesJSON string
	if strings.HasPrefix(packagesInput, "@") {
		// Read from file
		filename := strings.TrimPrefix(packagesInput, "@")
		//nolint:gosec // G304: User explicitly provides file path for packages input
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading packages file: %v\n", err)
			os.Exit(2)
		}
		packagesJSON = string(data)
	} else {
		packagesJSON = packagesInput
	}

	var packages []PackageBuildInput
	if err := json.Unmarshal([]byte(packagesJSON), &packages); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing packages JSON: %v\n", err)
		os.Exit(2)
	}

	if len(packages) == 0 {
		if !quiet {
			fmt.Println("No packages to build")
		}
		os.Exit(0)
	}

	// Build all packages
	report := buildPackages(ctx, packages, targetPlatform, recipesDir, outputDir, enableSecurity, timeoutMinutes, quiet)

	// Write report files
	if err := writeSuccessFile(successFile, report.SuccessDetails); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write success file: %v\n", err)
	}

	if err := writeFailureFile(failureFile, report.FailureDetails, report.TimeoutDetails); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write failure file: %v\n", err)
	}

	if err := writeTimeoutFile(timeoutFile, report.TimeoutDetails); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write timeout file: %v\n", err)
	}

	if err := writeErrorFile(errorFile, report.FailureDetails); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to write error file: %v\n", err)
	}

	// Write JSON report if requested
	if jsonOutput != "" {
		reportData, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to marshal JSON report: %v\n", err)
		} else {
			if err := os.WriteFile(jsonOutput, reportData, 0600); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write JSON report: %v\n", err)
			}
		}
	}

	// Print summary
	if !quiet {
		printBuildSummary(report, targetPlatform)
	}

	// Exit with error if all builds failed
	if report.SuccessfulBuilds == 0 && report.FailedBuilds > 0 {
		os.Exit(1)
	}
}

func buildPackages(ctx context.Context, packages []PackageBuildInput, targetPlatform, recipesDir, outputDir string, enableSecurity bool, timeoutMinutes int, quiet bool) BuildReport {
	startTime := time.Now()

	report := BuildReport{
		TotalPackages:     len(packages),
		SuccessDetails:    []BuildResult{},
		FailureDetails:    []BuildResult{},
		TimeoutDetails:    []BuildResult{},
		PlatformBreakdown: make(map[string]int),
	}

	// Initialize dependencies following architecture pattern
	recipeRepo := yaml.NewRecipeRepository(recipesDir)

	// Initialize security components
	securityGateway := gateways.NewCompositeSecurityGateway()
	var securityOrch *orchestrators.SecurityOrchestrator
	if enableSecurity {
		securityService := services.NewSecurityService(securityGateway)
		securityOrch = orchestrators.NewSecurityOrchestrator(securityService)
	}

	// Initialize other gateways
	versionFetcher := gateways.NewVersionFetcher()
	downloader := gateways.NewDownloader()
	scriptExecutor := gateways.NewScriptExecutor()
	packager := gateways.NewPackager()

	// Create build orchestrator following architecture
	buildOrchestrator := orchestrators.NewBuildOrchestrator(
		recipeRepo,
		securityOrch,
		securityGateway,
		versionFetcher,
		downloader,
		scriptExecutor,
		packager,
		orchestrators.BuildOrchestratorConfig{
			EnableSecurityScan: enableSecurity,
			OutputDir:          outputDir,
		},
	)

	// Initialize security artifacts service
	securityArtifactsService := services.NewSecurityArtifactsService()

	for _, pkg := range packages {
		if !quiet {
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Printf("📦 Processing package: %s v%s\n", pkg.Package, pkg.Version)
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		}

		// Load recipe to check platform support
		recipe, err := recipeRepo.GetRecipe(ctx, pkg.Package)
		if err != nil {
			if !quiet {
				fmt.Printf("  ❌ Failed to load recipe: %v\n\n", err)
			}
			report.FailureDetails = append(report.FailureDetails, BuildResult{
				Package:  pkg.Package,
				Version:  pkg.Version,
				Platform: targetPlatform,
				Status:   "error",
				Message:  fmt.Sprintf("Recipe not found: %v", err),
			})
			report.FailedBuilds++
			continue
		}

		// Check if package supports the target platform
		if !packageSupportsPlatform(recipe, targetPlatform) {
			if !quiet {
				fmt.Printf("  ⏭️  Skipping %s - platform %s not supported\n\n", pkg.Package, targetPlatform)
			}
			continue
		}

		// Build the package using orchestrator
		if !quiet {
			fmt.Printf("  🔨 Building %s v%s for %s\n", pkg.Package, pkg.Version, targetPlatform)
		}

		result := buildPackageWithOrchestrator(
			ctx,
			buildOrchestrator,
			securityArtifactsService,
			pkg.Package,
			pkg.Version,
			targetPlatform,
			enableSecurity,
			timeoutMinutes,
			quiet,
		)

		switch result.Status {
		case "success":
			report.SuccessfulBuilds++
			report.SuccessDetails = append(report.SuccessDetails, result)
			report.PlatformBreakdown[targetPlatform]++
			if !quiet {
				fmt.Printf("  ✅ Built %s %s successfully\n", pkg.Package, targetPlatform)
			}
		case "timeout":
			report.TimeoutBuilds++
			report.TimeoutDetails = append(report.TimeoutDetails, result)
			report.FailedBuilds++
			if !quiet {
				fmt.Printf("  ⏱️  Build timeout (%d min) for %s (%s)\n", timeoutMinutes, pkg.Package, targetPlatform)
			}
		case "error":
			report.FailedBuilds++
			report.FailureDetails = append(report.FailureDetails, result)
			if !quiet {
				fmt.Printf("  ❌ Build failed for %s (%s): %s\n", pkg.Package, targetPlatform, result.Message)
			}
		}

		if !quiet {
			fmt.Println()
		}
	}

	report.DurationSeconds = time.Since(startTime).Seconds()
	return report
}

func packageSupportsPlatform(recipe *entities.Recipe, platform string) bool {
	if len(recipe.Download.Platforms) == 0 {
		return false
	}

	// Check both the standard platform name and alternate naming conventions
	platformName := convertPlatformName(platform)

	for pname := range recipe.Download.Platforms {
		if pname == platform || pname == platformName {
			return true
		}
	}

	return false
}

// convertPlatformName converts between different platform naming conventions
func convertPlatformName(platform string) string {
	switch platform {
	case "linux-x86_64":
		return "linux-amd64"
	case "linux-amd64":
		return "linux-x86_64"
	default:
		return platform
	}
}

// buildPackageWithOrchestrator builds a single package using the orchestrator
func buildPackageWithOrchestrator(
	ctx context.Context,
	buildOrch *orchestrators.BuildOrchestrator,
	securityService *services.SecurityArtifactsService,
	packageName, version, platform string,
	enableSecurity bool,
	timeoutMinutes int,
	quiet bool,
) BuildResult {
	result := BuildResult{
		Package:  packageName,
		Version:  version,
		Platform: platform,
	}

	// Create context with timeout
	buildCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	// Execute build using orchestrator
	buildResult, err := buildOrch.BuildPackage(buildCtx, packageName, version, platform)
	if err != nil {
		if buildCtx.Err() == context.DeadlineExceeded {
			result.Status = "timeout"
			result.Message = fmt.Sprintf("Build exceeded %d minute timeout", timeoutMinutes)
		} else {
			result.Status = "error"
			result.Message = err.Error()
		}
		return result
	}

	// Generate security artifacts if enabled and artifact was created
	if enableSecurity && buildResult.Artifact != nil && buildResult.Artifact.Path != "" {
		_, err := securityService.GenerateAllArtifacts(buildCtx, buildResult.Artifact.Path)
		if err != nil {
			if !quiet {
				fmt.Printf("    ⚠️  Warning: Failed to generate security artifacts: %v\n", err)
			}
		}
	}

	result.Status = "success"
	return result
}

func writeSuccessFile(filename string, successes []BuildResult) error {
	if len(successes) == 0 {
		return os.WriteFile(filename, []byte{}, 0600)
	}

	var lines []string
	for _, s := range successes {
		lines = append(lines, fmt.Sprintf("%s:%s", s.Package, s.Version))
	}

	return os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func writeFailureFile(filename string, failures, timeouts []BuildResult) error {
	var lines []string

	for _, f := range failures {
		if f.Status == "timeout" {
			continue // Handled in timeout file
		}
		line := fmt.Sprintf("%s v%s (%s)", f.Package, f.Version, f.Platform)
		if f.Message != "" {
			line += " - " + f.Message
		}
		lines = append(lines, line)
	}

	for _, t := range timeouts {
		lines = append(lines, fmt.Sprintf("%s v%s (%s) - TIMEOUT", t.Package, t.Version, t.Platform))
	}

	if len(lines) == 0 {
		return os.WriteFile(filename, []byte{}, 0600)
	}

	return os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func writeTimeoutFile(filename string, timeouts []BuildResult) error {
	if len(timeouts) == 0 {
		return os.WriteFile(filename, []byte{}, 0600)
	}

	var lines []string
	for _, t := range timeouts {
		lines = append(lines, fmt.Sprintf("%s v%s (%s)", t.Package, t.Version, t.Platform))
	}

	return os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func writeErrorFile(filename string, errors []BuildResult) error {
	if len(errors) == 0 {
		return os.WriteFile(filename, []byte{}, 0600)
	}

	var lines []string
	for _, e := range errors {
		line := fmt.Sprintf("%s v%s (%s) - ERROR", e.Package, e.Version, e.Platform)
		if e.Message != "" {
			line += " - " + e.Message
		}
		lines = append(lines, line)
	}

	return os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

func printBuildSummary(report BuildReport, platform string) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("📊 Build Summary for %s\n", platform)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Printf("✅ Successful builds: %d\n", report.SuccessfulBuilds)
	if len(report.SuccessDetails) > 0 {
		for _, s := range report.SuccessDetails {
			fmt.Printf("  ✓ %s:%s\n", s.Package, s.Version)
		}
	}

	fmt.Println()

	fmt.Printf("❌ Failed builds: %d\n", report.FailedBuilds)
	if report.TimeoutBuilds > 0 {
		fmt.Printf("\n  ⏱️  Timeouts: %d\n", report.TimeoutBuilds)
		for _, t := range report.TimeoutDetails {
			fmt.Printf("    ✗ %s v%s (%s)\n", t.Package, t.Version, t.Platform)
		}
	}

	if len(report.FailureDetails) > 0 {
		fmt.Printf("\n  💥 Errors: %d\n", len(report.FailureDetails))
		for _, f := range report.FailureDetails {
			if f.Status == "timeout" {
				continue
			}
			fmt.Printf("    ✗ %s v%s (%s)", f.Package, f.Version, f.Platform)
			if f.Message != "" {
				fmt.Printf(" - %s", f.Message)
			}
			fmt.Println()
		}
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("⏱️  Duration: %.2f seconds\n", report.DurationSeconds)
}
