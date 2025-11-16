// Package orchestrators coordinates complex workflows across multiple domain services.
package orchestrators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
	"github.com/ochairo/potions/internal/domain/interfaces"
	"github.com/ochairo/potions/internal/domain/interfaces/repositories"
)

// VersionFetcher interface for fetching latest versions
type VersionFetcher interface {
	FetchLatestVersion(def *entities.Recipe) (string, error)
}

// Downloader interface for downloading artifacts
type Downloader interface {
	DownloadArtifact(def *entities.Recipe, version, platform, outputDir string) (*entities.Artifact, error)
}

// ScriptExecutor interface for executing build scripts
type ScriptExecutor interface {
	ExecuteBuildScripts(ctx context.Context, def *entities.Recipe, artifact *entities.Artifact, outputDir string) error
}

// Packager interface for packaging built binaries into distributable archives
type Packager interface {
	PackageArtifact(ctx context.Context, def *entities.Recipe, artifact *entities.Artifact, version, platform, outputDir string) (*entities.Artifact, error)
}

// SecurityGateway interface for security operations
type SecurityGateway interface {
	VerifyGPGSignature(ctx context.Context, filePath, sigURL string) error
	ImportGPGKeys(ctx context.Context, keyIDs []string) error
	ImportGPGKeysFromURL(ctx context.Context, keysURL string) error
}

// BuildOrchestrator coordinates the complete package build workflow
type BuildOrchestrator struct {
	defRepo        repositories.RecipeRepository
	securityOrch   *SecurityOrchestrator
	securityGW     SecurityGateway
	versionFetcher VersionFetcher
	downloader     Downloader
	scriptExecutor ScriptExecutor
	packager       Packager
	enableSecurity bool
	outputDir      string
	logger         interfaces.Logger
}

// BuildOrchestratorConfig holds configuration for the orchestrator
type BuildOrchestratorConfig struct {
	EnableSecurityScan bool
	OutputDir          string
}

// NewBuildOrchestrator creates a new build orchestrator
func NewBuildOrchestrator(
	defRepo repositories.RecipeRepository,
	securityOrch *SecurityOrchestrator,
	securityGW SecurityGateway,
	versionFetcher VersionFetcher,
	downloader Downloader,
	scriptExecutor ScriptExecutor,
	packager Packager,
	config BuildOrchestratorConfig,
	logger interfaces.Logger,
) *BuildOrchestrator {
	outputDir := config.OutputDir
	if outputDir == "" {
		outputDir = "dist"
	}
	if logger == nil {
		logger = &interfaces.StdoutLogger{}
	}

	return &BuildOrchestrator{
		defRepo:        defRepo,
		securityOrch:   securityOrch,
		securityGW:     securityGW,
		versionFetcher: versionFetcher,
		downloader:     downloader,
		scriptExecutor: scriptExecutor,
		packager:       packager,
		enableSecurity: config.EnableSecurityScan,
		outputDir:      outputDir,
		logger:         logger,
	}
}

// BuildResult contains the result of a build operation
type BuildResult struct {
	Recipe           *entities.Recipe
	Artifact         *entities.Artifact
	SecurityResult   *SecurityWorkflowResult
	DownloadDuration time.Duration
	BuildDuration    time.Duration
	TotalDuration    time.Duration
	Success          bool
	Error            error
}

