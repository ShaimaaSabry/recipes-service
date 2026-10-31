// Package applygroceryactions is the use case for changing the grocery list. The app sends
// actions, not finished lines: offline it queues them and previews the result with the same
// rules, then sends the queue when it's back online. Each action carries an ID the app made, so a
// resent action is recognised and not applied twice. Actions are applied in order, each on its
// own; one that's refused (say, it needs a conversion the item doesn't have yet) doesn't stop
// the rest.
package applygroceryactions

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type ActionType string

const (
	// Add adds Quantity of the item in UnitID to its line, converting it into the line's unit.
	Add ActionType = "add"
	// SetQuantity sets the line's amount by hand, in the line's unit.
	SetQuantity ActionType = "setQuantity"
	// SetUnit switches the line to UnitID, one the item is bought in, converting its amount.
	SetUnit      ActionType = "setUnit"
	Check        ActionType = "check"
	Uncheck      ActionType = "uncheck"
	Remove       ActionType = "remove"
	ClearChecked ActionType = "clearChecked"
)

// MaxActions is the most actions one request may carry; a longer queue is sent in parts.
const MaxActions = 200

type Action struct {
	ID       uuid.UUID
	Type     ActionType
	ItemID   uuid.UUID
	UnitID   uuid.UUID
	Quantity float64
}

type Outcome string

const (
	Applied        Outcome = "applied"
	AlreadyApplied Outcome = "alreadyApplied"
	// Rejected actions changed nothing and weren't recorded, so the same action can be sent
	// again once the problem is fixed (e.g. the missing conversion is added).
	Rejected Outcome = "rejected"
)

type Result struct {
	ActionID uuid.UUID
	Outcome  Outcome
	// Err says why an action was rejected. It may be a *model.MissingConversionError.
	Err error
}

type groceryListRepository interface {
	GetUnits(ctx context.Context) ([]model.Unit, error)
	GetLines(ctx context.Context, owner owner.ID) ([]*model.GroceryLine, error)
	// ApplyAction runs apply in one transaction recorded under actionID, and returns false
	// without running it when that action was applied before.
	ApplyAction(ctx context.Context, owner owner.ID, actionID uuid.UUID, apply func(model.GroceryListStore) error) (bool, error)
}

type Interactor struct {
	groceryListRepository groceryListRepository
}

func NewInteractor(groceryListRepository groceryListRepository) *Interactor {
	return &Interactor{groceryListRepository: groceryListRepository}
}

type Command struct {
	Owner   owner.ID
	Actions []Action
}

var ErrTooManyActions = fmt.Errorf("%w: send at most %d actions at a time", apperrors.ErrInvalidInput, MaxActions)

// Execute applies the actions and returns what happened to each, and the list after them. An
// unexpected failure (the database) stops it: the actions before were applied and recorded, so
// the app resends the whole queue and those are skipped.
func (i *Interactor) Execute(ctx context.Context, command Command) ([]Result, model.GroceryList, error) {
	if len(command.Actions) > MaxActions {
		return nil, model.GroceryList{}, ErrTooManyActions
	}
	results := make([]Result, 0, len(command.Actions))
	for _, action := range command.Actions {
		applied, err := i.groceryListRepository.ApplyAction(ctx, command.Owner, action.ID, func(store model.GroceryListStore) error {
			return apply(ctx, store, command.Owner, action)
		})
		switch {
		case err != nil && isRefusal(err):
			results = append(results, Result{ActionID: action.ID, Outcome: Rejected, Err: err})
		case err != nil:
			return nil, model.GroceryList{}, err
		case applied:
			results = append(results, Result{ActionID: action.ID, Outcome: Applied})
		default:
			results = append(results, Result{ActionID: action.ID, Outcome: AlreadyApplied})
		}
	}

	units, err := i.groceryListRepository.GetUnits(ctx)
	if err != nil {
		return nil, model.GroceryList{}, err
	}
	lines, err := i.groceryListRepository.GetLines(ctx, command.Owner)
	if err != nil {
		return nil, model.GroceryList{}, err
	}
	return results, model.GroceryList{Lines: lines, Units: model.NewUnits(units)}, nil
}

// isRefusal tells a business rule saying no from something going wrong.
func isRefusal(err error) bool {
	return errors.Is(err, apperrors.ErrInvalidInput) || errors.Is(err, apperrors.ErrNotFound) || errors.Is(err, apperrors.ErrConflict)
}

var errUnknownAction = fmt.Errorf("%w: unknown action", apperrors.ErrInvalidInput)

func apply(ctx context.Context, store model.GroceryListStore, owner owner.ID, action Action) error {
	switch action.Type {
	case Remove:
		return store.DeleteLine(ctx, owner, action.ItemID)
	case ClearChecked:
		return store.DeleteCheckedLines(ctx, owner)
	case Add, SetQuantity, SetUnit, Check, Uncheck:
	default:
		return errUnknownAction
	}

	item, err := store.FindItem(ctx, owner, action.ItemID)
	if err != nil {
		return err
	}
	if item == nil {
		return model.ErrItemNotFound
	}
	line, err := store.FindLine(ctx, owner, item.ID())
	if err != nil {
		return err
	}
	units, err := store.Units(ctx)
	if err != nil {
		return err
	}

	switch action.Type {
	case Add:
		id, err := uuid.NewV7()
		if err != nil {
			return err
		}
		if line, err = model.AddToList(line, item, units, action.UnitID, action.Quantity, id); err != nil {
			return err
		}
	case Check, Uncheck:
		// Ticking a line someone already removed changes nothing; that's fine.
		if line == nil {
			return nil
		}
		line.SetChecked(action.Type == Check)
	case SetQuantity, SetUnit:
		if line == nil {
			return model.ErrNotOnList
		}
		if action.Type == SetQuantity {
			err = line.SetQuantity(action.Quantity)
		} else {
			err = line.SetUnit(action.UnitID, item, units)
		}
		if err != nil {
			return err
		}
	}
	return store.SaveLine(ctx, line)
}
