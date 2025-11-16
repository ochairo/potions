package yaml

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRecipeRepository_GetRecipe_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test recipe file
	testYAML := []byte(`name: test-pkg
build_type: official_binary
download:
  platforms:
    linux-amd64:
      os: linux
      arch: amd64
`)
	err := os.WriteFile(filepath.Join(tmpDir, "test-pkg.yml"), testYAML, 0600)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := NewRecipeRepository(tmpDir)
	recipe, err := repo.GetRecipe(context.Background(), "test-pkg")
	if err != nil {
		t.Fatalf("GetRecipe() error = %v", err)
	}

	if recipe.Name != "test-pkg" {
		t.Errorf("GetRecipe() name = %v, want test-pkg", recipe.Name)
	}
}

func TestRecipeRepository_GetRecipe_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewRecipeRepository(tmpDir)

	_, err := repo.GetRecipe(context.Background(), "nonexistent")
	if err == nil {
		t.Error("GetRecipe() should return error for nonexistent recipe")
	}
}
