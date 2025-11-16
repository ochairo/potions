// Package repositories defines interfaces for data access layers.
package repositories

import (
	"context"

	"github.com/ochairo/potions/internal/domain/entities"
)

// RecipeRepository defines the interface for accessing package recipes
type RecipeRepository interface {
	// GetRecipe retrieves a package recipe by name
	GetRecipe(ctx context.Context, name string) (*entities.Recipe, error)

	// ListRecipes returns all available package recipes
	ListRecipes(ctx context.Context) ([]*entities.Recipe, error)

	// GetRecipesByPlatform returns recipes that support a specific platform
	GetRecipesByPlatform(ctx context.Context, platform string) ([]*entities.Recipe, error)
}
