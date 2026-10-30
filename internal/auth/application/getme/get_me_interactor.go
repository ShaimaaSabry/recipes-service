// Package getme is the use case for reading the signed-in account.
package getme

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

var ErrUserNotFound = fmt.Errorf("%w: account not found", apperrors.ErrNotFound)

type userRepository interface {
	// FindUser returns nil when there is no such account.
	FindUser(ctx context.Context, id uuid.UUID) (*model.User, error)
}

type Interactor struct {
	userRepository userRepository
}

func NewInteractor(userRepository userRepository) *Interactor {
	return &Interactor{userRepository: userRepository}
}

type Query struct {
	Owner owner.ID
}

func (i *Interactor) Execute(ctx context.Context, query Query) (*model.User, error) {
	id, err := model.OwnerOf(query.Owner)
	if err != nil {
		return nil, err
	}
	user, err := i.userRepository.FindUser(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}
