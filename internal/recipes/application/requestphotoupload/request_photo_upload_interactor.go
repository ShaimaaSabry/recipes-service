// Package requestphotoupload is the use case for giving the app somewhere to upload a recipe
// photo: a short-lived link straight to the photos bucket, so the image never passes through the
// service. Once uploaded, the app attaches it with attachphoto.
package requestphotoupload

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type recipeRepository interface {
	FindRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Recipe, error)
}

type photoStorage interface {
	PresignUpload(ctx context.Context, key, contentType string) (model.PhotoUpload, error)
}

type Interactor struct {
	recipeRepository recipeRepository
	photos           photoStorage
}

func NewInteractor(recipeRepository recipeRepository, photos photoStorage) *Interactor {
	return &Interactor{recipeRepository: recipeRepository, photos: photos}
}

type Command struct {
	Owner       owner.ID
	RecipeID    uuid.UUID
	ContentType string
}

func (i *Interactor) Execute(ctx context.Context, command Command) (model.PhotoUpload, error) {
	key, err := model.NewPhotoKey(command.Owner, command.RecipeID, command.ContentType)
	if err != nil {
		return model.PhotoUpload{}, err
	}
	recipe, err := i.recipeRepository.FindRecipe(ctx, command.Owner, command.RecipeID)
	if err != nil {
		return model.PhotoUpload{}, err
	}
	if recipe == nil {
		return model.PhotoUpload{}, model.ErrRecipeNotFound
	}
	return i.photos.PresignUpload(ctx, key, command.ContentType)
}
