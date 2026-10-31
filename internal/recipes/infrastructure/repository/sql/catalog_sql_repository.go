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
	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// ErrCategoryInUse is the database refusing to delete a category that items still use, when the
// use case's own check lost a race.
var ErrCategoryInUse = fmt.Errorf("%w: grocery items use this category. Move them to another category first", apperrors.ErrConflict)

// CatalogSqlRepository stores units, and each account's grocery categories and items.
type CatalogSqlRepository struct {
	db *gorm.DB
}

// NewCatalogSqlRepository needs a db opened with TranslateError (database.Open does), so key
// violations come back as gorm.ErrDuplicatedKey and gorm.ErrForeignKeyViolated.
func NewCatalogSqlRepository(db *gorm.DB) *CatalogSqlRepository {
	return &CatalogSqlRepository{db: db}
}

func (r *CatalogSqlRepository) GetUnits(ctx context.Context) ([]model.Unit, error) {
	var dbos []unitDBO
	if err := r.db.WithContext(ctx).Order("position").Find(&dbos).Error; err != nil {
		return nil, fmt.Errorf("get units: %w", err)
	}
	units := make([]model.Unit, 0, len(dbos))
	for _, d := range dbos {
		var base uuid.UUID
		var size float64
		if d.BaseUnitID != nil && d.BaseSize != nil {
			base, size = *d.BaseUnitID, *d.BaseSize
		}
		units = append(units, model.UnitOf(d.ID, d.Code, d.Name, base, size, d.SoldWhole))
	}
	return units, nil
}

// ---------------------------------------------------------------------------
// Categories
// ---------------------------------------------------------------------------

// GetCategories returns the owner's categories by name.
func (r *CatalogSqlRepository) GetCategories(ctx context.Context, owner owner.ID) ([]*model.Category, error) {
	var dbos []categoryDBO
	err := r.db.WithContext(ctx).Where("owner_id = ?", string(owner)).Order("lower(name), id").Find(&dbos).Error
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}
	categories := make([]*model.Category, 0, len(dbos))
	for _, d := range dbos {
		categories = append(categories, toCategory(d))
	}
	return categories, nil
}

// FindCategory returns nil when the owner has no category with that id.
func (r *CatalogSqlRepository) FindCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Category, error) {
	return r.findCategory(ctx, "owner_id = ? AND id = ?", string(owner), id)
}

// FindCategoryByName returns the owner's category with that name, ignoring case, or nil.
func (r *CatalogSqlRepository) FindCategoryByName(ctx context.Context, owner owner.ID, name string) (*model.Category, error) {
	return r.findCategory(ctx, "owner_id = ? AND lower(name) = lower(?)", string(owner), name)
}

func (r *CatalogSqlRepository) findCategory(ctx context.Context, where string, args ...any) (*model.Category, error) {
	var dbos []categoryDBO
	if err := r.db.WithContext(ctx).Where(where, args...).Limit(1).Find(&dbos).Error; err != nil {
		return nil, fmt.Errorf("find category: %w", err)
	}
	if len(dbos) == 0 {
		return nil, nil
	}
	return toCategory(dbos[0]), nil
}

func (r *CatalogSqlRepository) CreateCategory(ctx context.Context, category *model.Category) error {
	return categoryError(r.db.WithContext(ctx).Create(toCategoryDBO(category)).Error, category, "create")
}

// SaveCategory saves an existing category's name, emoji and hidden flag.
func (r *CatalogSqlRepository) SaveCategory(ctx context.Context, category *model.Category) error {
	err := r.db.WithContext(ctx).Model(&categoryDBO{}).
		Where("owner_id = ? AND id = ?", string(category.Owner()), category.ID()).
		Updates(map[string]any{
			"name": category.Name(), "emoji": category.Emoji(), "hidden": category.Hidden(), "updated_at": time.Now(),
		}).Error
	return categoryError(err, category, "save")
}

func categoryError(err error, category *model.Category, action string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return model.NameTaken("category", category.Name(), false)
	default:
		return fmt.Errorf("%s category: %w", action, err)
	}
}

