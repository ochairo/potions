package services

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Platform represents a build platform
type Platform string

// Supported build platforms for package releases
const (
	PlatformLinuxAMD64  Platform = "linux-amd64"
	PlatformLinuxARM64  Platform = "linux-arm64"
	PlatformDarwinAMD64 Platform = "darwin-x86_64"
	PlatformDarwinARM64 Platform = "darwin-arm64"
)

// ReleaseStatus represents the readiness status of a package for release
type ReleaseStatus string

// Release validation statuses
const (
	StatusReady               ReleaseStatus = "ready"
	StatusInsufficientBuilds  ReleaseStatus = "insufficient_builds"
	StatusMissingPlatforms    ReleaseStatus = "missing_platforms"
	StatusPlatformNotFound    ReleaseStatus = "platform_not_found"
	StatusVersionMismatch     ReleaseStatus = "version_mismatch"
	StatusInvalidArtifactName ReleaseStatus = "invalid_artifact_name"
	StatusNoArtifacts         ReleaseStatus = "no_artifacts"
	StatusPlatformMismatch    ReleaseStatus = "platform_mismatch"
	StatusUnexpectedPlatforms ReleaseStatus = "unexpected_platforms"
)

// ReleaseValidation contains the validation result for a package release
type ReleaseValidation struct {
	Status              ReleaseStatus
	ExpectedPlatforms   []Platform
	AvailablePlatforms  []Platform
	MissingPlatforms    []Platform
	UnexpectedPlatforms []Platform
	ExpectedCount       int
	AvailableCount      int
}

// IsReady returns true if the package is ready for release
func (rv *ReleaseValidation) IsReady() bool {
	return rv.Status == StatusReady
}

// ErrorMessage returns a human-readable error message if not ready
func (rv *ReleaseValidation) ErrorMessage(_, _ string) string {
	switch rv.Status {
	case StatusReady:
		return ""
	case StatusNoArtifacts:
		return fmt.Sprintf("No artifacts found (expected: %d platforms)", rv.ExpectedCount)
	case StatusPlatformMismatch:
		msg := fmt.Sprintf("Platform count mismatch (expected: %d, have: %d)", rv.ExpectedCount, rv.AvailableCount)
		if len(rv.MissingPlatforms) > 0 {
			msg += fmt.Sprintf("\n   Missing: %s", platformsToString(rv.MissingPlatforms))
		}
		if len(rv.UnexpectedPlatforms) > 0 {
			msg += fmt.Sprintf("\n   Unexpected: %s", platformsToString(rv.UnexpectedPlatforms))
		}
		return msg
	case StatusUnexpectedPlatforms:
		return fmt.Sprintf("Unexpected platforms found: %s", platformsToString(rv.UnexpectedPlatforms))
	default:
		return "Unknown status"
	}
}

// ReleaseService handles release validation logic
type ReleaseService struct{}

// NewReleaseService creates a new release service
func NewReleaseService() *ReleaseService {
	return &ReleaseService{}
}

// ValidateRelease validates if a package is ready for release based on recipe and available artifacts
func (s *ReleaseService) ValidateRelease(recipe *entities.Recipe, packageName, version string, artifactPaths []string) *ReleaseValidation {
	validation := &ReleaseValidation{}

	// Extract expected platforms from recipe
	validation.ExpectedPlatforms = s.extractExpectedPlatforms(recipe)
	validation.ExpectedCount = len(validation.ExpectedPlatforms)

	// Extract available platforms from artifact paths
	validation.AvailablePlatforms = s.extractAvailablePlatforms(packageName, version, artifactPaths)
	validation.AvailableCount = len(validation.AvailablePlatforms)

	// Determine missing and unexpected platforms
	validation.MissingPlatforms = s.findMissingPlatforms(validation.ExpectedPlatforms, validation.AvailablePlatforms)
	validation.UnexpectedPlatforms = s.findUnexpectedPlatforms(validation.ExpectedPlatforms, validation.AvailablePlatforms)

	// Determine status
	switch {
	case validation.AvailableCount == 0:
		validation.Status = StatusNoArtifacts
	case validation.AvailableCount != validation.ExpectedCount:
		validation.Status = StatusPlatformMismatch
	case len(validation.UnexpectedPlatforms) > 0:
		validation.Status = StatusUnexpectedPlatforms
	default:
		validation.Status = StatusReady
	}

	return validation
}

