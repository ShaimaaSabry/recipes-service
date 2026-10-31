// Package deletecategory is the use case for deleting one of the owner's own categories. Starter
// categories can only be hidden, and a category can't go while items are in it.
package deletecategory

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type categoryRepository interface {
	FindCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (*model.Category, error)
	CountItemsInCategory(ctx context.Context, owner owner.ID, id uuid.UUID) (int64, error)
	DeleteCategory(ctx context.Context, owner owner.ID, id uuid.UUID) error
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
}

func (i *Interactor) Execute(ctx context.Context, command Command) error {
	category, err := i.categoryRepository.FindCategory(ctx, command.Owner, command.ID)
	if err != nil {
		return err
	}
	if category == nil {
		return model.ErrCategoryNotFound
	}
	if err := category.CanDelete(); err != nil {
		return err
	}
	count, err := i.categoryRepository.CountItemsInCategory(ctx, command.Owner, command.ID)
	if err != nil {
		return err
	}
	if count > 0 {
		noun := "grocery items use"
		if count == 1 {
			noun = "grocery item uses"
		}
		return fmt.Errorf("%w: %d %s this category. Move them to another category first", apperrors.ErrConflict, count, noun)
	}
	return i.categoryRepository.DeleteCategory(ctx, command.Owner, command.ID)
}
