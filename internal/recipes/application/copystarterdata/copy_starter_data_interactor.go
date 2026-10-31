// Package copystarterdata is the use case for giving an account its own copies of the starter
// categories and grocery items. It runs on every sign-in, so a copy that failed on the first one
// is made on the next; once an account has its copies it does nothing.
package copystarterdata

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type catalogRepository interface {
	GetUnits(ctx context.Context) ([]model.Unit, error)
	StarterDataCopied(ctx context.Context, owner owner.ID) (bool, error)
	// SaveStarterData saves the copies unless the owner already has them, and says whether it did.
	SaveStarterData(ctx context.Context, owner owner.ID, categories []*model.Category, items []*model.GroceryItem) (bool, error)
}

type Interactor struct {
	catalogRepository catalogRepository
	starterData       model.StarterData
}

func NewInteractor(catalogRepository catalogRepository, starterData model.StarterData) *Interactor {
	return &Interactor{catalogRepository: catalogRepository, starterData: starterData}
}

type Command struct {
	Owner owner.ID
}

func (i *Interactor) Execute(ctx context.Context, command Command) error {
	copied, err := i.catalogRepository.StarterDataCopied(ctx, command.Owner)
	if err != nil || copied {
		return err
	}
	units, err := i.catalogRepository.GetUnits(ctx)
	if err != nil {
		return err
	}
	categories, items, err := i.starterData.CopyFor(command.Owner, model.NewUnits(units), uuid.NewV7)
	if err != nil {
		return fmt.Errorf("copy starter data: %w", err)
	}
	_, err = i.catalogRepository.SaveStarterData(ctx, command.Owner, categories, items)
	return err
}
