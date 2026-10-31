package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

var (
	// ErrMealIDTaken is a meal ID another account already uses. The app makes meal IDs as random
	// UUIDs, so this only happens to a client that reuses them.
	ErrMealIDTaken = fmt.Errorf("%w: another meal already has this id", apperrors.ErrConflict)
	// ErrNoSuchRecipe is a meal for a recipe the owner doesn't have, e.g. one deleted on another
	// phone before this meal synced.
	ErrNoSuchRecipe  = fmt.Errorf("%w: please choose one of your recipes", apperrors.ErrInvalidInput)
	ErrInvalidMealID = fmt.Errorf("%w: a meal's id must be a UUID", apperrors.ErrInvalidInput)
)

// PlannedMeal is a recipe on the upcoming menu, for a number of people. Unlike everything else,
// its ID comes from the app, which plans meals offline.
type PlannedMeal struct {
	id        uuid.UUID
	owner     owner.ID
	recipeID  uuid.UUID
	servings  int
	createdAt time.Time
}

func NewPlannedMeal(id uuid.UUID, owner owner.ID, recipeID uuid.UUID, servings int) (*PlannedMeal, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidMealID
	}
	if err := ValidateServings(servings); err != nil {
		return nil, err
	}
	return &PlannedMeal{id: id, owner: owner, recipeID: recipeID, servings: servings}, nil
}

// PlannedMealOf rebuilds a meal that was already validated, e.g. one read back from the database.
func PlannedMealOf(id uuid.UUID, owner owner.ID, recipeID uuid.UUID, servings int, createdAt time.Time) *PlannedMeal {
	return &PlannedMeal{id: id, owner: owner, recipeID: recipeID, servings: servings, createdAt: createdAt}
}

func (m PlannedMeal) ID() uuid.UUID        { return m.id }
func (m PlannedMeal) Owner() owner.ID      { return m.owner }
func (m PlannedMeal) RecipeID() uuid.UUID  { return m.recipeID }
func (m PlannedMeal) Servings() int        { return m.servings }
func (m PlannedMeal) CreatedAt() time.Time { return m.createdAt }
