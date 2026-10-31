// Package attachphoto is the use case for setting a recipe's photo to one the app has uploaded.
// The previous photo, if any, is deleted.
package attachphoto

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type recipeRepository interface {
	FindRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Recipe, error)
	SaveRecipePhoto(ctx context.Context, recipe *model.Recipe) error
}

type photoStorage interface {
	// Stat returns found false when nothing is stored under key.
	Stat(ctx context.Context, key string) (photo model.StoredPhoto, found bool, err error)
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
	Owner    owner.ID
	RecipeID uuid.UUID
	PhotoKey string
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.Recipe, error) {
	recipe, err := i.recipeRepository.FindRecipe(ctx, command.Owner, command.RecipeID)
	if err != nil {
		return nil, err
	}
	if recipe == nil {
		return nil, model.ErrRecipeNotFound
	}
	if recipe.PhotoKey() == command.PhotoKey {
		return recipe, nil
	}
	if !model.IsPhotoKeyFor(command.PhotoKey, command.Owner, command.RecipeID) {
		return nil, model.ErrPhotoNotUploaded
	}
	photo, found, err := i.photos.Stat(ctx, command.PhotoKey)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, model.ErrPhotoNotUploaded
	}
	// The upload link can't limit the size or the type, so check them now, and don't keep a
	// photo that fails.
	if err := model.CheckStoredPhoto(photo); err != nil {
		i.deleteQuietly(ctx, command.PhotoKey)
		return nil, err
	}

	previous := recipe.PhotoKey()
	recipe.SetPhotoKey(command.PhotoKey)
	if err := i.recipeRepository.SaveRecipePhoto(ctx, recipe); err != nil {
		return nil, err
	}
	if previous != "" {
		i.deleteQuietly(ctx, previous)
	}
	return recipe, nil
}

// deleteQuietly deletes a photo nothing shows any more; one left behind is only wasted space.
func (i *Interactor) deleteQuietly(ctx context.Context, key string) {
	if err := i.photos.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "recipe photo not deleted", "key", key, "error", err)
	}
}
