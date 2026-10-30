// Package refreshsession is the use case for trading a refresh token for new tokens.
package refreshsession

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/auth/application/session"
	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

type refreshTokenRepository interface {
	// FindRefreshToken returns nil when no token has this hash.
	FindRefreshToken(ctx context.Context, hash []byte) (*model.RefreshToken, error)
	// MarkRefreshTokenUsed reports false when another request used the token first.
	MarkRefreshTokenUsed(ctx context.Context, id uuid.UUID, at time.Time) (bool, error)
	RevokeRefreshTokenFamily(ctx context.Context, familyID uuid.UUID, at time.Time) error
}

type sessionIssuer interface {
	Continue(ctx context.Context, userID, familyID uuid.UUID) (session.Tokens, error)
}

type Interactor struct {
	repository refreshTokenRepository
	issuer     sessionIssuer
	now        func() time.Time
}

func NewInteractor(repository refreshTokenRepository, issuer sessionIssuer, now func() time.Time) *Interactor {
	return &Interactor{repository: repository, issuer: issuer, now: now}
}

type Command struct {
	RefreshToken string
}

func (i *Interactor) Execute(ctx context.Context, command Command) (session.Tokens, error) {
	token, err := i.repository.FindRefreshToken(ctx, model.HashRefreshToken(command.RefreshToken))
	if err != nil {
		return session.Tokens{}, err
	}
	if token == nil {
		return session.Tokens{}, model.ErrInvalidRefreshToken
	}

	now := i.now()
	switch token.UsabilityAt(now) {
	case model.Refused:
		return session.Tokens{}, model.ErrInvalidRefreshToken
	case model.Reused:
		if err := i.repository.RevokeRefreshTokenFamily(ctx, token.FamilyID(), now); err != nil {
			return session.Tokens{}, err
		}
		return session.Tokens{}, model.ErrInvalidRefreshToken
	}

	marked, err := i.repository.MarkRefreshTokenUsed(ctx, token.ID(), now)
	if err != nil {
		return session.Tokens{}, err
	}
	if !marked {
		return session.Tokens{}, model.ErrInvalidRefreshToken
	}
	return i.issuer.Continue(ctx, token.UserID(), token.FamilyID())
}
