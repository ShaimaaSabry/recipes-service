// Package deleteitem is the use case for deleting one of the owner's own grocery items. Starter
// items can only be hidden, and an item can't go while a recipe or the grocery list uses it.
package deleteitem

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type itemRepository interface {
	FindItem(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.GroceryItem, error)
	ItemUsage(ctx context.Context, owner owner.ID, itemID uuid.UUID) (model.ItemUsage, error)
	DeleteItem(ctx context.Context, owner owner.ID, id uuid.UUID) error
}

type Interactor struct {
	itemRepository itemRepository
}

func NewInteractor(itemRepository itemRepository) *Interactor {
	return &Interactor{itemRepository: itemRepository}
}

type Command struct {
	Owner owner.ID
	ID    uuid.UUID
}

func (i *Interactor) Execute(ctx context.Context, command Command) error {
	item, err := i.itemRepository.FindItem(ctx, command.Owner, command.ID)
	if err != nil {
		return err
	}
	if item == nil {
		return model.ErrItemNotFound
	}
	if err := item.CanDelete(); err != nil {
		return err
	}
	usage, err := i.itemRepository.ItemUsage(ctx, command.Owner, item.ID())
	if err != nil {
		return err
	}
	if usage.InUse() {
		return model.ErrItemInUse
	}
	return i.itemRepository.DeleteItem(ctx, command.Owner, command.ID)
}
