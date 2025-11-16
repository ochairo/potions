package orchestrators

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ochairo/potions/internal/domain/entities"
)

// Mock implementations for testing
type mockRecipeRepository struct {
	recipe *entities.Recipe
	err    error
}

func (m *mockRecipeRepository) GetRecipe(_ context.Context, _ string) (*entities.Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.recipe, nil
}

func (m *mockRecipeRepository) ListRecipes(_ context.Context) ([]*entities.Recipe, error) {
	return nil, errors.New("not implemented")
}

func (m *mockRecipeRepository) GetRecipesByPlatform(_ context.Context, _ string) ([]*entities.Recipe, error) {
	return nil, errors.New("not implemented")
}

type mockVersionFetcher struct {
	version string
	err     error
}

func (m *mockVersionFetcher) FetchLatestVersion(_ *entities.Recipe) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.version, nil
}

type mockDownloader struct {
	artifact *entities.Artifact
	err      error
}

func (m *mockDownloader) DownloadArtifact(_ *entities.Recipe, _, _, _ string) (*entities.Artifact, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.artifact, nil
}

type mockScriptExecutor struct {
	err error
}

func (m *mockScriptExecutor) ExecuteBuildScripts(_ context.Context, _ *entities.Recipe, _ *entities.Artifact, _ string) error {
	return m.err
}

type mockPackager struct {
	artifact *entities.Artifact
	err      error
}

func (m *mockPackager) PackageArtifact(_ context.Context, _ *entities.Recipe, _ *entities.Artifact, _, _, _ string) (*entities.Artifact, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.artifact, nil
}

type mockSecurityGateway struct{}

func (m *mockSecurityGateway) VerifyGPGSignature(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockSecurityGateway) ImportGPGKeys(_ context.Context, _ []string) error {
	return nil
}

func (m *mockSecurityGateway) ImportGPGKeysFromURL(_ context.Context, _ string) error {
	return nil
}

