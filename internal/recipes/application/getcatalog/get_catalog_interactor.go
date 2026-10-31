// Package getcatalog is the use case for reading everything the app needs to describe groceries:
// the units, and the owner's categories and grocery items with their units and conversions. The
// app keeps a copy, so it can add to the grocery list offline.
package getcatalog

import (
	"context"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type catalogRepository interface {
	// GetUnits returns every unit, in picker order.
	GetUnits(ctx context.Context) ([]model.Unit, error)
	// GetCategories returns the owner's categories, hidden ones included, by name.
	GetCategories(ctx context.Context, owner owner.ID) ([]*model.Category, error)
	// GetItems returns the owner's grocery items, hidden ones included, by name.
	GetItems(ctx context.Context, owner owner.ID) ([]*model.GroceryItem, error)
}

type Interactor struct {
	catalogRepository catalogRepository
}

func NewInteractor(catalogRepository catalogRepository) *Interactor {
	return &Interactor{catalogRepository: catalogRepository}
}

type Query struct {
	Owner owner.ID
}

type Catalog struct {
	Units      []model.Unit
	Categories []*model.Category
	Items      []*model.GroceryItem
}

func (i *Interactor) Execute(ctx context.Context, query Query) (Catalog, error) {
	units, err := i.catalogRepository.GetUnits(ctx)
	if err != nil {
		return Catalog{}, err
	}
	categories, err := i.catalogRepository.GetCategories(ctx, query.Owner)
	if err != nil {
		return Catalog{}, err
	}
	items, err := i.catalogRepository.GetItems(ctx, query.Owner)
	if err != nil {
		return Catalog{}, err
	}
	return Catalog{Units: units, Categories: categories, Items: items}, nil
}
