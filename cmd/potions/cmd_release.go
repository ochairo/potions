package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ochairo/potions/internal/domain-adapters/gateways"
	domainGateways "github.com/ochairo/potions/internal/domain/interfaces/gateways"
	"github.com/ochairo/potions/internal/domain/services"
	"github.com/ochairo/potions/internal/external-adapters/yaml"
)

// PackageRelease represents a package to release
type PackageRelease struct {
	Package string `json:"package"`
	Version string `json:"version"`
}

// ReleaseReport contains the results of release operations
type ReleaseReport struct {
	Created     []string `json:"created"`
	Skipped     []string `json:"skipped"`
	Failed      []string `json:"failed"`
	Total       int      `json:"total"`
	SuccessRate float64  `json:"success_rate"`
}

// RateLimitInfo contains GitHub API rate limit information
type RateLimitInfo struct {
	Limit     int
	Remaining int
	Reset     int64
}

// BatchConfig contains batch processing configuration
type BatchConfig struct {
	MaxReleasesPerRun int
	SafeThreshold     int // Remaining calls to consider as low
}

func runRelease(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("release", flag.ExitOnError)
	var (
		// Common flags
		owner = fs.String("owner", "ochairo", "GitHub repository owner")
		repo  = fs.String("repo", "potions", "GitHub repository name")

		// Single package flags
		binariesDir = fs.String("binaries", "dist", "Directory containing built binaries")
		dryRun      = fs.Bool("dry-run", false, "Show what would be released without actually releasing")
		draft       = fs.Bool("draft", false, "Create as draft release")
		prerelease  = fs.Bool("prerelease", false, "Mark as pre-release")

		// Multiple packages flags
		packages      = fs.String("packages", "", "JSON array of packages to release")
		artifactsDir  = fs.String("artifacts", "current-artifacts", "Directory containing artifacts")
		recipesDir    = fs.String("recipes", "recipes", "Directory containing recipe files")
		reportFile    = fs.String("report", "", "Write JSON report to file")
		failuresFile  = fs.String("failures", "release-failures.txt", "Write failures to file")
		successesFile = fs.String("successes", "release-successes.txt", "Write successes to file")
		maxReleases   = fs.Int("max-releases", 50, "Maximum releases to process per run (for rate limit safety)")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: potions release <package> <version> [options]
       potions release --packages <json> [options]

Create GitHub releases with built binaries and security attestations.

Examples:
  # Single package
  potions release kubectl v1.28.0
  potions release kubectl v1.28.0 --binaries ./dist
  potions release kubectl v1.28.0 --dry-run
  potions release kubectl v1.28.0 --draft --prerelease

  # Multiple packages from JSON
  potions release --packages '[{"package":"kubectl","version":"v1.28.0"}]'
  potions release --packages @packages.json --artifacts ./dist
  potions release --packages "$PACKAGES_JSON" --report report.json

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Environment Variables:
  GITHUB_TOKEN    GitHub personal access token (required)
`)
	}

	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing flags: %v\n", err)
		os.Exit(1)
	}

	// Release multiple packages from JSON input
	if *packages != "" {
		token := os.Getenv("GITHUB_TOKEN")
		if token == "" {
			fmt.Fprintf(os.Stderr, "Error: GITHUB_TOKEN environment variable is required\n")
			os.Exit(2)
		}
		if err := releaseFromPackageList(ctx, *packages, *artifactsDir, *recipesDir, *owner, *repo, token, *reportFile, *failuresFile, *successesFile, *maxReleases); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Release single package from CLI args
	if fs.NArg() < 2 {
		fmt.Fprintf(os.Stderr, "Error: package name and version are required\n\n")
		fs.Usage()
		os.Exit(1)
	}

	packageName := fs.Arg(0)
	version := fs.Arg(1)

	// Get GitHub token (only required for non-dry-run)
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" && !*dryRun {
		fmt.Fprintf(os.Stderr, "Error: GITHUB_TOKEN environment variable is required (not needed for --dry-run)\n")
		os.Exit(1)
	}

	if err := releasePackage(ctx, packageName, version, *binariesDir, *owner, *repo, token, *dryRun, *draft, *prerelease); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func releasePackage(ctx context.Context, packageName, version, binariesDir, owner, repo, token string, dryRun, draft, prerelease bool) error {
	fmt.Printf("🚀 Releasing %s %s\n", packageName, version)
	fmt.Printf("📁 Binaries directory: %s\n", binariesDir)

	// Initialize artifact finder
	artifactFinder := gateways.NewArtifactFinder()

	// Normalize version to ensure it starts with 'v'
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}

	// Tag format: packageName-version (e.g., kubectl-v1.28.0)
	tagName := fmt.Sprintf("%s-%s", packageName, version)

	// Load recipe to validate expected platforms
	recipeRepo := yaml.NewRecipeRepository("recipes")
	recipe, err := recipeRepo.GetRecipe(ctx, packageName)
	if err != nil {
		fmt.Printf("⚠️  Warning: Could not load recipe for %s: %v\n", packageName, err)
		fmt.Println("   Continuing without platform validation...")
	}

	// Find all artifacts for this package
	artifacts, err := artifactFinder.FindByGlob(binariesDir, packageName, version)
	if err != nil {
		return fmt.Errorf("failed to find artifacts: %w", err)
	}

	if len(artifacts) == 0 {
		return fmt.Errorf("no artifacts found in %s for %s %s", binariesDir, packageName, version)
	}

	fmt.Printf("📦 Found %d artifacts:\n", len(artifacts))
	for _, artifact := range artifacts {
		info, err := os.Stat(artifact)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: cannot stat artifact %s: %v\n", artifact, err)
			continue
		}
		size := "0"
		if info != nil {
			size = fmt.Sprintf("%d bytes", info.Size())
		}
		fmt.Printf("  - %s (%s)\n", filepath.Base(artifact), size)
	}

	// Validate platform coverage if recipe is available
	if recipe != nil {
		releaseService := services.NewReleaseService()
		validation := releaseService.ValidateRelease(recipe, packageName, version, artifacts)

		fmt.Printf("\n🔍 Platform Validation:\n")
		fmt.Printf("  Expected platforms: %d\n", validation.ExpectedCount)
		fmt.Printf("  Available platforms: %d\n", validation.AvailableCount)

		if !validation.IsReady() {
			errMsg := validation.ErrorMessage(packageName, version)
			fmt.Printf("\n❌ ERROR: %s\n", errMsg)
			return fmt.Errorf("platform validation failed: %s", errMsg)
		}

		fmt.Println("  ✅ All expected platforms present")
	}

	if dryRun {
		fmt.Println("\n🔍 Dry-run mode - no release will be created")
		fmt.Printf("Would create release:\n")
		fmt.Printf("  Repository: %s/%s\n", owner, repo)
		fmt.Printf("  Tag: %s\n", tagName)
		fmt.Printf("  Name: %s %s\n", packageName, version)
		fmt.Printf("  Draft: %v\n", draft)
		fmt.Printf("  Prerelease: %v\n", prerelease)
		fmt.Printf("  Artifacts: %d files\n", len(artifacts))
		return nil
	}

	// Initialize GitHub gateway
	githubGW := gateways.NewHTTPGitHubGateway(token)

	// Check if release already exists
	fmt.Printf("\n🔍 Checking if release %s already exists...\n", tagName)
	existingRelease, err := githubGW.GetRelease(ctx, owner, repo, tagName)
	if err == nil {
		fmt.Printf("⚠️  Release %s already exists: %s\n", tagName, existingRelease.HTMLURL)

		// List existing assets
		assets, err := githubGW.ListReleaseAssets(ctx, owner, repo, existingRelease.ID)
		if err != nil {
			return fmt.Errorf("failed to list existing assets: %w", err)
		}

		fmt.Printf("📦 Existing release has %d assets:\n", len(assets))
		for _, asset := range assets {
			fmt.Printf("  - %s (%d bytes)\n", asset.Name, asset.Size)
		}

		// Upload new artifacts to existing release
		return uploadArtifacts(ctx, githubGW, existingRelease.UploadURL, artifacts)
	}

	// Create new release
	fmt.Printf("\n✨ Creating new release %s...\n", tagName)
	releaseBody := generateReleaseBody(packageName, version, artifacts)

	release := &domainGateways.GitHubRelease{
		TagName:    tagName,
		Name:       fmt.Sprintf("%s %s", packageName, version),
		Body:       releaseBody,
		Draft:      draft,
		Prerelease: prerelease,
	}

	createdRelease, err := githubGW.CreateRelease(ctx, owner, repo, release)
	if err != nil {
		return fmt.Errorf("failed to create release: %w", err)
	}

	fmt.Printf("✅ Release created: %s\n", createdRelease.HTMLURL)

	// Upload artifacts
	return uploadArtifacts(ctx, githubGW, createdRelease.UploadURL, artifacts)
}

//nolint:gocyclo // High complexity acceptable for batch release orchestration (CLI handler)
func releaseFromPackageList(ctx context.Context, packagesJSON, artifactsDir, recipesDir, owner, repo, token, reportFile, failuresFile, successesFile string, maxReleases int) error {
	fmt.Println("🔍 Processing releases...")

	// Parse packages JSON
	var packages []PackageRelease

	// Handle @file syntax
	if strings.HasPrefix(packagesJSON, "@") {
		filename := strings.TrimPrefix(packagesJSON, "@")
		//nolint:gosec // G304: User explicitly provides file path for packages input
		data, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read packages file: %w", err)
		}
		if err := json.Unmarshal(data, &packages); err != nil {
			return fmt.Errorf("failed to parse packages JSON from file: %w", err)
		}
	} else {
		// Check if it's a file path
		if fileInfo, err := os.Stat(packagesJSON); err == nil && !fileInfo.IsDir() {
			//nolint:gosec // G304: User explicitly provides file path for packages JSON
			data, err := os.ReadFile(packagesJSON)
			if err != nil {
				return fmt.Errorf("failed to read packages file: %w", err)
			}
			if err := json.Unmarshal(data, &packages); err != nil {
				return fmt.Errorf("failed to parse packages JSON from file: %w", err)
			}
		} else {
			// Parse as direct JSON
			if err := json.Unmarshal([]byte(packagesJSON), &packages); err != nil {
				return fmt.Errorf("failed to parse packages JSON: %w", err)
			}
		}
	}

	if len(packages) == 0 {
		fmt.Println("ℹ️  No packages to release")
		return nil
	}

	fmt.Printf("📦 Processing %d package(s)\n\n", len(packages))

	// Initialize GitHub gateway early to check rate limits
	githubGW := gateways.NewHTTPGitHubGateway(token)

	// Split into batches based on rate limit
	batches := splitPackagesIntoBatches(ctx, packages, githubGW, maxReleases)

	if len(batches) > 1 {
		fmt.Printf("📊 Splitting into %d batch(es) for rate limit safety\n", len(batches))
		fmt.Printf("   Max releases per batch: %d\n", len(batches[0]))
		fmt.Println()
	}

	// Initialize services
	recipeRepo := yaml.NewRecipeRepository(recipesDir)
	releaseService := services.NewReleaseService()

	// Get existing releases
	fmt.Println("🔍 Fetching existing releases...")
	existingReleases, err := fetchExistingReleases(ctx, githubGW, owner, repo)
	if err != nil {
		return fmt.Errorf("failed to fetch existing releases: %w", err)
	}
	fmt.Printf("   Found %d existing releases\n\n", len(existingReleases))

	// Track results across all batches
	var created, skipped, failed []string
	var failureDetails []string

	// Process batches
	for batchNum, batch := range batches {
		if len(batches) > 1 {
			fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Printf("📦 Batch %d of %d (%d package(s))\n", batchNum+1, len(batches), len(batch))
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
		}

		// Process each package in this batch
		for i, pkg := range batch {
			fmt.Printf("[%d/%d] Processing %s v%s\n", i+1, len(batch), pkg.Package, pkg.Version)

			releaseTag := fmt.Sprintf("%s-%s", pkg.Package, pkg.Version)

			// Check if already exists
			if existingReleases[releaseTag] {
				fmt.Printf("  ⏭️  Release already exists, skipping\n\n")
				skipped = append(skipped, fmt.Sprintf("%s v%s", pkg.Package, pkg.Version))
				continue
			}

			// Load recipe
			recipe, err := recipeRepo.GetRecipe(ctx, pkg.Package)
			if err != nil {
				errMsg := fmt.Sprintf("%s v%s - NO_RECIPE: %v", pkg.Package, pkg.Version, err)
				fmt.Printf("  ❌ %s\n\n", errMsg)
				failed = append(failed, fmt.Sprintf("%s v%s", pkg.Package, pkg.Version))
				failureDetails = append(failureDetails, errMsg)
				continue
			}

			// Initialize artifact finder
			artifactFinder := gateways.NewArtifactFinder()

			// Find artifacts
			artifacts, err := artifactFinder.FindRecursive(artifactsDir, pkg.Package, pkg.Version)
			if err != nil {
				errMsg := fmt.Sprintf("%s v%s - FIND_ERROR: %v", pkg.Package, pkg.Version, err)
				fmt.Printf("  ❌ %s\n\n", errMsg)
				failed = append(failed, fmt.Sprintf("%s v%s", pkg.Package, pkg.Version))
				failureDetails = append(failureDetails, errMsg)
				continue
			}

			// Debug: Show found artifacts by type
			fmt.Printf("  📦 Found %d artifacts:\n", len(artifacts))
			tarballCount := 0
			checksumCount := 0
			sbomCount := 0
			for _, a := range artifacts {
				basename := filepath.Base(a)
				switch {
				case strings.HasSuffix(basename, ".tar.gz"):
					tarballCount++
					fmt.Printf("     - %s (tarball)\n", basename)
				case strings.HasSuffix(basename, ".sha256"):
					checksumCount++
				case strings.HasSuffix(basename, ".sbom.json"):
					sbomCount++
				case strings.HasSuffix(basename, ".provenance.json"):
					// Don't log individually
				default:
					fmt.Printf("     - %s (other)\n", basename)
				}
			}
			if checksumCount > 0 || sbomCount > 0 {
				fmt.Printf("     + %d checksums, %d SBOMs, etc.\n", checksumCount, sbomCount)
			}

			// Early validation: must have at least one tarball
			if tarballCount == 0 {
				errMsg := fmt.Sprintf("%s v%s - NO_TARBALLS: Found %d artifacts but no .tar.gz files",
					pkg.Package, pkg.Version, len(artifacts))
				fmt.Printf("  ❌ %s\n\n", errMsg)
				failed = append(failed, fmt.Sprintf("%s v%s", pkg.Package, pkg.Version))
				failureDetails = append(failureDetails, errMsg)
				continue
			}

			// Validate platforms
			validation := releaseService.ValidateRelease(recipe, pkg.Package, pkg.Version, artifacts)
			if !validation.IsReady() {
				errMsg := fmt.Sprintf("%s v%s - VALIDATION: %s", pkg.Package, pkg.Version, validation.ErrorMessage(pkg.Package, pkg.Version))
				fmt.Printf("  ❌ %s\n", errMsg)
				fmt.Printf("     Expected: %d, Available: %d\n", validation.ExpectedCount, validation.AvailableCount)
				if len(validation.MissingPlatforms) > 0 {
					fmt.Printf("     Missing: %v\n", validation.MissingPlatforms)
				}
				if len(validation.AvailablePlatforms) > 0 {
					fmt.Printf("     Available: %v\n", validation.AvailablePlatforms)
				}
				fmt.Println()
				failed = append(failed, fmt.Sprintf("%s v%s", pkg.Package, pkg.Version))
				failureDetails = append(failureDetails, errMsg)
				continue
			}

			// Warn if not all platforms are present
			if validation.AvailableCount < validation.ExpectedCount {
				fmt.Printf("  ⚠️  Partial release: %d/%d platforms\n", validation.AvailableCount, validation.ExpectedCount)
				if len(validation.MissingPlatforms) > 0 {
					fmt.Printf("     Missing: %v\n", validation.MissingPlatforms)
				}
			} else {
				fmt.Printf("  ✅ Validation passed (%d platforms)\n", validation.AvailableCount)
			}

			// Create release
			releaseBody := generateReleaseBody(pkg.Package, pkg.Version, artifacts)

			// Add warning if not all platforms are available
			if validation.AvailableCount < validation.ExpectedCount {
				warningNote := fmt.Sprintf("\n> ⚠️ **Note**: This release is missing some platforms. Available: %d/%d\n",
					validation.AvailableCount, validation.ExpectedCount)
				if len(validation.MissingPlatforms) > 0 {
					missing := make([]string, len(validation.MissingPlatforms))
					for i, p := range validation.MissingPlatforms {
						missing[i] = string(p)
					}
					warningNote += fmt.Sprintf("> Missing: %s\n", strings.Join(missing, ", "))
				}
				releaseBody = warningNote + "\n" + releaseBody
			}

			release := &domainGateways.GitHubRelease{
				TagName:    releaseTag,
				Name:       fmt.Sprintf("%s %s", pkg.Package, pkg.Version),
				Body:       releaseBody,
				Draft:      false,
				Prerelease: false,
			}

			fmt.Printf("  🚀 Creating release...\n")
			createdRelease, err := githubGW.CreateRelease(ctx, owner, repo, release)
			if err != nil {
				errMsg := fmt.Sprintf("%s v%s - CREATE_FAILED: %v", pkg.Package, pkg.Version, err)
				fmt.Printf("  ❌ %s\n\n", errMsg)
				failed = append(failed, fmt.Sprintf("%s v%s", pkg.Package, pkg.Version))
				failureDetails = append(failureDetails, errMsg)
				continue
			}

			// Upload artifacts
			fmt.Printf("  📤 Uploading %d artifact(s)...\n", len(artifacts))
			if err := uploadArtifacts(ctx, githubGW, createdRelease.UploadURL, artifacts); err != nil {
				errMsg := fmt.Sprintf("%s v%s - UPLOAD_FAILED: %v", pkg.Package, pkg.Version, err)
				fmt.Printf("  ⚠️  %s\n", errMsg)
				// Don't mark as completely failed if release was created
				fmt.Printf("  ⚠️  Release created but uploads incomplete: %s\n", createdRelease.HTMLURL)
			} else {
				fmt.Printf("  ✅ Release created successfully\n")
				fmt.Printf("     %s\n", createdRelease.HTMLURL)
			}

			created = append(created, fmt.Sprintf("%s v%s", pkg.Package, pkg.Version))
			fmt.Println()
		}
	}

	// Print summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 Batch Release Summary")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("✅ Releases created: %d\n", len(created))
	fmt.Printf("⏭️  Releases skipped (already exist): %d\n", len(skipped))
	fmt.Printf("❌ Failed releases: %d\n", len(failed))

	total := len(created) + len(skipped) + len(failed)
	fmt.Printf("📦 Total packages processed: %d of %d\n", total, len(packages))

	if total > 0 {
		successRate := float64(len(created)+len(skipped)) * 100.0 / float64(total)
		fmt.Printf("🎯 Success rate: %.1f%%\n", successRate)
	}

	// Show batch information
	if len(batches) > 1 {
		fmt.Printf("\n📊 Batch Processing:\n")
		fmt.Printf("   Total batches: %d\n", len(batches))
		fmt.Printf("   Max releases per batch: %d\n", len(batches[0]))
	}

	// Show if packages were not processed
	if total < len(packages) {
		unprocessed := len(packages) - total
		fmt.Printf("\n⏳ Unprocessed packages: %d (will be processed in next workflow run)\n", unprocessed)
		fmt.Printf("   Reason: Rate limit safety threshold reached\n")
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Write failures file
	if len(failureDetails) > 0 && failuresFile != "" {
		if err := os.WriteFile(failuresFile, []byte(strings.Join(failureDetails, "\n")+"\n"), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write failures file: %v\n", err)
		}
		fmt.Println("\n❌ Failed Releases:")
		for _, f := range failureDetails {
			fmt.Printf("  • %s\n", f)
		}
	}

	// Write successes file
	if len(created) > 0 && successesFile != "" {
		if err := os.WriteFile(successesFile, []byte(strings.Join(created, "\n")+"\n"), 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to write successes file: %v\n", err)
		}
	}

	// Write JSON report
	if reportFile != "" {
		report := ReleaseReport{
			Created: created,
			Skipped: skipped,
			Failed:  failed,
			Total:   total,
		}
		if total > 0 {
			report.SuccessRate = float64(len(created)+len(skipped)) * 100.0 / float64(total)
		}

		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal report: %w", err)
		}

		if err := os.WriteFile(reportFile, data, 0600); err != nil {
			return fmt.Errorf("failed to write report file: %w", err)
		}
	}

	// Exit with error if all releases failed
	if len(created) == 0 && len(failed) > 0 {
		fmt.Println("\n⚠️  Warning: No releases were created")
		return fmt.Errorf("all %d release(s) failed", len(failed))
	}

	return nil
}

func uploadArtifacts(ctx context.Context, githubGW *gateways.HTTPGitHubGateway, uploadURL string, artifacts []string) error {
	fmt.Printf("\n📤 Uploading %d artifacts...\n", len(artifacts))

	var uploadErrors []error
	successCount := 0

	for i, artifactPath := range artifacts {
		filename := filepath.Base(artifactPath)
		fmt.Printf("  [%d/%d] Uploading %s... ", i+1, len(artifacts), filename)

		//nolint:gosec // G304: artifactPath is from glob pattern for release uploads
		file, err := os.Open(artifactPath)
		if err != nil {
			fmt.Printf("❌\n")
			uploadErrors = append(uploadErrors, fmt.Errorf("failed to open %s: %w", filename, err))
			continue
		}

		asset, err := githubGW.UploadAsset(ctx, uploadURL, filename, file)
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("❌\n")
			uploadErrors = append(uploadErrors, fmt.Errorf("failed to close %s: %w", filename, closeErr))
			continue
		}

		if err != nil {
			fmt.Printf("❌\n")
			uploadErrors = append(uploadErrors, fmt.Errorf("failed to upload %s: %w", filename, err))
			continue
		}

		fmt.Printf("✅ (%d bytes)\n", asset.Size)
		successCount++
	}

	if len(uploadErrors) > 0 {
		fmt.Printf("\n⚠️  Upload summary: %d succeeded, %d failed\n", successCount, len(uploadErrors))
		for _, err := range uploadErrors {
			fmt.Printf("  ❌ %v\n", err)
		}

		// Only return error if ALL uploads failed
		if successCount == 0 {
			return fmt.Errorf("all %d artifact uploads failed", len(uploadErrors))
		}

		// Partial success - warn but don't fail the release
		fmt.Printf("⚠️  Warning: Partial upload - continuing with %d successful artifacts\n", successCount)
		return nil
	}

	fmt.Println("\n🎉 All artifacts uploaded successfully!")
	return nil
}

func generateReleaseBody(packageName, version string, artifacts []string) string {
	var body strings.Builder

	body.WriteString(fmt.Sprintf("# %s %s\n\n", packageName, version))
	body.WriteString("Prebuilt binaries with security scanning and attestations.\n\n")

	// Group artifacts by platform
	platformArtifacts := make(map[string][]string)
	for _, artifact := range artifacts {
		basename := filepath.Base(artifact)

		// Extract platform from filename
		// Format: packageName-version-platform.extension
		parts := strings.Split(basename, "-")
		if len(parts) >= 3 {
			platform := parts[len(parts)-1]
			// Remove extension
			platform = strings.Split(platform, ".")[0]
			platformArtifacts[platform] = append(platformArtifacts[platform], basename)
		}
	}

	if len(platformArtifacts) > 0 {
		body.WriteString("## Platform Support\n\n")
		for platform, files := range platformArtifacts {
			body.WriteString(fmt.Sprintf("### %s\n\n", platform))
			for _, file := range files {
				ext := filepath.Ext(file)
				var description string
				switch {
				case strings.HasSuffix(file, ".tar.gz"):
					description = "Binary tarball"
				case ext == ".sha256":
					description = "SHA256 checksum"
				case ext == ".json" && strings.Contains(file, "sbom"):
					description = "SBOM (Software Bill of Materials)"
				case ext == ".json" && strings.Contains(file, "provenance"):
					description = "SLSA Provenance attestation"
				default:
					description = "Artifact"
				}
				body.WriteString(fmt.Sprintf("- `%s` - %s\n", file, description))
			}
			body.WriteString("\n")
		}
	}

	body.WriteString("## Installation\n\n")
	body.WriteString("```bash\n")
	body.WriteString("# Download for your platform\n")
	body.WriteString(fmt.Sprintf("curl -LO https://github.com/ochairo/potions/releases/download/%s-%s/%s-%s-<platform>.tar.gz\n\n",
		packageName, version, packageName, strings.TrimPrefix(version, "v")))
	body.WriteString("# Verify checksum\n")
	body.WriteString(fmt.Sprintf("curl -LO https://github.com/ochairo/potions/releases/download/%s-%s/%s-%s-<platform>.tar.gz.sha256\n",
		packageName, version, packageName, strings.TrimPrefix(version, "v")))
	body.WriteString(fmt.Sprintf("shasum -a 256 -c %s-%s-<platform>.tar.gz.sha256\n\n", packageName, strings.TrimPrefix(version, "v")))
	body.WriteString("# Extract and install\n")
	body.WriteString(fmt.Sprintf("tar xzf %s-%s-<platform>.tar.gz\n", packageName, strings.TrimPrefix(version, "v")))
	body.WriteString("```\n\n")

	body.WriteString("## Security\n\n")
	body.WriteString("All binaries are:\n")
	body.WriteString("- ✅ Scanned for vulnerabilities using OSV\n")
	body.WriteString("- ✅ Analyzed for suspicious patterns\n")
	body.WriteString("- ✅ Provided with SBOM (CycloneDX format)\n")
	body.WriteString("- ✅ Attested with SLSA provenance\n")

	return body.String()
}

// fetchExistingReleases gets a map of existing release tags
func fetchExistingReleases(ctx context.Context, githubGW *gateways.HTTPGitHubGateway, owner, repo string) (map[string]bool, error) {
	releases, err := githubGW.ListReleases(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to list releases: %w", err)
	}

	existingTags := make(map[string]bool, len(releases))
	for _, release := range releases {
		existingTags[release.TagName] = true
	}

	return existingTags, nil
}

// calculateMaxSafeReleases calculates how many releases can be safely processed
// given the current API rate limit. Leaves a safety margin.
func calculateMaxSafeReleases(remaining, maxConfigured int) int {
	const safetyMargin = 200 // Reserve 200 calls for other operations
	const callsPerRelease = 8

	if remaining <= safetyMargin {
		return 0 // Not enough quota
	}

	available := remaining - safetyMargin
	maxReleases := available / callsPerRelease

	// Cap at configured maximum
	if maxReleases > maxConfigured {
		maxReleases = maxConfigured
	}

	return maxReleases
}

// splitPackagesIntoBatches splits the packages into batches based on rate limit
func splitPackagesIntoBatches(_ context.Context, packages []PackageRelease, _ *gateways.HTTPGitHubGateway, maxReleases int) [][]PackageRelease {
	if len(packages) == 0 {
		return nil
	}

	// Use configured maximum or calculate from rate limit
	maxPerBatch := maxReleases
	if maxPerBatch <= 0 {
		maxPerBatch = calculateMaxSafeReleases(5000, 50)
	}
	if maxPerBatch == 0 {
		maxPerBatch = 25 // Absolute minimum
	}

	var batches [][]PackageRelease
	for i := 0; i < len(packages); i += maxPerBatch {
		end := i + maxPerBatch
		if end > len(packages) {
			end = len(packages)
		}
		batches = append(batches, packages[i:end])
	}

	return batches
}
