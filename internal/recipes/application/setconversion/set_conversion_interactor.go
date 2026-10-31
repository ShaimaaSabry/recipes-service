// Package setconversion is the use case for adding one conversion to a grocery item, or replacing
// the one for the same two units: the app's answer when it asks "1 box = ? piece".
package setconversion

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type itemRepository interface {
	GetUnits(ctx context.Context) ([]model.Unit, error)
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
	Owner      owner.ID
	ItemID     uuid.UUID
	Conversion model.Conversion
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.GroceryItem, error) {
	item, err := i.itemRepository.FindItem(ctx, command.Owner, command.ItemID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, model.ErrItemNotFound
	}
	units, err := i.itemRepository.GetUnits(ctx)
	if err != nil {
		return nil, err
	}
	if err := item.SetConversion(command.Conversion, model.NewUnits(units)); err != nil {
		return nil, err
	}
	if err := i.itemRepository.SaveItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}
