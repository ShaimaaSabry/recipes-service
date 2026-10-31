// Package planmeal is the use case for putting a recipe on the upcoming menu, or changing a
// planned meal's recipe or number of people. The app makes the meal's ID, possibly offline, so
// planning the same meal again (a resent request) just saves it again.
package planmeal

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type mealRepository interface {
	FindRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Recipe, error)
	// SaveMeal creates or updates the meal, failing with model.ErrMealIDTaken when the id is
	// another account's.
	SaveMeal(ctx context.Context, meal *model.PlannedMeal) (*model.PlannedMeal, error)
}

type Interactor struct {
	mealRepository mealRepository
}

func NewInteractor(mealRepository mealRepository) *Interactor {
	return &Interactor{mealRepository: mealRepository}
}

type Command struct {
	Owner    owner.ID
	ID       uuid.UUID
	RecipeID uuid.UUID
	Servings int
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.PlannedMeal, error) {
	meal, err := model.NewPlannedMeal(command.ID, command.Owner, command.RecipeID, command.Servings)
	if err != nil {
		return nil, err
	}
	recipe, err := i.mealRepository.FindRecipe(ctx, command.Owner, command.RecipeID)
	if err != nil {
		return nil, err
	}
	if recipe == nil {
		return nil, model.ErrNoSuchRecipe
	}
	return i.mealRepository.SaveMeal(ctx, meal)
}
