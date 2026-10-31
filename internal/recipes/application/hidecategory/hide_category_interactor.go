// Package hidecategory is the use case for hiding a category from pickers, or showing it again.
// Hiding is how starter categories, which can't be deleted, are put away.
package hidecategory

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type categoryRepository interface {
	FindCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Category, error)
	SaveCategory(ctx context.Context, category *model.Category) error
}

type Interactor struct {
	categoryRepository categoryRepository
}

func NewInteractor(categoryRepository categoryRepository) *Interactor {
	return &Interactor{categoryRepository: categoryRepository}
}

type Command struct {
	Owner  owner.ID
	ID     uuid.UUID
	Hidden bool
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.Category, error) {
	category, err := i.categoryRepository.FindCategory(ctx, command.Owner, command.ID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, model.ErrCategoryNotFound
	}
	category.SetHidden(command.Hidden)
	if err := i.categoryRepository.SaveCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}
