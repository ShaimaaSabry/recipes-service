// Package editcategory is the use case for renaming a category, starter ones included.
package editcategory

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type categoryRepository interface {
	FindCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Category, error)
	FindCategoryByName(ctx context.Context, owner owner.ID, name string) (*model.Category, error)
	SaveCategory(ctx context.Context, category *model.Category) error
}

type Interactor struct {
	categoryRepository categoryRepository
}

func NewInteractor(categoryRepository categoryRepository) *Interactor {
	return &Interactor{categoryRepository: categoryRepository}
}

type Command struct {
	Owner owner.ID
	ID    uuid.UUID
	Name  string
	Emoji string
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.Category, error) {
	category, err := i.categoryRepository.FindCategory(ctx, command.Owner, command.ID)
	if err != nil {
		return nil, err
	}
	if category == nil {
		return nil, model.ErrCategoryNotFound
	}
	if err := category.Edit(command.Name, command.Emoji); err != nil {
		return nil, err
	}
	taken, err := i.categoryRepository.FindCategoryByName(ctx, command.Owner, category.Name())
	if err != nil {
		return nil, err
	}
	if taken != nil && taken.ID() != category.ID() {
		return nil, model.NameTaken("category", taken.Name(), taken.Hidden())
	}
	if err := i.categoryRepository.SaveCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}
