// Package removemeal is the use case for taking a meal off the upcoming menu. The grocery list is
// left as it is. Removing a meal that isn't there (already removed, e.g. by a resent request)
// succeeds, so the app's queue can always be replayed.
package removemeal

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type mealRepository interface {
	DeleteMeal(ctx context.Context, owner owner.ID, id uuid.UUID) error
}

type Interactor struct {
	mealRepository mealRepository
}

func NewInteractor(mealRepository mealRepository) *Interactor {
	return &Interactor{mealRepository: mealRepository}
}

type Command struct {
	Owner owner.ID
	ID    uuid.UUID
}

func (i *Interactor) Execute(ctx context.Context, command Command) error {
	return i.mealRepository.DeleteMeal(ctx, command.Owner, command.ID)
}
