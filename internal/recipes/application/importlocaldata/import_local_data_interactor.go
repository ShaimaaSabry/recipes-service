// Package importlocaldata is the use case for bringing in what the app kept on the phone before
// accounts existed, so the user loses nothing. The app sends it once, with an ID it made for the
// import; sending it again (after a dropped connection) returns the first import's result. Only
// once the import and the recipes' photos are saved does the app delete its local copy.
package importlocaldata

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type importRepository interface {
	// Import runs plan on the account's current data and saves the result in one transaction
	// recorded under importID. For an import saved before, it returns that one's result and
	// false, without running plan.
	Import(ctx context.Context, owner owner.ID, importID uuid.UUID, plan func(model.ImportState) (*model.ImportPlan, error)) (model.ImportResult, bool, error)
}

type Interactor struct {
	importRepository importRepository
}

func NewInteractor(importRepository importRepository) *Interactor {
	return &Interactor{importRepository: importRepository}
}

type Command struct {
	Owner    owner.ID
	ImportID uuid.UUID
	Data     model.LocalData
}

// Execute returns the import's result, and false when it was imported before.
func (i *Interactor) Execute(ctx context.Context, command Command) (model.ImportResult, bool, error) {
	return i.importRepository.Import(ctx, command.Owner, command.ImportID, func(state model.ImportState) (*model.ImportPlan, error) {
		return model.PlanImport(command.Owner, command.Data, state, func() uuid.UUID { return uuid.Must(uuid.NewV7()) })
	})
}
