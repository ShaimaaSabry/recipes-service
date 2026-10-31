// Package createrecipe is the use case for saving a new recipe.
package createrecipe

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type catalogRepository interface {
	GetUnits(ctx context.Context) ([]model.Unit, error)
	ItemsByID(ctx context.Context, owner owner.ID) (map[uuid.UUID]*model.GroceryItem, error)
}

type recipeRepository interface {
	CreateRecipe(ctx context.Context, recipe *model.Recipe) error
}

type Interactor struct {
	catalogRepository catalogRepository
	recipeRepository  recipeRepository
}

func NewInteractor(catalogRepository catalogRepository, recipeRepository recipeRepository) *Interactor {
	return &Interactor{catalogRepository: catalogRepository, recipeRepository: recipeRepository}
}

type Command struct {
	Owner   owner.ID
	Details model.RecipeDetails
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.Recipe, error) {
	units, err := i.catalogRepository.GetUnits(ctx)
	if err != nil {
		return nil, err
	}
	items, err := i.catalogRepository.ItemsByID(ctx, command.Owner)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("create recipe: %w", err)
	}
	recipe, err := model.NewRecipe(id, command.Owner, command.Details, items, model.NewUnits(units))
	if err != nil {
		return nil, err
	}
	if err := i.recipeRepository.CreateRecipe(ctx, recipe); err != nil {
		return nil, err
	}
	return recipe, nil
}