// extractExpectedPlatforms converts recipe platform definitions to our Platform type
func (s *ReleaseService) extractExpectedPlatforms(recipe *entities.Recipe) []Platform {
	var platforms []Platform

	for platformKey := range recipe.Download.Platforms {
		platform := s.recipePlatformToStandard(platformKey)
		if platform != "" {
			platforms = append(platforms, platform)
		}
	}

	return platforms
}

// recipePlatformToStandard maps recipe platform names to standard platform identifiers
// Both recipes and artifacts use the same naming: linux-amd64, linux-arm64, darwin-x86_64, darwin-arm64
func (s *ReleaseService) recipePlatformToStandard(recipePlatform string) Platform {
	switch recipePlatform {
	case "linux-amd64":
		return PlatformLinuxAMD64
	case "linux-arm64":
		return PlatformLinuxARM64
	case "darwin-x86_64":
		return PlatformDarwinAMD64
	case "darwin-arm64":
		return PlatformDarwinARM64
	default:
		return ""
	}
}

// extractAvailablePlatforms extracts platforms from artifact filenames
// Expected format: packageName-version-platform.tar.gz
func (s *ReleaseService) extractAvailablePlatforms(packageName, version string, artifactPaths []string) []Platform {
	platformSet := make(map[Platform]bool)

	// Clean version (remove 'v' prefix if present)
	versionClean := strings.TrimPrefix(version, "v")

	// Look for .tar.gz files only (not checksums or metadata)
	for _, path := range artifactPaths {
		basename := filepath.Base(path)

		// Must be a tarball
		if !strings.HasSuffix(basename, ".tar.gz") {
			continue
		}

		// Must match package-version pattern
		expectedPrefix := fmt.Sprintf("%s-%s-", packageName, versionClean)
		if !strings.HasPrefix(basename, expectedPrefix) {
			continue
		}

		// Extract platform from filename
		// Format: package-version-platform.tar.gz
		platformPart := strings.TrimPrefix(basename, expectedPrefix)
		platformPart = strings.TrimSuffix(platformPart, ".tar.gz")

		platform := Platform(platformPart)
		if s.isValidPlatform(platform) {
			platformSet[platform] = true
		}
	}

	// Convert set to slice
	platforms := make([]Platform, 0, len(platformSet))
	for platform := range platformSet {
		platforms = append(platforms, platform)
	}

	return platforms
}

// isValidPlatform checks if a platform string is valid
func (s *ReleaseService) isValidPlatform(platform Platform) bool {
	switch platform {
	case PlatformLinuxAMD64, PlatformLinuxARM64, PlatformDarwinAMD64, PlatformDarwinARM64:
		return true
	default:
		return false
	}
}

// findMissingPlatforms returns platforms that are expected but not available
func (s *ReleaseService) findMissingPlatforms(expected, available []Platform) []Platform {
	availableSet := make(map[Platform]bool)
	for _, p := range available {
		availableSet[p] = true
	}

	var missing []Platform
	for _, p := range expected {
		if !availableSet[p] {
			missing = append(missing, p)
		}
	}

	return missing
}

// findUnexpectedPlatforms returns platforms that are available but not expected
func (s *ReleaseService) findUnexpectedPlatforms(expected, available []Platform) []Platform {
	expectedSet := make(map[Platform]bool)
	for _, p := range expected {
		expectedSet[p] = true
	}

	var unexpected []Platform
	for _, p := range available {
		if !expectedSet[p] {
			unexpected = append(unexpected, p)
		}
	}

	return unexpected
}

// platformsToString converts a slice of platforms to a comma-separated string
func platformsToString(platforms []Platform) string {
	strs := make([]string, len(platforms))
	for i, p := range platforms {
		strs[i] = string(p)
	}
	return strings.Join(strs, ", ")
}
