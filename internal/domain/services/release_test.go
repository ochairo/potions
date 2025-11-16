package services

import (
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

func TestValidateRelease(t *testing.T) {
	tests := []struct {
		name            string
		recipe          *entities.Recipe
		packageName     string
		version         string
		artifactPaths   []string
		expectedStatus  ReleaseStatus
		expectedReady   bool
		expectedMissing int
	}{
		{
			name: "all platforms present - ready",
			recipe: &entities.Recipe{
				Download: entities.RecipeDownload{
					Platforms: map[string]entities.PlatformConfig{
						"linux-amd64":   {},
						"linux-arm64":   {},
						"darwin-x86_64": {},
						"darwin-arm64":  {},
					},
				},
			},
			packageName: "kubectl",
			version:     "v1.28.0",
			artifactPaths: []string{
				"kubectl-1.28.0-linux-amd64.tar.gz",
				"kubectl-1.28.0-linux-arm64.tar.gz",
				"kubectl-1.28.0-darwin-x86_64.tar.gz",
				"kubectl-1.28.0-darwin-arm64.tar.gz",
			},
			expectedStatus:  StatusReady,
			expectedReady:   true,
			expectedMissing: 0,
		},
		{
			name: "no artifacts - error",
			recipe: &entities.Recipe{
				Download: entities.RecipeDownload{
					Platforms: map[string]entities.PlatformConfig{
						"linux-amd64": {},
						"linux-arm64": {},
					},
				},
			},
			packageName:     "kubectl",
			version:         "v1.28.0",
			artifactPaths:   []string{},
			expectedStatus:  StatusNoArtifacts,
			expectedReady:   false,
			expectedMissing: 2,
		},
		{
			name: "missing platforms - error",
			recipe: &entities.Recipe{
				Download: entities.RecipeDownload{
					Platforms: map[string]entities.PlatformConfig{
						"linux-amd64":   {},
						"linux-arm64":   {},
						"darwin-x86_64": {},
						"darwin-arm64":  {},
					},
				},
			},
			packageName: "kubectl",
			version:     "v1.28.0",
			artifactPaths: []string{
				"kubectl-1.28.0-linux-arm64.tar.gz",
			},
			expectedStatus:  StatusPlatformMismatch,
			expectedReady:   false,
			expectedMissing: 3,
		},
		{
			name: "extra platforms - error",
			recipe: &entities.Recipe{
				Download: entities.RecipeDownload{
					Platforms: map[string]entities.PlatformConfig{
						"linux-amd64": {},
						"linux-arm64": {},
					},
				},
			},
			packageName: "buildah",
			version:     "v1.42.1",
			artifactPaths: []string{
				"buildah-1.42.1-linux-amd64.tar.gz",
				"buildah-1.42.1-linux-arm64.tar.gz",
				"buildah-1.42.1-darwin-x86_64.tar.gz",
				"buildah-1.42.1-darwin-arm64.tar.gz",
			},
			expectedStatus:  StatusPlatformMismatch,
			expectedReady:   false,
			expectedMissing: 0,
		},
		{
			name: "linux-only package - ready",
			recipe: &entities.Recipe{
				Download: entities.RecipeDownload{
					Platforms: map[string]entities.PlatformConfig{
						"linux-amd64": {},
						"linux-arm64": {},
					},
				},
			},
			packageName: "buildah",
			version:     "v1.42.1",
			artifactPaths: []string{
				"buildah-1.42.1-linux-amd64.tar.gz",
				"buildah-1.42.1-linux-arm64.tar.gz",
			},
			expectedStatus:  StatusReady,
			expectedReady:   true,
			expectedMissing: 0,
		},
		{
			name: "checksums and metadata ignored",
			recipe: &entities.Recipe{
				Download: entities.RecipeDownload{
					Platforms: map[string]entities.PlatformConfig{
						"linux-amd64": {},
					},
				},
			},
			packageName: "kubectl",
			version:     "1.28.0", // version without 'v' prefix
			artifactPaths: []string{
				"kubectl-1.28.0-linux-amd64.tar.gz",
				"kubectl-1.28.0-linux-amd64.tar.gz.sha256",
				"kubectl-1.28.0-linux-amd64.tar.gz.sha512",
				"kubectl-1.28.0-linux-amd64.tar.gz.sbom.json",
			},
			expectedStatus:  StatusReady,
			expectedReady:   true,
			expectedMissing: 0,
		},
	}

	service := NewReleaseService()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validation := service.ValidateRelease(tt.recipe, tt.packageName, tt.version, tt.artifactPaths)

			if validation.Status != tt.expectedStatus {
				t.Errorf("Status = %v, want %v", validation.Status, tt.expectedStatus)
			}

			if validation.IsReady() != tt.expectedReady {
				t.Errorf("IsReady() = %v, want %v", validation.IsReady(), tt.expectedReady)
			}

			if len(validation.MissingPlatforms) != tt.expectedMissing {
				t.Errorf("Missing platforms count = %d, want %d (platforms: %v)",
					len(validation.MissingPlatforms), tt.expectedMissing, validation.MissingPlatforms)
			}

			// Validate error message is generated (except for ready status)
			if tt.expectedStatus != StatusReady {
				msg := validation.ErrorMessage(tt.packageName, tt.version)
				if msg == "" {
					t.Error("Expected error message but got empty string")
				}
			}
		})
	}
}

