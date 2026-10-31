// Package createcategory is the use case for adding a category of the owner's own.
package createcategory

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type categoryRepository interface {
	FindCategoryByName(ctx context.Context, owner owner.ID, name string) (*model.Category, error)
	CreateCategory(ctx context.Context, category *model.Category) error
}

type Interactor struct {
	categoryRepository categoryRepository
}

func NewInteractor(categoryRepository categoryRepository) *Interactor {
	return &Interactor{categoryRepository: categoryRepository}
}

type Command struct {
	Owner owner.ID
	Name  string
	Emoji string
}

func (i *Interactor) Execute(ctx context.Context, command Command) (*model.Category, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("create category: %w", err)
	}
	category, err := model.NewCategory(id, command.Owner, command.Name, command.Emoji)
	if err != nil {
		return nil, err
	}
	taken, err := i.categoryRepository.FindCategoryByName(ctx, command.Owner, category.Name())
	if err != nil {
		return nil, err
	}
	if taken != nil {
		return nil, model.NameTaken("category", taken.Name(), taken.Hidden())
	}
	if err := i.categoryRepository.CreateCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}