// CountItemsInCategory counts the items, hidden ones included, in one of the owner's categories.
func (r *CatalogSqlRepository) CountItemsInCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&itemDBO{}).Where("owner_id = ? AND category_id = ?", string(owner), id).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count category items: %w", err)
	}
	return count, nil
}

// DeleteCategory fails with ErrCategoryInUse while items use the category.
func (r *CatalogSqlRepository) DeleteCategory(ctx context.Context, owner owner.ID, id uuid.UUID) error {
	err := r.db.WithContext(ctx).Where("owner_id = ? AND id = ?", string(owner), id).Delete(&categoryDBO{}).Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return ErrCategoryInUse
	default:
		return fmt.Errorf("delete category: %w", err)
	}
}

// ---------------------------------------------------------------------------
// Grocery items
// ---------------------------------------------------------------------------

// GetItems returns the owner's items by name, each with its units and conversions.
func (r *CatalogSqlRepository) GetItems(ctx context.Context, owner owner.ID) ([]*model.GroceryItem, error) {
	return r.findItems(ctx, r.db.WithContext(ctx).Where("owner_id = ?", string(owner)).Order("lower(name), id"))
}

// FindItem returns nil when the owner has no item with that id.
func (r *CatalogSqlRepository) FindItem(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.GroceryItem, error) {
	return r.findItem(ctx, "owner_id = ? AND id = ?", string(owner), id)
}

// FindItemByName returns the owner's item with that name, ignoring case, or nil.
func (r *CatalogSqlRepository) FindItemByName(ctx context.Context, owner owner.ID, name string) (*model.GroceryItem, error) {
	return r.findItem(ctx, "owner_id = ? AND lower(name) = lower(?)", string(owner), name)
}

func (r *CatalogSqlRepository) findItem(ctx context.Context, where string, args ...any) (*model.GroceryItem, error) {
	items, err := r.findItems(ctx, r.db.WithContext(ctx).Where(where, args...).Limit(1))
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return items[0], nil
}

// findItems loads the items query selects, then all their units and conversions in two more
// queries.
func (r *CatalogSqlRepository) findItems(ctx context.Context, query *gorm.DB) ([]*model.GroceryItem, error) {
	var dbos []itemDBO
	if err := query.Find(&dbos).Error; err != nil {
		return nil, fmt.Errorf("get items: %w", err)
	}
	if len(dbos) == 0 {
		return []*model.GroceryItem{}, nil
	}
	ids := make([]uuid.UUID, 0, len(dbos))
	for _, d := range dbos {
		ids = append(ids, d.ID)
	}

	var unitRows []itemUnitDBO
	if err := r.db.WithContext(ctx).Where("item_id IN ?", ids).Order("item_id, purpose, position").Find(&unitRows).Error; err != nil {
		return nil, fmt.Errorf("get item units: %w", err)
	}
	var conversionRows []itemConversionDBO
	if err := r.db.WithContext(ctx).Where("item_id IN ?", ids).Order("item_id, position").Find(&conversionRows).Error; err != nil {
		return nil, fmt.Errorf("get item conversions: %w", err)
	}

	buy, cook := map[uuid.UUID][]uuid.UUID{}, map[uuid.UUID][]uuid.UUID{}
	for _, u := range unitRows {
		if u.Purpose == purposeBuy {
			buy[u.ItemID] = append(buy[u.ItemID], u.UnitID)
		} else {
			cook[u.ItemID] = append(cook[u.ItemID], u.UnitID)
		}
	}
	conversions := map[uuid.UUID][]model.Conversion{}
	for _, c := range conversionRows {
		conversions[c.ItemID] = append(conversions[c.ItemID], model.Conversion{From: c.FromUnitID, To: c.ToUnitID, Factor: c.Factor})
	}

	items := make([]*model.GroceryItem, 0, len(dbos))
	for _, d := range dbos {
		details := model.ItemDetails{
			Name: d.Name, Emoji: d.Emoji, CategoryID: d.CategoryID,
			BuyUnitIDs: buy[d.ID], CookUnitIDs: orEmpty(cook[d.ID]),
		}
		var systemKey string
		if d.SystemKey != nil {
			systemKey = *d.SystemKey
		}
		items = append(items, model.GroceryItemOf(d.ID, owner.ID(d.OwnerID), details, orEmpty(conversions[d.ID]),
			model.Source(d.Source), systemKey, d.Hidden))
	}
	return items, nil
}

