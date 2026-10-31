// Package edititem is the use case for changing a grocery item, starter ones included: its name,
// emoji, category, units and, optionally, its conversions.
package edititem

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type itemRepository interface {
	GetUnits(ctx context.Context) ([]model.Unit, error)
	FindCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Category, error)
	FindItem(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.GroceryItem, error)
	FindItemByName(ctx context.Context, owner owner.ID, name string) (*model.GroceryItem, error)
	SaveItem(ctx context.Context, item *model.GroceryItem) error
}

type Interactor struct {
	itemRepository itemRepository
}

func NewInteractor(itemRepository itemRepository) *Interactor {
	return &Interactor{itemRepository: itemRepository}
}

type Command struct {
	Owner   owner.ID
	ID      uuid.UUID
	Details model.ItemDetails
	// Conversions replace the item's when set. Nil keeps the saved ones, minus any naming a unit
	// the item no longer has, for forms that don't show them.
	Conversions *[]model.Conversion
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.GroceryItem, error) {
	item, err := i.itemRepository.FindItem(ctx, command.Owner, command.ID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, model.ErrItemNotFound
	}
	list, err := i.itemRepository.GetUnits(ctx)
	if err != nil {
		return nil, err
	}
	units := model.NewUnits(list)
	if command.Conversions != nil {
		err = item.Edit(command.Details, *command.Conversions, units)
	} else {
		err = item.EditKeepingConversions(command.Details, units)
	}
	if err != nil {
		return nil, err
	}

	category, err := i.itemRepository.FindCategory(ctx, command.Owner, item.CategoryID())
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, model.ErrNoSuchCategory
	}
	taken, err := i.itemRepository.FindItemByName(ctx, command.Owner, item.Name())
	if err != nil {
		return nil, err
	}
	if taken != nil && taken.ID() != item.ID() {
		return nil, model.NameTaken("grocery item", taken.Name(), taken.Hidden())
	}

	if err := i.itemRepository.SaveItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}
