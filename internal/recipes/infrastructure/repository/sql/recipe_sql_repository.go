// Package sql stores the recipes module's data in Postgres.
package sql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// pgerrcode holds the Postgres error codes the repository checks.
var pgerrcode = struct{ ForeignKeyViolation string }{ForeignKeyViolation: "23503"}

// RecipeSqlRepository stores recipes with their ingredients and steps, and the planned meals.
type RecipeSqlRepository struct {
	db *gorm.DB
}

func NewRecipeSqlRepository(db *gorm.DB) *RecipeSqlRepository {
	return &RecipeSqlRepository{db: db}
}

// GetRecipes returns the owner's recipes by name, each with its ingredients and steps.
func (r *RecipeSqlRepository) GetRecipes(ctx context.Context, owner owner.ID) ([]*model.Recipe, error) {
	return r.findRecipes(ctx, r.db.WithContext(ctx).Where("owner_id = ?", string(owner)).Order("lower(name), id"))
}

// FindRecipe returns nil when the owner has no recipe with that id.
func (r *RecipeSqlRepository) FindRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Recipe, error) {
	recipes, err := r.findRecipes(ctx, r.db.WithContext(ctx).Where("owner_id = ? AND id = ?", string(owner), id))
	if err != nil || len(recipes) == 0 {
		return nil, err
	}
	return recipes[0], nil
}

func (r *RecipeSqlRepository) findRecipes(ctx context.Context, query *gorm.DB) ([]*model.Recipe, error) {
	var dbos []recipeDBO
	if err := query.Find(&dbos).Error; err != nil {
		return nil, fmt.Errorf("get recipes: %w", err)
	}
	if len(dbos) == 0 {
		return []*model.Recipe{}, nil
	}
	ids := make([]uuid.UUID, 0, len(dbos))
	for _, d := range dbos {
		ids = append(ids, d.ID)
	}
	var ingredientRows []ingredientDBO
	if err := r.db.WithContext(ctx).Where("recipe_id IN ?", ids).Order("recipe_id, position").Find(&ingredientRows).Error; err != nil {
		return nil, fmt.Errorf("get ingredients: %w", err)
	}
	var stepRows []stepDBO
	if err := r.db.WithContext(ctx).Where("recipe_id IN ?", ids).Order("recipe_id, position").Find(&stepRows).Error; err != nil {
		return nil, fmt.Errorf("get steps: %w", err)
	}
	ingredients := map[uuid.UUID][]model.Ingredient{}
	for _, i := range ingredientRows {
		ingredients[i.RecipeID] = append(ingredients[i.RecipeID], model.Ingredient{ItemID: i.ItemID, UnitID: i.UnitID, Quantity: i.Quantity})
	}
	steps := map[uuid.UUID][]string{}
	for _, s := range stepRows {
		steps[s.RecipeID] = append(steps[s.RecipeID], s.Text)
	}

	recipes := make([]*model.Recipe, 0, len(dbos))
	for _, d := range dbos {
		details := model.RecipeDetails{
			Name: d.Name, Servings: d.Servings,
			Ingredients: orEmpty(ingredients[d.ID]), Steps: orEmpty(steps[d.ID]),
		}
		var photoKey string
		if d.PhotoKey != nil {
			photoKey = *d.PhotoKey
		}
		recipes = append(recipes, model.RecipeOf(d.ID, owner.ID(d.OwnerID), details, photoKey))
	}
	return recipes, nil
}

func (r *RecipeSqlRepository) CreateRecipe(ctx context.Context, recipe *model.Recipe) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		dbo := recipeDBO{
			ID: recipe.ID(), OwnerID: string(recipe.Owner()), Name: recipe.Name(),
			Servings: recipe.Servings(), PhotoKey: optional(recipe.PhotoKey()),
		}
		if err := tx.Create(&dbo).Error; err != nil {
			return err
		}
		return createRecipeParts(tx, recipe)
	})
	if err != nil {
		return fmt.Errorf("create recipe: %w", err)
	}
	return nil
}

// SaveRecipe saves an existing recipe: its details, photo, ingredients and steps.
func (r *RecipeSqlRepository) SaveRecipe(ctx context.Context, recipe *model.Recipe) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&recipeDBO{}).
			Where("owner_id = ? AND id = ?", string(recipe.Owner()), recipe.ID()).
			Updates(map[string]any{
				"name": recipe.Name(), "servings": recipe.Servings(), "photo_key": optional(recipe.PhotoKey()),
				"updated_at": time.Now(),
			}).Error
		if err != nil {
			return err
		}
		if err := tx.Where("recipe_id = ?", recipe.ID()).Delete(&ingredientDBO{}).Error; err != nil {
			return err
		}
		if err := tx.Where("recipe_id = ?", recipe.ID()).Delete(&stepDBO{}).Error; err != nil {
			return err
		}
		return createRecipeParts(tx, recipe)
	})
	if err != nil {
		return fmt.Errorf("save recipe: %w", err)
	}
	return nil
}

