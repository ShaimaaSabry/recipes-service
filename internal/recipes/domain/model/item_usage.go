package model

import (
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

var (
	ErrItemInUse = fmt.Errorf("%w: this item is on your grocery list or in a recipe. Remove it from there first",
		apperrors.ErrConflict)
	ErrItemUnitInUse = fmt.Errorf("%w: a unit you removed is still used for this item in your grocery list or a recipe. Change those first",
		apperrors.ErrConflict)
)

// ItemUsage is where an item is used: the units recipes measure it in, and the unit of its
// grocery line.
type ItemUsage struct {
	RecipeUnitIDs []uuid.UUID
	ListUnitIDs   []uuid.UUID
}

func (u ItemUsage) InUse() bool { return len(u.RecipeUnitIDs) > 0 || len(u.ListUnitIDs) > 0 }

// Allows checks an edited item still has every unit it's used in: recipes need their units to be
// ones it's cooked in, and its grocery line one it's bought in.
func (u ItemUsage) Allows(item *GroceryItem) error {
	for _, id := range u.RecipeUnitIDs {
		if !slices.Contains(item.RecipeUnitIDs(), id) {
			return ErrItemUnitInUse
		}
	}
	for _, id := range u.ListUnitIDs {
		if !slices.Contains(item.BuyUnitIDs(), id) {
			return ErrItemUnitInUse
		}
	}
	return nil
}
