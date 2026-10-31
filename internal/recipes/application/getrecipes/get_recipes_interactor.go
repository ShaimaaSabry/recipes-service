// Package getrecipes is the use case for listing an owner's recipes.
package getrecipes

import (
	"context"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type recipeRepository interface {
	// GetRecipes returns the owner's recipes, by name, with their ingredients and steps.
	GetRecipes(ctx context.Context, owner owner.ID) ([]*model.Recipe, error)
}

type Interactor struct {
	recipeRepository recipeRepository
}

func NewInteractor(recipeRepository recipeRepository) *Interactor {
	return &Interactor{recipeRepository: recipeRepository}
}

type Query struct {
	Owner owner.ID
}

func (i *Interactor) Execute(ctx context.Context, query Query) ([]*model.Recipe, error) {
	return i.recipeRepository.GetRecipes(ctx, query.Owner)
}
