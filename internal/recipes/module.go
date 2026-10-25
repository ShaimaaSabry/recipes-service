// Package recipes wires the recipes module: recipes, grocery items, units, categories and the
// grocery list. Other modules and main use only this package, never its layers directly.
package recipes

import (
	"context"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mamienellie/recipes-service/internal/platform/database"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getrecipes"
	"github.com/mamienellie/recipes-service/internal/recipes/entrypoint/api"
	"github.com/mamienellie/recipes-service/internal/recipes/infrastructure/repository/sql"
)

type Module struct {
	db         *gorm.DB
	controller *api.RecipeController
}

func New(db *gorm.DB) *Module {
	recipeRepository := sql.NewRecipeSqlRepository(db)
	return &Module{
		db: db,
		controller: api.NewRecipeController(
			getrecipes.NewInteractor(recipeRepository),
			createrecipe.NewInteractor(recipeRepository),
		),
	}
}

// Migrate brings the module's tables up to date.
func (m *Module) Migrate(ctx context.Context) error {
	return database.Migrate(ctx, m.db, "recipes", sql.Migrations())
}

// RegisterRoutes adds the module's routes to r, which must sit behind httpserver.RequireOwner.
func (m *Module) RegisterRoutes(r gin.IRouter) {
	m.controller.RegisterRoutes(r)
}