func (r *CatalogSqlRepository) CreateItem(ctx context.Context, item *model.GroceryItem) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(toItemDBO(item)).Error; err != nil {
			return err
		}
		return createItemParts(tx, item)
	})
	return itemError(err, item, "create")
}

// SaveItem saves an existing item: its details, hidden flag, units and conversions.
func (r *CatalogSqlRepository) SaveItem(ctx context.Context, item *model.GroceryItem) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&itemDBO{}).
			Where("owner_id = ? AND id = ?", string(item.Owner()), item.ID()).
			Updates(map[string]any{
				"name": item.Name(), "emoji": item.Emoji(), "category_id": item.CategoryID(),
				"hidden": item.Hidden(), "updated_at": time.Now(),
			}).Error
		if err != nil {
			return err
		}
		if err := tx.Where("item_id = ?", item.ID()).Delete(&itemUnitDBO{}).Error; err != nil {
			return err
		}
		if err := tx.Where("item_id = ?", item.ID()).Delete(&itemConversionDBO{}).Error; err != nil {
			return err
		}
		return createItemParts(tx, item)
	})
	return itemError(err, item, "save")
}

// createItemParts inserts an item's units and conversions.
func createItemParts(tx *gorm.DB, item *model.GroceryItem) error {
	units := make([]itemUnitDBO, 0, len(item.BuyUnitIDs())+len(item.CookUnitIDs()))
	for n, id := range item.BuyUnitIDs() {
		units = append(units, itemUnitDBO{ItemID: item.ID(), Purpose: purposeBuy, UnitID: id, Position: n})
	}
	for n, id := range item.CookUnitIDs() {
		units = append(units, itemUnitDBO{ItemID: item.ID(), Purpose: purposeCook, UnitID: id, Position: n})
	}
	if err := tx.Create(&units).Error; err != nil {
		return err
	}
	if len(item.Conversions()) == 0 {
		return nil
	}
	conversions := make([]itemConversionDBO, 0, len(item.Conversions()))
	for n, c := range item.Conversions() {
		conversions = append(conversions, itemConversionDBO{ItemID: item.ID(), FromUnitID: c.From, ToUnitID: c.To, Factor: c.Factor, Position: n})
	}
	return tx.Create(&conversions).Error
}

func itemError(err error, item *model.GroceryItem, action string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return model.NameTaken("grocery item", item.Name(), false)
	default:
		return fmt.Errorf("%s item: %w", action, err)
	}
}

// DeleteItem also deletes the item's units and conversions (ON DELETE CASCADE). It fails with
// model.ErrItemInUse while a recipe or the grocery list uses the item.
func (r *CatalogSqlRepository) DeleteItem(ctx context.Context, owner owner.ID, id uuid.UUID) error {
	err := r.db.WithContext(ctx).Where("owner_id = ? AND id = ?", string(owner), id).Delete(&itemDBO{}).Error
	switch {
	case err == nil:
		return nil
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return model.ErrItemInUse
	default:
		return fmt.Errorf("delete item: %w", err)
	}
}

// ---------------------------------------------------------------------------
// Starter data and account deletion
// ---------------------------------------------------------------------------

func (r *CatalogSqlRepository) StarterDataCopied(ctx context.Context, owner owner.ID) (bool, error) {
	var copied bool
	err := r.db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM starter_data_copies WHERE owner_id = ?)", string(owner)).Scan(&copied).Error
	if err != nil {
		return false, fmt.Errorf("check starter data: %w", err)
	}
	return copied, nil
}

// errAlreadyCopied rolls back a copy when another request copied the owner's starter data first.
var errAlreadyCopied = errors.New("starter data already copied")

