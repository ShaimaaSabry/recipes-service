// Package sql stores the recipes module's data in Postgres.
package sql

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

var ErrRecipeExists = fmt.Errorf("%w: a recipe with this id already exists", apperrors.ErrConflict)

type RecipeSqlRepository struct {
	db *gorm.DB
}

// NewRecipeSqlRepository needs a db opened with TranslateError (database.Open does), so a
// duplicate key comes back as gorm.ErrDuplicatedKey.
func NewRecipeSqlRepository(db *gorm.DB) *RecipeSqlRepository {
	return &RecipeSqlRepository{db: db}
}

func (r *RecipeSqlRepository) CreateRecipe(ctx context.Context, recipe *model.Recipe) error {
	dbo := recipeDBO{OwnerID: string(recipe.Owner()), ID: recipe.ID(), Name: recipe.Name()}
	err := r.db.WithContext(ctx).Create(&dbo).Error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrRecipeExists
	}
	if err != nil {
		return fmt.Errorf("create recipe: %w", err)
	}
	return nil
}

func (r *RecipeSqlRepository) GetRecipes(ctx context.Context, owner owner.ID) ([]model.Recipe, error) {
	var dbos []recipeDBO
	err := r.db.WithContext(ctx).Where("owner_id = ?", string(owner)).Order("name, id").Find(&dbos).Error
	if err != nil {
		return nil, fmt.Errorf("get recipes: %w", err)
	}
	recipes := make([]model.Recipe, 0, len(dbos))
	for _, dbo := range dbos {
		recipes = append(recipes, *model.Of(dbo.ID, owner, dbo.Name))
	}
	return recipes, nil
}
