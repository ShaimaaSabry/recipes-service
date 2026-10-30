// Package deleteaccount is the use case for deleting an account and everything saved with it.
// The App Store requires apps with sign-in to offer this.
package deleteaccount

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// OwnerDataDeleter deletes everything one module stores for an owner. Each module that keeps
// data provides one; it must succeed when there is nothing to delete.
type OwnerDataDeleter func(ctx context.Context, owner owner.ID) error

type userRepository interface {
	// DeleteUser deletes the account with its identities and sessions. No account is not an error.
	DeleteUser(ctx context.Context, id uuid.UUID) error
}

type Interactor struct {
	userRepository userRepository
	dataDeleters   []OwnerDataDeleter
}

func NewInteractor(userRepository userRepository, dataDeleters ...OwnerDataDeleter) *Interactor {
	return &Interactor{userRepository: userRepository, dataDeleters: dataDeleters}
}

type Command struct {
	Owner owner.ID
}

// Execute deletes the modules' data first and the account last: if anything fails part-way, the
// account still exists and the app can simply try again.
func (i *Interactor) Execute(ctx context.Context, command Command) error {
	id, err := model.OwnerOf(command.Owner)
	if err != nil {
		return err
	}
	for _, deleteData := range i.dataDeleters {
		if err := deleteData(ctx, command.Owner); err != nil {
			return fmt.Errorf("delete account data: %w", err)
		}
	}
	return i.userRepository.DeleteUser(ctx, id)
}
