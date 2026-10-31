package sql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// LocalImportSqlRepository saves imports of the data the app kept on the phone.
type LocalImportSqlRepository struct {
	db *gorm.DB
}

func NewLocalImportSqlRepository(db *gorm.DB) *LocalImportSqlRepository {
	return &LocalImportSqlRepository{db: db}
}

// errAlreadyImported rolls back an import whose ID was imported before.
var errAlreadyImported = errors.New("already imported")

// Import runs plan on what the account has now, and saves what it returns, all in one
// transaction recorded under importID. When that import was saved before, it returns its result
// and false without running plan. The owner's grocery list actions wait for it, so the list it
// reads stays current.
func (r *LocalImportSqlRepository) Import(ctx context.Context, owner owner.ID, importID uuid.UUID, plan func(model.ImportState) (*model.ImportPlan, error)) (model.ImportResult, bool, error) {
	var result model.ImportResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		recorded := tx.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&localImportDBO{OwnerID: string(owner), ImportID: importID, ImportedAt: time.Now()})
		if recorded.Error != nil {
			return fmt.Errorf("record import: %w", recorded.Error)
		}
		if recorded.RowsAffected == 0 {
			return errAlreadyImported
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "grocery-list:"+string(owner)).Error; err != nil {
			return fmt.Errorf("lock grocery list: %w", err)
		}
		state, err := importState(ctx, tx, owner)
		if err != nil {
			return err
		}
		planned, err := plan(state)
		if err != nil {
			return err
		}
		if err := saveImport(ctx, tx, planned); err != nil {
			return err
		}
		encoded, err := json.Marshal(planned.Result)
		if err != nil {
			return fmt.Errorf("encode import result: %w", err)
		}
		err = tx.Model(&localImportDBO{}).Where("owner_id = ? AND import_id = ?", string(owner), importID).
			Update("result", string(encoded)).Error
		if err != nil {
			return fmt.Errorf("save import result: %w", err)
		}
		result = planned.Result
		return nil
	})
	switch {
	case err == nil:
		return result, true, nil
	case errors.Is(err, errAlreadyImported):
		stored, err := r.storedResult(ctx, owner, importID)
		return stored, false, err
	default:
		return model.ImportResult{}, false, err
	}
}

func (r *LocalImportSqlRepository) storedResult(ctx context.Context, owner owner.ID, importID uuid.UUID) (model.ImportResult, error) {
	var dbo localImportDBO
	err := r.db.WithContext(ctx).Where("owner_id = ? AND import_id = ?", string(owner), importID).Take(&dbo).Error
	if err != nil {
		return model.ImportResult{}, fmt.Errorf("get import result: %w", err)
	}
	var result model.ImportResult
	if dbo.Result == nil {
		return result, errors.New("get import result: the import has no result")
	}
	if err := json.Unmarshal([]byte(*dbo.Result), &result); err != nil {
		return result, fmt.Errorf("decode import result: %w", err)
	}
	return result, nil
}

func importState(ctx context.Context, tx *gorm.DB, owner owner.ID) (model.ImportState, error) {
	catalog := NewCatalogSqlRepository(tx)
	units, err := catalog.GetUnits(ctx)
	if err != nil {
		return model.ImportState{}, err
	}
	categories, err := catalog.GetCategories(ctx, owner)
	if err != nil {
		return model.ImportState{}, err
	}
	items, err := catalog.GetItems(ctx, owner)
	if err != nil {
		return model.ImportState{}, err
	}
	lines, err := NewGroceryListSqlRepository(tx).GetLines(ctx, owner)
	if err != nil {
		return model.ImportState{}, err
	}
	usage, err := itemUsages(ctx, tx, owner)
	if err != nil {
		return model.ImportState{}, err
	}
	return model.ImportState{
		Units: model.NewUnits(units), Categories: categories, Items: items, Usage: usage, Lines: lines,
	}, nil
}

// itemUsages says where each of the owner's items is used.
func itemUsages(ctx context.Context, tx *gorm.DB, owner owner.ID) (map[uuid.UUID]model.ItemUsage, error) {
	type use struct {
		ItemID uuid.UUID
		UnitID uuid.UUID
	}
	var inRecipes, onList []use
	err := tx.WithContext(ctx).Model(&ingredientDBO{}).Distinct("item_id", "unit_id").
		Where("owner_id = ?", string(owner)).Scan(&inRecipes).Error
	if err != nil {
		return nil, fmt.Errorf("item usage: %w", err)
	}
	err = tx.WithContext(ctx).Model(&groceryLineDBO{}).Select("item_id", "unit_id").
		Where("owner_id = ?", string(owner)).Scan(&onList).Error
	if err != nil {
		return nil, fmt.Errorf("item usage: %w", err)
	}
	usage := map[uuid.UUID]model.ItemUsage{}
	for _, u := range inRecipes {
		entry := usage[u.ItemID]
		entry.RecipeUnitIDs = append(entry.RecipeUnitIDs, u.UnitID)
		usage[u.ItemID] = entry
	}
	for _, u := range onList {
		entry := usage[u.ItemID]
		entry.ListUnitIDs = append(entry.ListUnitIDs, u.UnitID)
		usage[u.ItemID] = entry
	}
	return usage, nil
}

// saveImport saves the plan. Renamed records go before new ones, so a name one gives up is free
// for the other; categories go before the items in them.
func saveImport(ctx context.Context, tx *gorm.DB, plan *model.ImportPlan) error {
	catalog := NewCatalogSqlRepository(tx)
	for _, category := range plan.ChangedCategories {
		if err := catalog.SaveCategory(ctx, category); err != nil {
			return err
		}
	}
	for _, category := range plan.NewCategories {
		if err := catalog.CreateCategory(ctx, category); err != nil {
			return err
		}
	}
	for _, item := range plan.ChangedItems {
		if err := catalog.SaveItem(ctx, item); err != nil {
			return err
		}
	}
	for _, item := range plan.NewItems {
		if err := catalog.CreateItem(ctx, item); err != nil {
			return err
		}
	}
	recipes := NewRecipeSqlRepository(tx)
	for _, recipe := range plan.Recipes {
		if err := recipes.CreateRecipe(ctx, recipe); err != nil {
			return err
		}
	}
	for _, meal := range plan.Meals {
		dbo := plannedMealDBO{ID: meal.ID(), OwnerID: string(meal.Owner()), RecipeID: meal.RecipeID(), Servings: meal.Servings()}
		if err := tx.WithContext(ctx).Create(&dbo).Error; err != nil {
			return fmt.Errorf("plan meal: %w", err)
		}
	}
	list := groceryListStore{tx: tx}
	for _, line := range plan.Lines {
		if err := list.SaveLine(ctx, line); err != nil {
			return err
		}
	}
	return nil
}
