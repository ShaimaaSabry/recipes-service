// Package deleterecipe is the use case for deleting a recipe, with its planned meals and photo.
// The grocery list keeps what was added from it: those are ordinary amounts once on the list.
package deleterecipe

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type recipeRepository interface {
	FindRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Recipe, error)
	DeleteRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) error
}

type photoStorage interface {
	Delete(ctx context.Context, keys ...string) error
}

type Interactor struct {
	recipeRepository recipeRepository
	photos           photoStorage
}

func NewInteractor(recipeRepository recipeRepository, photos photoStorage) *Interactor {
	return &Interactor{recipeRepository: recipeRepository, photos: photos}
}

type Command struct {
	Owner owner.ID
	ID    uuid.UUID
}

func (i *Interactor) Execute(ctx context.Context, command Command) error {
	recipe, err := i.recipeRepository.FindRecipe(ctx, command.Owner, command.ID)
	if err != nil {
		return err
	}
	if recipe == nil {
		return model.ErrRecipeNotFound
	}
	if err := i.recipeRepository.DeleteRecipe(ctx, command.Owner, command.ID); err != nil {
		return err
	}
	// The recipe is gone either way; a photo left behind is only wasted space.
	if recipe.PhotoKey() != "" {
		if err := i.photos.Delete(ctx, recipe.PhotoKey()); err != nil {
			slog.WarnContext(ctx, "recipe photo not deleted", "key", recipe.PhotoKey(), "error", err)
		}
	}
	return nil
}
