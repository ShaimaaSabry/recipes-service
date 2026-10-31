// Package createitem is the use case for adding a grocery item of the owner's own.
package createitem

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type itemRepository interface {
	GetUnits(ctx context.Context) ([]model.Unit, error)
	FindCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Category, error)
	FindItemByName(ctx context.Context, owner owner.ID, name string) (*model.GroceryItem, error)
	CreateItem(ctx context.Context, item *model.GroceryItem) error
}

type Interactor struct {
	itemRepository itemRepository
}

func NewInteractor(itemRepository itemRepository) *Interactor {
	return &Interactor{itemRepository: itemRepository}
}

type Command struct {
	Owner       owner.ID
	Details     model.ItemDetails
	Conversions []model.Conversion
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.GroceryItem, error) {
	units, err := i.itemRepository.GetUnits(ctx)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("create item: %w", err)
	}
	item, err := model.NewGroceryItem(id, command.Owner, command.Details, command.Conversions, model.NewUnits(units))
	if err != nil {
		return nil, err
	}
	if err := checkCategoryAndName(ctx, i.itemRepository, item); err != nil {
		return nil, err
	}
	if err := i.itemRepository.CreateItem(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

type lookups interface {
	FindCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Category, error)
	FindItemByName(ctx context.Context, owner owner.ID, name string) (*model.GroceryItem, error)
}

// checkCategoryAndName checks the item's category is the owner's and no other item has its name.
func checkCategoryAndName(ctx context.Context, repository lookups, item *model.GroceryItem) error {
	category, err := repository.FindCategory(ctx, item.Owner(), item.CategoryID())
	if err != nil {
		return err
	}
	if category == nil {
		return model.ErrNoSuchCategory
	}
	taken, err := repository.FindItemByName(ctx, item.Owner(), item.Name())
	if err != nil {
		return err
	}
	if taken != nil && taken.ID() != item.ID() {
		return model.NameTaken("grocery item", taken.Name(), taken.Hidden())
	}
	return nil
}
