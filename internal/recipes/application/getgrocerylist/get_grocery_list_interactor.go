// Package getgrocerylist is the use case for reading the grocery list.
package getgrocerylist

import (
	"context"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type groceryListRepository interface {
	GetUnits(ctx context.Context) ([]model.Unit, error)
	GetLines(ctx context.Context, owner owner.ID) ([]*model.GroceryLine, error)
}

type Interactor struct {
	groceryListRepository groceryListRepository
}

func NewInteractor(groceryListRepository groceryListRepository) *Interactor {
	return &Interactor{groceryListRepository: groceryListRepository}
}

type Query struct {
	Owner owner.ID
}

func (i *Interactor) Execute(ctx context.Context, query Query) (model.GroceryList, error) {
	units, err := i.groceryListRepository.GetUnits(ctx)
	if err != nil {
		return model.GroceryList{}, err
	}
	lines, err := i.groceryListRepository.GetLines(ctx, query.Owner)
	if err != nil {
		return model.GroceryList{}, err
	}
	return model.GroceryList{Lines: lines, Units: model.NewUnits(units)}, nil
}