// BuildPackage executes the complete build workflow for a package
// If version is empty, it will fetch the latest version automatically
func (o *BuildOrchestrator) BuildPackage(ctx context.Context, packageName, version, platform string) (*BuildResult, error) {
	startTime := time.Now()
	result := &BuildResult{}

	// Step 1: Load package recipe
	def, err := o.defRepo.GetRecipe(ctx, packageName)
	if err != nil {
		result.Error = fmt.Errorf("failed to load recipe: %w", err)
		return result, result.Error
	}
	result.Recipe = def

	// Step 2: Fetch version if not provided or if "latest" is specified
	if version == "" || version == "latest" {
		fetchedVersion, err := o.versionFetcher.FetchLatestVersion(def)
		if err != nil {
			result.Error = fmt.Errorf("failed to fetch latest version: %w", err)
			return result, result.Error
		}
		version = fetchedVersion
	}

	// Step 3: Validate platform support
	_, hasPlatform := def.Download.Platforms[platform]
	if !hasPlatform {
		result.Error = fmt.Errorf("package %s does not support platform %s", packageName, platform)
		return result, result.Error
	}

	// Step 4: Download artifact
	downloadStart := time.Now()
	artifact, err := o.downloader.DownloadArtifact(def, version, platform, o.outputDir)
	if err != nil {
		result.Error = fmt.Errorf("failed to download artifact: %w", err)
		return result, result.Error
	}
	result.Artifact = artifact
	result.DownloadDuration = time.Since(downloadStart)

	// Step 4.5: Verify GPG signature if required (only for HTTP downloads)
	hasGPGKeys := len(def.Security.GPGKeyIDs) > 0 || def.Security.GPGKeysURL != ""
	if def.Security.VerifySignature && hasGPGKeys {
		if def.Download.Method == "git" {
			o.logger.Info("skipping GPG verification for git clone (no signature files in git repos)")
		} else {
			if err := o.verifyGPGSignature(ctx, def, artifact); err != nil {
				result.Error = fmt.Errorf("GPG signature verification failed: %w", err)
				return result, result.Error
			}
		}
	}

	// Step 5: Security workflow (if enabled and requested)
	if o.enableSecurity && def.Security.ScanVulnerabilities {
		secResult, err := o.securityOrch.PerformSecurityWorkflow(ctx, artifact)
		if err != nil {
			result.Error = fmt.Errorf("security workflow failed: %w", err)
			return result, result.Error
		}
		result.SecurityResult = secResult

		// Check if build should be blocked
		if secResult.Blocked {
			result.Error = fmt.Errorf("build blocked due to security issues: %s", secResult.BlockReason)
			return result, result.Error
		}
	}

	// Step 6: Build/Install using script executor
	buildStart := time.Now()
	if err := o.scriptExecutor.ExecuteBuildScripts(ctx, def, artifact, o.outputDir); err != nil {
		result.Error = fmt.Errorf("build/install failed: %w", err)
		return result, result.Error
	}
	result.BuildDuration = time.Since(buildStart)

	// Step 7: Package the built artifact into distributable tar.gz
	packagedArtifact, err := o.packager.PackageArtifact(ctx, def, artifact, version, platform, o.outputDir)
	if err != nil {
		result.Error = fmt.Errorf("packaging failed: %w", err)
		return result, result.Error
	}
	// Update artifact to point to the packaged tar.gz instead of extracted directory
	result.Artifact = packagedArtifact

	result.Success = true
	result.TotalDuration = time.Since(startTime)
	return result, nil
}

// GetBuildSummary returns a human-readable summary of the build
func (r *BuildResult) GetBuildSummary() string {
	if !r.Success {
		return fmt.Sprintf("Build failed: %v", r.Error)
	}

	summary := fmt.Sprintf(`Build successful!
Package: %s
Platform: %s
Download: %v
Build: %v
Total: %v`,
		r.Recipe.Name,
		r.Artifact.Platform,
		r.DownloadDuration,
		r.BuildDuration,
		r.TotalDuration,
	)

	if r.SecurityResult != nil {
		// Note: GetSecuritySummary is a method on SecurityOrchestrator, not SecurityWorkflowResult
		// For now, include basic security info
		if r.SecurityResult.Blocked {
			summary += fmt.Sprintf("\n\nSecurity: BLOCKED - %s", r.SecurityResult.BlockReason)
		} else {
			summary += fmt.Sprintf("\n\nSecurity: PASSED (score: %.1f/10.0)", r.SecurityResult.SecurityReport.Score)
		}
	}

	return summary
}

// verifyGPGSignature verifies the GPG signature of a downloaded artifact
func (o *BuildOrchestrator) verifyGPGSignature(ctx context.Context, def *entities.Recipe, artifact *entities.Artifact) error {
	// Import GPG keys from KEYS URL if provided (auto-fetch)
	switch {
	case def.Security.GPGKeysURL != "":
		o.logger.Info("auto-importing GPG keys from URL", interfaces.F("url", def.Security.GPGKeysURL))
		if err := o.securityGW.ImportGPGKeysFromURL(ctx, def.Security.GPGKeysURL); err != nil {
			return fmt.Errorf("failed to import GPG keys from URL: %w", err)
		}
	case len(def.Security.GPGKeyIDs) > 0:
		// Fallback to manual key IDs
		o.logger.Info("importing GPG keys", interfaces.F("keys", def.Security.GPGKeyIDs))
		if err := o.securityGW.ImportGPGKeys(ctx, def.Security.GPGKeyIDs); err != nil {
			return fmt.Errorf("failed to import GPG keys: %w", err)
		}
	default:
		return fmt.Errorf("no GPG keys configured (need gpg_keys_url or gpg_key_ids)")
	}

	// Determine signature URL
	var sigURL string
	switch {
	case def.Security.SignatureURL != "":
		// Use recipe-defined signature URL with template substitution
		sigURL = strings.ReplaceAll(def.Security.SignatureURL, "{version}", artifact.Version)
	case def.Download.DownloadURL != "":
		// Fallback: try common extensions
		sigURL = strings.ReplaceAll(def.Download.DownloadURL, "{version}", artifact.Version) + ".sig"
	default:
		return fmt.Errorf("no signature URL configured and no download URL to construct from")
	}

	// Verify signature
	o.logger.Info("verifying GPG signature", interfaces.F("url", sigURL))

	// Use the original download path for verification (not the extracted directory)
	verifyPath := artifact.DownloadPath
	if verifyPath == "" {
		verifyPath = artifact.Path // Fallback for non-extracted artifacts
	}

	if err := o.securityGW.VerifyGPGSignature(ctx, verifyPath, sigURL); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	o.logger.Info("GPG signature verified successfully")
	return nil
}