func TestRecipePlatformMapping(t *testing.T) {
	tests := []struct {
		recipePlatform string
		expected       Platform
	}{
		{"linux-amd64", PlatformLinuxAMD64},
		{"linux-arm64", PlatformLinuxARM64},
		{"darwin-x86_64", PlatformDarwinAMD64},
		{"darwin-arm64", PlatformDarwinARM64},
		{"unknown", ""},
	}

	service := NewReleaseService()

	for _, tt := range tests {
		t.Run(tt.recipePlatform, func(t *testing.T) {
			result := service.recipePlatformToStandard(tt.recipePlatform)
			if result != tt.expected {
				t.Errorf("recipePlatformToStandard(%q) = %q, want %q", tt.recipePlatform, result, tt.expected)
			}
		})
	}
}

func TestExtractAvailablePlatforms(t *testing.T) {
	tests := []struct {
		name          string
		packageName   string
		version       string
		artifactPaths []string
		expected      []Platform
	}{
		{
			name:        "all four platforms",
			packageName: "kubectl",
			version:     "v1.28.0",
			artifactPaths: []string{
				"kubectl-1.28.0-linux-amd64.tar.gz",
				"kubectl-1.28.0-linux-arm64.tar.gz",
				"kubectl-1.28.0-darwin-x86_64.tar.gz",
				"kubectl-1.28.0-darwin-arm64.tar.gz",
			},
			expected: []Platform{
				PlatformLinuxAMD64,
				PlatformLinuxARM64,
				PlatformDarwinAMD64,
				PlatformDarwinARM64,
			},
		},
		{
			name:        "version without v prefix",
			packageName: "kubectl",
			version:     "1.28.0",
			artifactPaths: []string{
				"kubectl-1.28.0-linux-amd64.tar.gz",
			},
			expected: []Platform{PlatformLinuxAMD64},
		},
		{
			name:        "checksums filtered out",
			packageName: "kubectl",
			version:     "v1.28.0",
			artifactPaths: []string{
				"kubectl-1.28.0-linux-amd64.tar.gz",
				"kubectl-1.28.0-linux-amd64.tar.gz.sha256",
				"kubectl-1.28.0-linux-amd64.tar.gz.sha512",
			},
			expected: []Platform{PlatformLinuxAMD64},
		},
		{
			name:          "empty artifacts",
			packageName:   "kubectl",
			version:       "v1.28.0",
			artifactPaths: []string{},
			expected:      []Platform{},
		},
		{
			name:        "wrong package name ignored",
			packageName: "kubectl",
			version:     "v1.28.0",
			artifactPaths: []string{
				"helm-3.0.0-linux-amd64.tar.gz",
			},
			expected: []Platform{},
		},
	}

	service := NewReleaseService()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.extractAvailablePlatforms(tt.packageName, tt.version, tt.artifactPaths)

			// Convert to map for easier comparison (order doesn't matter)
			resultMap := make(map[Platform]bool)
			for _, p := range result {
				resultMap[p] = true
			}

			expectedMap := make(map[Platform]bool)
			for _, p := range tt.expected {
				expectedMap[p] = true
			}

			if len(resultMap) != len(expectedMap) {
				t.Errorf("Platform count = %d, want %d (got: %v, want: %v)",
					len(resultMap), len(expectedMap), result, tt.expected)
				return
			}

			for _, p := range tt.expected {
				if !resultMap[p] {
					t.Errorf("Missing expected platform: %v", p)
				}
			}
		})
	}
}
