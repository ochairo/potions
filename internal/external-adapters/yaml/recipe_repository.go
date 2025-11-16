package yaml

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ochairo/potions/internal/domain/entities"
)

// RecipeRepository implements repositories.RecipeRepository using YAML files
type RecipeRepository struct {
	recipesDir string
	parser     *RecipeParser
}

// NewRecipeRepository creates a new YAML-based recipe repository
func NewRecipeRepository(recipesDir string) *RecipeRepository {
	return &RecipeRepository{
		recipesDir: recipesDir,
		parser:     NewRecipeParser(),
	}
}

// GetRecipe retrieves a package recipe by name
func (r *RecipeRepository) GetRecipe(_ context.Context, name string) (*entities.Recipe, error) {
	filePath := filepath.Join(r.recipesDir, name+".yml")

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("recipe not found: %s", name)
	}

	return r.parser.ParseFile(filePath)
}

// ListRecipes returns all available package recipes
func (r *RecipeRepository) ListRecipes(_ context.Context) ([]*entities.Recipe, error) {
	entries, err := os.ReadDir(r.recipesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read recipes directory: %w", err)
	}

	recipes := make([]*entities.Recipe, 0)
	for _, entry := range entries {
		// Skip non-YAML files
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yml") {
			continue
		}

		filePath := filepath.Join(r.recipesDir, entry.Name())
		def, err := r.parser.ParseFile(filePath)
		if err != nil {
			// Log warning but continue processing other files
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", entry.Name(), err)
			continue
		}

		recipes = append(recipes, def)
	}

	return recipes, nil
}

// GetRecipesByPlatform returns recipes that support a specific platform
func (r *RecipeRepository) GetRecipesByPlatform(ctx context.Context, platform string) ([]*entities.Recipe, error) {
	allDefs, err := r.ListRecipes(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]*entities.Recipe, 0)
	for _, def := range allDefs {
		if _, hasPlatform := def.Download.Platforms[platform]; hasPlatform {
			filtered = append(filtered, def)
		}
	}

	return filtered, nil
}
