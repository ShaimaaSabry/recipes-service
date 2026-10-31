package model

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// GroceryListStore is what one grocery list action reads and writes, all inside the action's
// transaction: the action either happens completely or not at all.
type GroceryListStore interface {
	Units(ctx context.Context) (Units, error)
	// FindItem returns nil when the owner has no item with that id.
	FindItem(ctx context.Context, owner owner.ID, id uuid.UUID) (*GroceryItem, error)
	// FindLine returns the item's line, or nil when it isn't on the list.
	FindLine(ctx context.Context, owner owner.ID, itemID uuid.UUID) (*GroceryLine, error)
	SaveLine(ctx context.Context, line *GroceryLine) error
	DeleteLine(ctx context.Context, owner owner.ID, itemID uuid.UUID) error
	DeleteCheckedLines(ctx context.Context, owner owner.ID) error
}
