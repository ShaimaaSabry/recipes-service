package sql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/platform/database"
	"github.com/mamienellie/recipes-service/internal/platform/database/databasetest"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

func newRepository(t *testing.T) *RecipeSqlRepository {
	db := databasetest.New(t)
	require.NoError(t, database.Migrate(context.Background(), db, "recipes", Migrations()))
	return NewRecipeSqlRepository(db)
}

func TestRecipeSqlRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("returns an owner's recipes by name, and no one else's", func(t *testing.T) {
		repo := newRepository(t)
		require.NoError(t, repo.CreateRecipe(ctx, model.Of("r-2", "device-1", "Soup")))
		require.NoError(t, repo.CreateRecipe(ctx, model.Of("r-1", "device-1", "Apple cake")))
		require.NoError(t, repo.CreateRecipe(ctx, model.Of("r-3", "device-2", "Not mine")))

		recipes, err := repo.GetRecipes(ctx, "device-1")

		require.NoError(t, err)
		require.Equal(t, []model.Recipe{*model.Of("r-1", "device-1", "Apple cake"), *model.Of("r-2", "device-1", "Soup")}, recipes)
	})

	t.Run("returns an empty list for an owner with no recipes", func(t *testing.T) {
		recipes, err := newRepository(t).GetRecipes(ctx, "device-1")

		require.NoError(t, err)
		require.Empty(t, recipes)
	})

	t.Run("refuses a second recipe with the same id for the same owner", func(t *testing.T) {
		repo := newRepository(t)
		require.NoError(t, repo.CreateRecipe(ctx, model.Of("r-1", "device-1", "Soup")))

		require.ErrorIs(t, repo.CreateRecipe(ctx, model.Of("r-1", "device-1", "Other")), ErrRecipeExists)
	})

	t.Run("lets two owners use the same id", func(t *testing.T) {
		repo := newRepository(t)
		require.NoError(t, repo.CreateRecipe(ctx, model.Of("item-garlic", "device-1", "Soup")))

		require.NoError(t, repo.CreateRecipe(ctx, model.Of("item-garlic", "device-2", "Soup")))
	})

	t.Run("migrating twice changes nothing", func(t *testing.T) {
		db := databasetest.New(t)
		require.NoError(t, database.Migrate(ctx, db, "recipes", Migrations()))
		require.NoError(t, database.Migrate(ctx, db, "recipes", Migrations()))
	})
}
