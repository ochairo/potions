// Package orchestrators coordinates complex workflows across multiple domain services.
package orchestrators

import (
	"context"
	"fmt"
	"time"

	"github.com/ochairo/potions/internal/domain/entities"
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

// BuildOrchestrator coordinates the complete package build workflow
type BuildOrchestrator struct {
	defRepo        repositories.RecipeRepository
	securityOrch   *SecurityOrchestrator
	versionFetcher VersionFetcher
	downloader     Downloader
	scriptExecutor ScriptExecutor
	packager       Packager
	enableSecurity bool
	outputDir      string
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
	versionFetcher VersionFetcher,
	downloader Downloader,
	scriptExecutor ScriptExecutor,
	packager Packager,
	config BuildOrchestratorConfig,
) *BuildOrchestrator {
	outputDir := config.OutputDir
	if outputDir == "" {
		outputDir = "dist"
	}

	return &BuildOrchestrator{
		defRepo:        defRepo,
		securityOrch:   securityOrch,
		versionFetcher: versionFetcher,
		downloader:     downloader,
		scriptExecutor: scriptExecutor,
		packager:       packager,
		enableSecurity: config.EnableSecurityScan,
		outputDir:      outputDir,
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

	// Step 2: Fetch version if not provided
	if version == "" {
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
