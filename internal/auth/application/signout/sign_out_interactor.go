// Package signout is the use case for ending a session: its refresh tokens stop working, and its
// access token runs out within minutes.
package signout

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

type refreshTokenRepository interface {
	// FindRefreshToken returns nil when no token has this hash.
	FindRefreshToken(ctx context.Context, hash []byte) (*model.RefreshToken, error)
	RevokeRefreshTokenFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error
}

type Interactor struct {
	repository refreshTokenRepository
	now        func() time.Time
}

func NewInteractor(repository refreshTokenRepository, now func() time.Time) *Interactor {
	return &Interactor{repository: repository, now: now}
}

type Command struct {
	RefreshToken string
}

// Execute succeeds for an unknown or already-ended session too: either way it's over.
func (i *Interactor) Execute(ctx context.Context, command Command) error {
	token, err := i.repository.FindRefreshToken(ctx, model.HashRefreshToken(command.RefreshToken))
	if err != nil || token == nil {
		return err
	}
	return i.repository.RevokeRefreshTokenFamily(ctx, token.FamilyID(), i.now())
}