// Test successful build workflow
func TestBuildOrchestrator_BuildPackage_Success(t *testing.T) {
	recipe := &entities.Recipe{
		Name: "kubectl",
		Download: entities.RecipeDownload{
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	artifact := &entities.Artifact{
		Path: "kubectl-1.28.0-linux-amd64.tar.gz",
	}

	orch := NewBuildOrchestrator(
		&mockRecipeRepository{recipe: recipe},
		nil,
		&mockSecurityGateway{},
		&mockVersionFetcher{version: "1.28.0"},
		&mockDownloader{artifact: artifact},
		&mockScriptExecutor{},
		&mockPackager{},
		BuildOrchestratorConfig{},
		nil,
	)

	result, err := orch.BuildPackage(context.Background(), "kubectl", "latest", "linux-amd64")

	if err != nil {
		t.Fatalf("Expected successful build, got error: %v", err)
	}

	if result.Recipe.Name != "kubectl" {
		t.Errorf("Recipe name = %v, want kubectl", result.Recipe.Name)
	}
}

// Test recipe not found error
func TestBuildOrchestrator_RecipeNotFound(t *testing.T) {
	orch := NewBuildOrchestrator(
		&mockRecipeRepository{err: errors.New("recipe not found")},
		nil,
		&mockSecurityGateway{},
		&mockVersionFetcher{},
		&mockDownloader{},
		&mockScriptExecutor{},
		&mockPackager{},
		BuildOrchestratorConfig{},
		nil,
	)

	_, err := orch.BuildPackage(context.Background(), "nonexistent", "1.0.0", "linux-amd64")

	if err == nil {
		t.Fatal("Expected error for nonexistent recipe, got nil")
	}
}

// Test unsupported platform error
func TestBuildOrchestrator_UnsupportedPlatform(t *testing.T) {
	recipe := &entities.Recipe{
		Name: "kubectl",
		Download: entities.RecipeDownload{
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	orch := NewBuildOrchestrator(
		&mockRecipeRepository{recipe: recipe},
		nil,
		&mockSecurityGateway{},
		&mockVersionFetcher{version: "1.0.0"},
		&mockDownloader{},
		&mockScriptExecutor{},
		&mockPackager{},
		BuildOrchestratorConfig{},
		nil,
	)

	_, err := orch.BuildPackage(context.Background(), "kubectl", "1.0.0", "windows-amd64")

	if err == nil {
		t.Fatal("Expected error for unsupported platform, got nil")
	}
}

// Test build failure
func TestBuildOrchestrator_BuildFailure(t *testing.T) {
	recipe := &entities.Recipe{
		Name: "kubectl",
		Download: entities.RecipeDownload{
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	orch := NewBuildOrchestrator(
		&mockRecipeRepository{recipe: recipe},
		nil,
		&mockSecurityGateway{},
		&mockVersionFetcher{version: "1.0.0"},
		&mockDownloader{err: errors.New("download failed")},
		&mockScriptExecutor{},
		&mockPackager{},
		BuildOrchestratorConfig{},
		nil,
	)

	_, err := orch.BuildPackage(context.Background(), "kubectl", "1.0.0", "linux-amd64")

	if err == nil {
		t.Fatal("Expected error for download failure, got nil")
	}
}

// Test build script execution failure
func TestBuildOrchestrator_BuildScriptFailure(t *testing.T) {
	recipe := &entities.Recipe{
		Name: "kubectl",
		Download: entities.RecipeDownload{
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	artifact := &entities.Artifact{Path: "kubectl.tar.gz"}

	orch := NewBuildOrchestrator(
		&mockRecipeRepository{recipe: recipe},
		nil,
		&mockSecurityGateway{},
		&mockVersionFetcher{version: "1.0.0"},
		&mockDownloader{artifact: artifact},
		&mockScriptExecutor{err: errors.New("build script failed")},
		&mockPackager{},
		BuildOrchestratorConfig{},
		nil,
	)

	_, err := orch.BuildPackage(context.Background(), "kubectl", "1.0.0", "linux-amd64")

	if err == nil {
		t.Fatal("Expected error for build script failure, got nil")
	}

	if !contains(err.Error(), "build/install failed") {
		t.Errorf("Error message = %v, want 'build/install failed'", err.Error())
	}
}

// Test packaging failure
func TestBuildOrchestrator_PackageError(t *testing.T) {
	recipe := &entities.Recipe{
		Name: "test",
		Download: entities.RecipeDownload{
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	orch := NewBuildOrchestrator(
		&mockRecipeRepository{recipe: recipe},
		nil,
		&mockSecurityGateway{},
		&mockVersionFetcher{version: "1.0.0"},
		&mockDownloader{artifact: &entities.Artifact{}},
		&mockScriptExecutor{},
		&mockPackager{err: errors.New("package failed")},
		BuildOrchestratorConfig{},
		nil,
	)

	_, err := orch.BuildPackage(context.Background(), "test", "1.0.0", "linux-amd64")

	if err == nil || !strings.Contains(err.Error(), "package failed") {
		t.Errorf("Expected package error, got: %v", err)
	}
}

// Test version fetching when empty version provided
func TestBuildOrchestrator_FetchLatestVersion(t *testing.T) {
	recipe := &entities.Recipe{
		Name: "kubectl",
		Download: entities.RecipeDownload{
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	artifact := &entities.Artifact{Path: "kubectl.tar.gz"}

	orch := NewBuildOrchestrator(
		&mockRecipeRepository{recipe: recipe},
		nil,
		&mockSecurityGateway{},
		&mockVersionFetcher{version: "1.28.5"},
		&mockDownloader{artifact: artifact},
		&mockScriptExecutor{},
		&mockPackager{artifact: artifact},
		BuildOrchestratorConfig{},
		nil,
	)

	// Pass empty version to trigger version fetching
	result, err := orch.BuildPackage(context.Background(), "kubectl", "", "linux-amd64")

	if err != nil {
		t.Fatalf("Expected successful build with fetched version, got error: %v", err)
	}

	if !result.Success {
		t.Error("Expected successful build result")
	}
}

// Test version fetch failure
func TestBuildOrchestrator_VersionFetchFailure(t *testing.T) {
	recipe := &entities.Recipe{
		Name: "kubectl",
		Download: entities.RecipeDownload{
			Platforms: map[string]entities.PlatformConfig{
				"linux-amd64": {OS: "linux", Arch: "amd64"},
			},
		},
	}

	orch := NewBuildOrchestrator(
		&mockRecipeRepository{recipe: recipe},
		nil,
		&mockSecurityGateway{},
		&mockVersionFetcher{err: errors.New("API rate limit exceeded")},
		&mockDownloader{},
		&mockScriptExecutor{},
		&mockPackager{},
		BuildOrchestratorConfig{},
		nil,
	)

	_, err := orch.BuildPackage(context.Background(), "kubectl", "", "linux-amd64")

	if err == nil {
		t.Fatal("Expected error for version fetch failure, got nil")
	}

	if !contains(err.Error(), "failed to fetch latest version") {
		t.Errorf("Error message = %v, want 'failed to fetch latest version'", err.Error())
	}
}

// Test build result summary for success
func TestBuildResult_GetBuildSummary_Success(t *testing.T) {
	result := &BuildResult{
		Recipe: &entities.Recipe{Name: "kubectl"},
		Artifact: &entities.Artifact{
			Platform: "linux-amd64",
		},
		Success: true,
	}

	summary := result.GetBuildSummary()

	if !contains(summary, "Build successful") {
		t.Errorf("Summary should contain 'Build successful', got: %s", summary)
	}

	if !contains(summary, "kubectl") {
		t.Errorf("Summary should contain package name 'kubectl', got: %s", summary)
	}
}

// Test build result summary for failure
func TestBuildResult_GetBuildSummary_Failure(t *testing.T) {
	result := &BuildResult{
		Success: false,
		Error:   errors.New("network timeout"),
	}

	summary := result.GetBuildSummary()

	if !contains(summary, "Build failed") {
		t.Errorf("Summary should contain 'Build failed', got: %s", summary)
	}

	if !contains(summary, "network timeout") {
		t.Errorf("Summary should contain error message, got: %s", summary)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
