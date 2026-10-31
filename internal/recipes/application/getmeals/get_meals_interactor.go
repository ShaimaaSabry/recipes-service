// Package getmeals is the use case for reading the upcoming menu.
package getmeals

import (
	"context"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type mealRepository interface {
	// GetMeals returns the owner's planned meals in the order they were planned.
	GetMeals(ctx context.Context, owner owner.ID) ([]*model.PlannedMeal, error)
}

type Interactor struct {
	mealRepository mealRepository
}

func NewInteractor(mealRepository mealRepository) *Interactor {
	return &Interactor{mealRepository: mealRepository}
}

type Query struct {
	Owner owner.ID
}

func (i *Interactor) Execute(ctx context.Context, query Query) ([]*model.PlannedMeal, error) {
	return i.mealRepository.GetMeals(ctx, query.Owner)
}