// SaveStarterData saves the owner's copies of the starter categories and items, unless the owner
// already has them: then it saves nothing and returns false.
func (r *CatalogSqlRepository) SaveStarterData(ctx context.Context, owner owner.ID, categories []*model.Category, items []*model.GroceryItem) (bool, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		marked := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&starterDataCopyDBO{OwnerID: string(owner), CopiedAt: time.Now()})
		if marked.Error != nil {
			return marked.Error
		}
		if marked.RowsAffected == 0 {
			return errAlreadyCopied
		}
		for _, category := range categories {
			if err := tx.Create(toCategoryDBO(category)).Error; err != nil {
				return err
			}
		}
		for _, item := range items {
			if err := tx.Create(toItemDBO(item)).Error; err != nil {
				return err
			}
			if err := createItemParts(tx, item); err != nil {
				return err
			}
		}
		return nil
	})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, errAlreadyCopied):
		return false, nil
	default:
		return false, fmt.Errorf("copy starter data: %w", err)
	}
}

// DeleteOwnerData deletes the owner's categories and items, and the record of their starter data.
func (r *CatalogSqlRepository) DeleteOwnerData(ctx context.Context, owner owner.ID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range []any{&itemDBO{}, &categoryDBO{}, &starterDataCopyDBO{}} {
			if err := tx.Where("owner_id = ?", string(owner)).Delete(table).Error; err != nil {
				return fmt.Errorf("delete catalog: %w", err)
			}
		}
		return nil
	})
}

func toCategoryDBO(c *model.Category) *categoryDBO {
	return &categoryDBO{
		ID: c.ID(), OwnerID: string(c.Owner()), Name: c.Name(), Emoji: c.Emoji(),
		Source: string(c.Source()), SystemKey: optional(c.SystemKey()), Hidden: c.Hidden(),
	}
}

func toCategory(d categoryDBO) *model.Category {
	var systemKey string
	if d.SystemKey != nil {
		systemKey = *d.SystemKey
	}
	return model.CategoryOf(d.ID, owner.ID(d.OwnerID), d.Name, d.Emoji, model.Source(d.Source), systemKey, d.Hidden)
}

func toItemDBO(i *model.GroceryItem) *itemDBO {
	return &itemDBO{
		ID: i.ID(), OwnerID: string(i.Owner()), Name: i.Name(), Emoji: i.Emoji(), CategoryID: i.CategoryID(),
		Source: string(i.Source()), SystemKey: optional(i.SystemKey()), Hidden: i.Hidden(),
	}
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// ItemUsage says which units the owner's recipes and grocery list use the item in.
func (r *CatalogSqlRepository) ItemUsage(ctx context.Context, owner owner.ID, itemID uuid.UUID) (model.ItemUsage, error) {
	var usage model.ItemUsage
	err := r.db.WithContext(ctx).Model(&ingredientDBO{}).Distinct("unit_id").
		Where("owner_id = ? AND item_id = ?", string(owner), itemID).Pluck("unit_id", &usage.RecipeUnitIDs).Error
	if err != nil {
		return model.ItemUsage{}, fmt.Errorf("item usage: %w", err)
	}
	err = r.db.WithContext(ctx).Model(&groceryLineDBO{}).
		Where("owner_id = ? AND item_id = ?", string(owner), itemID).Pluck("unit_id", &usage.ListUnitIDs).Error
	if err != nil {
		return model.ItemUsage{}, fmt.Errorf("item usage: %w", err)
	}
	return usage, nil
}

// ItemsByID returns the owner's items, keyed by ID.
func (r *CatalogSqlRepository) ItemsByID(ctx context.Context, owner owner.ID) (map[uuid.UUID]*model.GroceryItem, error) {
	items, err := r.GetItems(ctx, owner)
	if err != nil {
		return nil, err
	}
	byID := make(map[uuid.UUID]*model.GroceryItem, len(items))
	for _, item := range items {
		byID[item.ID()] = item
	}
	return byID, nil
}
