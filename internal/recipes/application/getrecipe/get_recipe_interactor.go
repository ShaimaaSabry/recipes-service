// Package getrecipe is the use case for reading one recipe, optionally scaled to a number of
// people.
package getrecipe

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type recipeRepository interface {
	FindRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Recipe, error)
}

type Interactor struct {
	recipeRepository recipeRepository
}

func NewInteractor(recipeRepository recipeRepository) *Interactor {
	return &Interactor{recipeRepository: recipeRepository}
}

type Query struct {
	Owner owner.ID
	ID    uuid.UUID
	// Servings scales the ingredients to that many people; nil leaves them as the recipe has them.
	Servings *int
}

type Result struct {
	Recipe *model.Recipe
	// Servings and Ingredients are the recipe's, scaled when the query asked for it.
	Servings    int
	Ingredients []model.Ingredient
}

func (i *Interactor) Execute(ctx context.Context, query Query) (Result, error) {
	if query.Servings != nil {
		if err := model.ValidateServings(*query.Servings); err != nil {
			return Result{}, err
		}
	}
	recipe, err := i.recipeRepository.FindRecipe(ctx, query.Owner, query.ID)
	if err != nil {
		return Result{}, err
	}
	if recipe == nil {
		return Result{}, model.ErrRecipeNotFound
	}
	servings := recipe.Servings()
	if query.Servings != nil {
		servings = *query.Servings
	}
	return Result{Recipe: recipe, Servings: servings, Ingredients: recipe.IngredientsFor(servings)}, nil
}
