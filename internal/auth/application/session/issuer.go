// Package session issues the pair of tokens a signed-in app holds: a short-lived access token it
// sends with every request, and a refresh token it trades for the next pair. Sign-in and refresh
// both end here.
package session

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

// Tokens is what the app receives after signing in or refreshing.
type Tokens struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type accessTokenSigner interface {
	Sign(userID uuid.UUID, now time.Time) (token string, expiresAt time.Time, err error)
}

type refreshTokenRepository interface {
	SaveRefreshToken(ctx context.Context, token *model.RefreshToken) error
}

type Issuer struct {
	signer     accessTokenSigner
	repository refreshTokenRepository
	refreshTTL time.Duration
	now        func() time.Time
}

func NewIssuer(signer accessTokenSigner, repository refreshTokenRepository, refreshTTL time.Duration, now func() time.Time) *Issuer {
	return &Issuer{signer: signer, repository: repository, refreshTTL: refreshTTL, now: now}
}

// Start issues the first tokens of a new session (a sign-in).
func (i *Issuer) Start(ctx context.Context, userID uuid.UUID) (Tokens, error) {
	familyID, err := uuid.NewV7()
	if err != nil {
		return Tokens{}, fmt.Errorf("start session: %w", err)
	}
	return i.Continue(ctx, userID, familyID)
}

// Continue issues the next tokens of an existing session (a refresh).
func (i *Issuer) Continue(ctx context.Context, userID, familyID uuid.UUID) (Tokens, error) {
	now := i.now()
	accessToken, accessExpiresAt, err := i.signer.Sign(userID, now)
	if err != nil {
		return Tokens{}, fmt.Errorf("issue tokens: %w", err)
	}
	refreshToken, record, err := model.NewRefreshToken(userID, familyID, now.Add(i.refreshTTL))
	if err != nil {
		return Tokens{}, err
	}
	if err := i.repository.SaveRefreshToken(ctx, record); err != nil {
		return Tokens{}, err
	}
	return Tokens{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: record.ExpiresAt(),
	}, nil
}
