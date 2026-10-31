// Package removephoto is the use case for taking a recipe's photo away.
package removephoto

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
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.Recipe, error) {
	recipe, err := i.recipeRepository.FindRecipe(ctx, command.Owner, command.RecipeID)
	if err != nil {
		return nil, err
	}
	if recipe == nil {
		return nil, model.ErrRecipeNotFound
	}
	key := recipe.PhotoKey()
	if key == "" {
		return recipe, nil
	}
	recipe.SetPhotoKey("")
	if err := i.recipeRepository.SaveRecipePhoto(ctx, recipe); err != nil {
		return nil, err
	}
	if err := i.photos.Delete(ctx, key); err != nil {
		slog.WarnContext(ctx, "recipe photo not deleted", "key", key, "error", err)
	}
	return recipe, nil
}
