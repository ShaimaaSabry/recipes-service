package createrecipe

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

type fakeRepository struct {
	saved []*model.Recipe
	err   error
}

func (f *fakeRepository) CreateRecipe(_ context.Context, recipe *model.Recipe) error {
	if f.err != nil {
		return f.err
	}
	f.saved = append(f.saved, recipe)
	return nil
}

func TestExecute(t *testing.T) {
	ctx := context.Background()

	t.Run("saves the recipe with the id the app chose", func(t *testing.T) {
		repo := &fakeRepository{}

		recipe, err := NewInteractor(repo).Execute(ctx, Command{Owner: "device-1", ID: "recipe-1", Name: " Soup "})

		require.NoError(t, err)
		require.Equal(t, []*model.Recipe{model.Of("recipe-1", "device-1", "Soup")}, repo.saved)
		require.Equal(t, repo.saved[0], recipe)
	})

	t.Run("generates an id when none is given", func(t *testing.T) {
		repo := &fakeRepository{}

		recipe, err := NewInteractor(repo).Execute(ctx, Command{Owner: "device-1", Name: "Soup"})

		require.NoError(t, err)
		_, parseErr := uuid.Parse(recipe.ID())
		require.NoError(t, parseErr)
	})

	t.Run("saves nothing when the recipe is invalid", func(t *testing.T) {
		repo := &fakeRepository{}

		_, err := NewInteractor(repo).Execute(ctx, Command{Owner: "device-1", Name: " "})

		require.ErrorIs(t, err, model.ErrInvalidRecipeName)
		require.Empty(t, repo.saved)
	})

	t.Run("passes on a repository error", func(t *testing.T) {
		boom := errors.New("boom")

		_, err := NewInteractor(&fakeRepository{err: boom}).Execute(ctx, Command{Owner: "device-1", Name: "Soup"})

		require.ErrorIs(t, err, boom)
	})
}
