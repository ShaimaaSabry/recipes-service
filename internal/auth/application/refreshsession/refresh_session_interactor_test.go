package refreshsession

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/auth/application/session"
	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

type fakeTokens struct {
	token         *model.RefreshToken
	markFails     bool
	marked        bool
	revokedFamily uuid.UUID
}

func (f *fakeTokens) FindRefreshToken(_ context.Context, hash []byte) (*model.RefreshToken, error) {
	if f.token == nil || string(hash) != string(f.token.Hash()) {
		return nil, nil
	}
	return f.token, nil
}

func (f *fakeTokens) MarkRefreshTokenUsed(context.Context, uuid.UUID, time.Time) (bool, error) {
	f.marked = true
	return !f.markFails, nil
}

func (f *fakeTokens) RevokeRefreshTokenFamily(_ context.Context, familyID uuid.UUID, _ time.Time) error {
	f.revokedFamily = familyID
	return nil
}

type fakeIssuer struct{ continued bool }

func (f *fakeIssuer) Continue(context.Context, uuid.UUID, uuid.UUID) (session.Tokens, error) {
	f.continued = true
	return session.Tokens{AccessToken: "next-access", RefreshToken: "next-refresh"}, nil
}

func TestExecute(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	stored := func(usedAt *time.Time) (string, *fakeTokens) {
		token, record, err := model.NewRefreshToken(uuid.New(), uuid.New(), now.Add(time.Hour))
		require.NoError(t, err)
		return token, &fakeTokens{token: model.RefreshTokenOf(record.ID(), record.UserID(), record.FamilyID(), record.Hash(), record.ExpiresAt(), usedAt, nil)}
	}

	t.Run("issues the next tokens for an unused one", func(t *testing.T) {
		token, repo := stored(nil)
		issuer := &fakeIssuer{}

		tokens, err := NewInteractor(repo, issuer, clock).Execute(ctx, Command{RefreshToken: token})

		require.NoError(t, err)
		require.Equal(t, "next-refresh", tokens.RefreshToken)
		require.True(t, repo.marked)
		require.True(t, issuer.continued)
	})

	t.Run("refuses an unknown token", func(t *testing.T) {
		_, repo := stored(nil)

		_, err := NewInteractor(repo, &fakeIssuer{}, clock).Execute(ctx, Command{RefreshToken: "made-up"})

		require.ErrorIs(t, err, model.ErrInvalidRefreshToken)
	})

	t.Run("ends the whole session when a used token comes back", func(t *testing.T) {
		usedAt := now.Add(-time.Hour)
		token, repo := stored(&usedAt)
		issuer := &fakeIssuer{}

		_, err := NewInteractor(repo, issuer, clock).Execute(ctx, Command{RefreshToken: token})

		require.ErrorIs(t, err, model.ErrInvalidRefreshToken)
		require.Equal(t, repo.token.FamilyID(), repo.revokedFamily)
		require.False(t, issuer.continued)
	})

	t.Run("refuses an app's retry without ending the session", func(t *testing.T) {
		usedAt := now.Add(-time.Second)
		token, repo := stored(&usedAt)

		_, err := NewInteractor(repo, &fakeIssuer{}, clock).Execute(ctx, Command{RefreshToken: token})

		require.ErrorIs(t, err, model.ErrInvalidRefreshToken)
		require.Equal(t, uuid.Nil, repo.revokedFamily)
	})

	t.Run("refuses when another request used the token first", func(t *testing.T) {
		token, repo := stored(nil)
		repo.markFails = true
		issuer := &fakeIssuer{}

		_, err := NewInteractor(repo, issuer, clock).Execute(ctx, Command{RefreshToken: token})

		require.ErrorIs(t, err, model.ErrInvalidRefreshToken)
		require.False(t, issuer.continued)
	})
}
