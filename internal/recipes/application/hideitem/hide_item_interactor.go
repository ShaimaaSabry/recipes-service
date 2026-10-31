// Package hideitem is the use case for hiding a grocery item from pickers, or showing it again.
// Hiding is how starter items, which can't be deleted, are put away.
package hideitem

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type itemRepository interface {
	FindItem(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.GroceryItem, error)
	SaveItem(ctx context.Context, item *model.GroceryItem) error
}

type Interactor struct {
	itemRepository itemRepository
}

func NewInteractor(itemRepository itemRepository) *Interactor {
	return &Interactor{itemRepository: itemRepository}
}

type Command struct {
	Owner  owner.ID
	ID     uuid.UUID
	Hidden bool
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.GroceryItem, error) {
	item, err := i.itemRepository.FindItem(ctx, command.Owner, command.ID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, model.ErrItemNotFound
	}
	item.SetHidden(command.Hidden)
	if err := i.itemRepository.SaveItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}
