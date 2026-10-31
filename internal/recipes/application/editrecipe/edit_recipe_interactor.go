// Package editrecipe is the use case for changing a recipe's name, servings, ingredients and
// steps. Planned meals for it follow, since they read the recipe rather than copying it.
package editrecipe

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type catalogRepository interface {
	GetUnits(ctx context.Context) ([]model.Unit, error)
	ItemsByID(ctx context.Context, owner owner.ID) (map[uuid.UUID]*model.GroceryItem, error)
}

type recipeRepository interface {
	FindRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Recipe, error)
	SaveRecipe(ctx context.Context, recipe *model.Recipe) error
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
	ID      uuid.UUID
	Details model.RecipeDetails
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.Recipe, error) {
	recipe, err := i.recipeRepository.FindRecipe(ctx, command.Owner, command.ID)
	if err != nil {
		return nil, err
	}
	if recipe == nil {
		return nil, model.ErrRecipeNotFound
	}
	units, err := i.catalogRepository.GetUnits(ctx)
	if err != nil {
		return nil, err
	}
	items, err := i.catalogRepository.ItemsByID(ctx, command.Owner)
	if err != nil {
		return nil, err
	}
	if err := recipe.Edit(command.Details, items, model.NewUnits(units)); err != nil {
		return nil, err
	}
	if err := i.recipeRepository.SaveRecipe(ctx, recipe); err != nil {
		return nil, err
	}
	return recipe, nil
}
