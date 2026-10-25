// Package createrecipe is the use case for saving a new recipe.
package createrecipe

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type recipeRepository interface {
	// CreateRecipe fails with apperrors.ErrConflict if the owner already has a recipe with that id.
	CreateRecipe(ctx context.Context, recipe *model.Recipe) error
}

type Interactor struct {
	recipeRepository recipeRepository
}

func NewInteractor(recipeRepository recipeRepository) *Interactor {
	return &Interactor{recipeRepository: recipeRepository}
}

type Command struct {
	Owner owner.ID
	// ID is set by the app, which creates recipes while offline; when empty one is generated.
	ID   string
	Name string
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.Recipe, error) {
	id := command.ID
	if id == "" {
		id = uuid.NewString()
	}
	recipe, err := model.NewRecipe(id, command.Owner, command.Name)
	if err != nil {
		return nil, err
	}
	if err := i.recipeRepository.CreateRecipe(ctx, recipe); err != nil {
		return nil, err
	}
	return recipe, nil
}