func createRecipeParts(tx *gorm.DB, recipe *model.Recipe) error {
	if len(recipe.Ingredients()) > 0 {
		rows := make([]ingredientDBO, 0, len(recipe.Ingredients()))
		for n, i := range recipe.Ingredients() {
			rows = append(rows, ingredientDBO{
				RecipeID: recipe.ID(), Position: n, OwnerID: string(recipe.Owner()),
				ItemID: i.ItemID, UnitID: i.UnitID, Quantity: i.Quantity,
			})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	if len(recipe.Steps()) > 0 {
		rows := make([]stepDBO, 0, len(recipe.Steps()))
		for n, text := range recipe.Steps() {
			rows = append(rows, stepDBO{RecipeID: recipe.ID(), Position: n, Text: text})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
	}
	return nil
}

// DeleteRecipe also deletes its ingredients, steps and planned meals (ON DELETE CASCADE).
func (r *RecipeSqlRepository) DeleteRecipe(ctx context.Context, owner owner.ID, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Where("owner_id = ? AND id = ?", string(owner), id).Delete(&recipeDBO{}).Error; err != nil {
		return fmt.Errorf("delete recipe: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Planned meals
// ---------------------------------------------------------------------------

// GetMeals returns the owner's planned meals in the order they were planned.
func (r *RecipeSqlRepository) GetMeals(ctx context.Context, owner owner.ID) ([]*model.PlannedMeal, error) {
	var dbos []plannedMealDBO
	if err := r.db.WithContext(ctx).Where("owner_id = ?", string(owner)).Order("created_at, id").Find(&dbos).Error; err != nil {
		return nil, fmt.Errorf("get meals: %w", err)
	}
	meals := make([]*model.PlannedMeal, 0, len(dbos))
	for _, d := range dbos {
		meals = append(meals, model.PlannedMealOf(d.ID, owner, d.RecipeID, d.Servings, d.CreatedAt))
	}
	return meals, nil
}

// SaveMeal creates the meal, or updates the owner's meal with its id. It fails with
// model.ErrMealIDTaken when another account has a meal with that id.
func (r *RecipeSqlRepository) SaveMeal(ctx context.Context, meal *model.PlannedMeal) (*model.PlannedMeal, error) {
	var saved []plannedMealDBO
	err := r.db.WithContext(ctx).Raw(
		`INSERT INTO planned_meals (id, owner_id, recipe_id, servings) VALUES (?, ?, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET recipe_id = EXCLUDED.recipe_id, servings = EXCLUDED.servings, updated_at = now()
		 WHERE planned_meals.owner_id = EXCLUDED.owner_id
		 RETURNING id, owner_id, recipe_id, servings, created_at, updated_at`,
		meal.ID(), string(meal.Owner()), meal.RecipeID(), meal.Servings(),
	).Scan(&saved).Error
	// Raw queries skip GORM's error translation, so this checks Postgres's code itself.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.ForeignKeyViolation {
		return nil, model.ErrNoSuchRecipe
	}
	if err != nil {
		return nil, fmt.Errorf("save meal: %w", err)
	}
	if len(saved) == 0 {
		return nil, model.ErrMealIDTaken
	}
	d := saved[0]
	return model.PlannedMealOf(d.ID, meal.Owner(), d.RecipeID, d.Servings, d.CreatedAt), nil
}

// DeleteMeal deletes the owner's meal with that id, if there is one.
func (r *RecipeSqlRepository) DeleteMeal(ctx context.Context, owner owner.ID, id uuid.UUID) error {
	if err := r.db.WithContext(ctx).Where("owner_id = ? AND id = ?", string(owner), id).Delete(&plannedMealDBO{}).Error; err != nil {
		return fmt.Errorf("delete meal: %w", err)
	}
	return nil
}

// DeleteOwnerData deletes the owner's recipes (with their meals) and grocery list, for account
// deletion. It runs before the catalog's, since both point at the owner's items.
func (r *RecipeSqlRepository) DeleteOwnerData(ctx context.Context, owner owner.ID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, table := range []any{&groceryLineDBO{}, &appliedActionDBO{}, &localImportDBO{}, &recipeDBO{}} {
			if err := tx.Where("owner_id = ?", string(owner)).Delete(table).Error; err != nil {
				return fmt.Errorf("delete recipes: %w", err)
			}
		}
		return nil
	})
}

func optional(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// SaveRecipePhoto saves only the recipe's photo, leaving the rest as it is.
func (r *RecipeSqlRepository) SaveRecipePhoto(ctx context.Context, recipe *model.Recipe) error {
	err := r.db.WithContext(ctx).Model(&recipeDBO{}).
		Where("owner_id = ? AND id = ?", string(recipe.Owner()), recipe.ID()).
		Updates(map[string]any{"photo_key": optional(recipe.PhotoKey()), "updated_at": time.Now()}).Error
	if err != nil {
		return fmt.Errorf("save recipe photo: %w", err)
	}
	return nil
}
