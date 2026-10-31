package sql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// AppliedActionsKept is how long an action's ID is remembered, so a resend within it is ignored.
// The app's queue never holds an action that long.
const AppliedActionsKept = 30 * 24 * time.Hour

// GroceryListSqlRepository stores the grocery list, and which of its actions were applied.
type GroceryListSqlRepository struct {
	db *gorm.DB
}

func NewGroceryListSqlRepository(db *gorm.DB) *GroceryListSqlRepository {
	return &GroceryListSqlRepository{db: db}
}

// GetLines returns the owner's grocery list, in the order lines were first added.
func (r *GroceryListSqlRepository) GetLines(ctx context.Context, owner owner.ID) ([]*model.GroceryLine, error) {
	var dbos []groceryLineDBO
	if err := r.db.WithContext(ctx).Where("owner_id = ?", string(owner)).Order("created_at, id").Find(&dbos).Error; err != nil {
		return nil, fmt.Errorf("get grocery list: %w", err)
	}
	lines := make([]*model.GroceryLine, 0, len(dbos))
	for _, d := range dbos {
		lines = append(lines, toGroceryLine(d))
	}
	return lines, nil
}

func (r *GroceryListSqlRepository) GetUnits(ctx context.Context) ([]model.Unit, error) {
	return NewCatalogSqlRepository(r.db).GetUnits(ctx)
}

// errAlreadyApplied rolls back an action whose ID was applied before.
var errAlreadyApplied = errors.New("action already applied")

// ApplyAction runs apply in one transaction, recorded under actionID. It returns false, without
// running apply, when that action was applied before. If apply fails, nothing is saved, not
// even the action's ID, so the app can send it again once the problem is fixed. One owner's
// actions run one at a time, so two phones syncing at once can't both create the item's line.
func (r *GroceryListSqlRepository) ApplyAction(ctx context.Context, owner owner.ID, actionID uuid.UUID, apply func(model.GroceryListStore) error) (bool, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "grocery-list:"+string(owner)).Error; err != nil {
			return fmt.Errorf("lock grocery list: %w", err)
		}
		recorded := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&appliedActionDBO{OwnerID: string(owner), ActionID: actionID, AppliedAt: time.Now()})
		if recorded.Error != nil {
			return fmt.Errorf("record action: %w", recorded.Error)
		}
		if recorded.RowsAffected == 0 {
			return errAlreadyApplied
		}
		err := tx.Where("owner_id = ? AND applied_at < ?", string(owner), time.Now().Add(-AppliedActionsKept)).
			Delete(&appliedActionDBO{}).Error
		if err != nil {
			return fmt.Errorf("clear old actions: %w", err)
		}
		return apply(groceryListStore{tx: tx})
	})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, errAlreadyApplied):
		return false, nil
	default:
		return false, err
	}
}

// groceryListStore is model.GroceryListStore inside one action's transaction.
type groceryListStore struct {
	tx *gorm.DB
}

func (s groceryListStore) Units(ctx context.Context) (model.Units, error) {
	units, err := NewCatalogSqlRepository(s.tx).GetUnits(ctx)
	return model.NewUnits(units), err
}

func (s groceryListStore) FindItem(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.GroceryItem, error) {
	return NewCatalogSqlRepository(s.tx).FindItem(ctx, owner, id)
}

func (s groceryListStore) FindLine(ctx context.Context, owner owner.ID, itemID uuid.UUID) (*model.GroceryLine, error) {
	var dbos []groceryLineDBO
	err := s.tx.WithContext(ctx).Where("owner_id = ? AND item_id = ?", string(owner), itemID).Limit(1).Find(&dbos).Error
	if err != nil {
		return nil, fmt.Errorf("find grocery line: %w", err)
	}
	if len(dbos) == 0 {
		return nil, nil
	}
	return toGroceryLine(dbos[0]), nil
}

func (s groceryListStore) SaveLine(ctx context.Context, line *model.GroceryLine) error {
	dbo := groceryLineDBO{
		ID: line.ID(), OwnerID: string(line.Owner()), ItemID: line.ItemID(), UnitID: line.UnitID(),
		Quantity: line.Quantity(), Checked: line.Checked(), UpdatedAt: time.Now(),
	}
	err := s.tx.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"unit_id", "quantity", "checked", "updated_at"}),
	}).Create(&dbo).Error
	if err != nil {
		return fmt.Errorf("save grocery line: %w", err)
	}
	return nil
}

func (s groceryListStore) DeleteLine(ctx context.Context, owner owner.ID, itemID uuid.UUID) error {
	if err := s.tx.WithContext(ctx).Where("owner_id = ? AND item_id = ?", string(owner), itemID).Delete(&groceryLineDBO{}).Error; err != nil {
		return fmt.Errorf("remove grocery line: %w", err)
	}
	return nil
}

func (s groceryListStore) DeleteCheckedLines(ctx context.Context, owner owner.ID) error {
	if err := s.tx.WithContext(ctx).Where("owner_id = ? AND checked", string(owner)).Delete(&groceryLineDBO{}).Error; err != nil {
		return fmt.Errorf("clear checked lines: %w", err)
	}
	return nil
}

func toGroceryLine(d groceryLineDBO) *model.GroceryLine {
	return model.GroceryLineOf(d.ID, owner.ID(d.OwnerID), d.ItemID, d.UnitID, d.Quantity, d.Checked)
}
