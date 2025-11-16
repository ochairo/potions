package test_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ochairo/potions/internal/domain-adapters/gateways"
	orchestrators "github.com/ochairo/potions/internal/domain-orchestrators"
	"github.com/ochairo/potions/internal/external-adapters/yaml"
)

// TestEndToEnd_AllFixedPackages validates all packages with path fixes
func TestEndToEnd_AllFixedPackages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Get package name from environment variable or test run pattern
	packageName := os.Getenv("TEST_PACKAGE")

	// If not set via env var, try to extract from test name (subtest pattern)
	if packageName == "" {
		packageName = t.Name()
		if idx := strings.LastIndex(packageName, "/"); idx >= 0 {
			packageName = packageName[idx+1:]
		}
	}

	if packageName == "" || packageName == "TestEndToEnd_AllFixedPackages" {
		t.Skip("No package specified - set TEST_PACKAGE env var or run with -run TestEndToEnd_AllFixedPackages/<package>")
	}

	platform := "linux-amd64"
	// macOS CI support
	if os.Getenv("RUNNER_OS") == "macOS" {
		platform = "darwin-arm64"
	}

	recipesDir := "../recipes"
	recipeRepo := yaml.NewRecipeRepository(recipesDir)

	// Setup isolated test environment with unique directories inside test-dist/
	testDistDir := filepath.Join("..", "test-dist", "integration-tests")
	tmpDir := filepath.Join(testDistDir, packageName+"-"+platform)
	testHomeDir := filepath.Join(tmpDir, "home")
	outputDir := filepath.Join(tmpDir, "output")
	cacheDir := filepath.Join(tmpDir, "cache")

	// Clean up at the end
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	if err := os.MkdirAll(testHomeDir, 0750); err != nil {
		t.Fatalf("Failed to create test home directory: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0750); err != nil {
		t.Fatalf("Failed to create output directory: %v", err)
	}
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Set environment variables to isolate test
	oldHome := os.Getenv("HOME")
	oldXdgCache := os.Getenv("XDG_CACHE_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("XDG_CACHE_HOME", oldXdgCache)
	})
	_ = os.Setenv("HOME", testHomeDir)
	_ = os.Setenv("XDG_CACHE_HOME", cacheDir)

	// Initialize components
	versionFetcher := gateways.NewVersionFetcher()
	downloader := gateways.NewDownloader()
	scriptExecutor := gateways.NewScriptExecutor()
	packager := gateways.NewPackager()

	orchestrator := orchestrators.NewBuildOrchestrator(
		recipeRepo,
		nil,
		gateways.NewCompositeSecurityGateway(),
		versionFetcher,
		downloader,
		scriptExecutor,
		packager,
		orchestrators.BuildOrchestratorConfig{
			EnableSecurityScan: false,
			OutputDir:          outputDir,
		},
	)

	ctx := context.Background()

	// Load recipe
	recipe, err := recipeRepo.GetRecipe(ctx, packageName)
	if err != nil {
		t.Fatalf("Failed to get recipe: %v", err)
	}

	// Check if platform is supported
	if _, exists := recipe.Download.Platforms[platform]; !exists {
		t.Skipf("Package %s does not support platform %s", packageName, platform)
		return
	}

	// Fetch latest version
	version, err := versionFetcher.FetchLatestVersion(recipe)
	if err != nil {
		// Skip test only if actively rate limited (0 remaining)
		errMsg := err.Error()
		if strings.Contains(errMsg, "0 remaining") && strings.Contains(errMsg, "resets at") {
			t.Skipf("GitHub API rate limited: %v", err)
			return
		}
		t.Fatalf("Failed to fetch latest version: %v", err)
	}

	t.Logf("Building %s version %s for platform %s", packageName, version, platform)

	// Execute build
	result, err := orchestrator.BuildPackage(ctx, packageName, version, platform)

	// Verify success
	if err != nil {
		t.Fatalf("BuildPackage failed: %v", err)
	}

	if !result.Success {
		t.Fatalf("Build was not successful: %v", result.Error)
	}

	if result.Artifact == nil {
		t.Fatal("No artifact was produced")
	}

	// Verify artifact exists and is a tar.gz
	if _, err := os.Stat(result.Artifact.Path); os.IsNotExist(err) {
		t.Errorf("Artifact file does not exist: %s", result.Artifact.Path)
	}

	// Verify artifact is compressed
	if !strings.HasSuffix(result.Artifact.Path, ".tar.gz") {
		t.Errorf("Expected .tar.gz artifact, got: %s", result.Artifact.Path)
	}

	t.Logf("✅ Successfully built: %s", filepath.Base(result.Artifact.Path))
}

// TestErrorPropagation_MissingRecipe verifies errors propagate correctly
func TestErrorPropagation_MissingRecipe(t *testing.T) {
	tmpDir := t.TempDir()
	emptyDir := filepath.Join(tmpDir, "recipes")
	if err := os.MkdirAll(emptyDir, 0750); err != nil {
		t.Fatalf("Failed to create empty dir: %v", err)
	}

	recipeRepo := yaml.NewRecipeRepository(emptyDir)
	orchestrator := orchestrators.NewBuildOrchestrator(
		recipeRepo,
		nil,
		gateways.NewCompositeSecurityGateway(),
		gateways.NewVersionFetcher(),
		gateways.NewDownloader(),
		gateways.NewScriptExecutor(),
		gateways.NewPackager(),
		orchestrators.BuildOrchestratorConfig{
			OutputDir: tmpDir,
		},
	)

	_, err := orchestrator.BuildPackage(context.Background(), "nonexistent", "1.0.0", "linux-amd64")

	if err == nil {
		t.Fatal("Expected error for nonexistent recipe")
	}

	t.Logf("✅ Correctly handled missing recipe: %v", err)
}

// TestVersionFetching_MultipleSourceTypes validates version fetching across different sources
func TestVersionFetching_MultipleSourceTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	versionFetcher := gateways.NewVersionFetcher()

	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "GitHub Release",
			yaml: `name: age
version:
  source: github-release:FiloSottile/age
  extract_pattern: 'v(\d+\.\d+\.\d+)$'
  cleanup: s/^v//`,
		},
		{
			name: "URL Source",
			yaml: `name: kubectl
version:
  source: url:https://dl.k8s.io/release/stable.txt
  extract_pattern: 'v?(\d+\.\d+\.\d+)'
  cleanup: s/^v//`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := yaml.NewRecipeParser()
			recipe, err := parser.Parse([]byte(tt.yaml))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			version, err := versionFetcher.FetchLatestVersion(recipe)
			if err != nil {
				// Skip test only if actively rate limited (0 remaining)
				errMsg := err.Error()
				if strings.Contains(errMsg, "0 remaining") && strings.Contains(errMsg, "resets at") {
					t.Skipf("GitHub API rate limited: %v", err)
					return
				}
				t.Errorf("FetchLatestVersion failed: %v", err)
				return
			}

			if version == "" || !strings.Contains(version, ".") {
				t.Errorf("Invalid version: %s", version)
				return
			}

			t.Logf("✅ %s version: %s", tt.name, version)
		})
	}
}
