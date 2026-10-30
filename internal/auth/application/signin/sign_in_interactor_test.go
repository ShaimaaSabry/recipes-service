package signin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/auth/application/session"
	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

type fakeVerifier struct {
	identity model.Identity
	err      error
}

func (f fakeVerifier) Verify(_ context.Context, provider model.Provider, _ string) (model.Identity, error) {
	f.identity.Provider = provider
	return f.identity, f.err
}

type fakeUsers struct {
	existing *model.User
	created  *model.User
}

func (f *fakeUsers) FindOrCreateUser(_ context.Context, _ model.Identity, newUser *model.User) (*model.User, bool, error) {
	if f.existing != nil {
		return f.existing, false, nil
	}
	f.created = newUser
	return newUser, true, nil
}

type fakeIssuer struct{ startedFor uuid.UUID }

func (f *fakeIssuer) Start(_ context.Context, userID uuid.UUID) (session.Tokens, error) {
	f.startedFor = userID
	return session.Tokens{AccessToken: "access", RefreshToken: "refresh"}, nil
}

func TestExecute(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }

	t.Run("creates the account on the first sign-in", func(t *testing.T) {
		users, issuer := &fakeUsers{}, &fakeIssuer{}
		verifier := fakeVerifier{identity: model.Identity{Subject: "apple-1", Email: "ada@example.com"}}

		result, err := NewInteractor(verifier, users, issuer, clock).Execute(ctx, Command{Provider: model.ProviderApple, IDToken: "t", Name: "Ada"})

		require.NoError(t, err)
		require.True(t, result.Created)
		require.Equal(t, "Ada", result.User.Name(), "Apple's name comes from the app")
		require.Equal(t, "ada@example.com", result.User.Email())
		require.Equal(t, now, result.User.CreatedAt())
		require.Equal(t, result.User.ID(), issuer.startedFor)
		require.Equal(t, uuid.Version(7), result.User.ID().Version())
		require.Equal(t, "access", result.Tokens.AccessToken)
	})

	t.Run("prefers the name in the provider's token", func(t *testing.T) {
		users := &fakeUsers{}
		verifier := fakeVerifier{identity: model.Identity{Subject: "google-1", Name: "Ada Lovelace"}}

		result, err := NewInteractor(verifier, users, &fakeIssuer{}, clock).Execute(ctx, Command{Provider: model.ProviderGoogle, Name: "ignored"})

		require.NoError(t, err)
		require.Equal(t, "Ada Lovelace", result.User.Name())
	})

	t.Run("signs into the existing account", func(t *testing.T) {
		existing := model.Of(uuid.New(), "", "Ada", now.Add(-time.Hour))
		users, issuer := &fakeUsers{existing: existing}, &fakeIssuer{}

		result, err := NewInteractor(fakeVerifier{}, users, issuer, clock).Execute(ctx, Command{Provider: model.ProviderApple})

		require.NoError(t, err)
		require.False(t, result.Created)
		require.Same(t, existing, result.User)
		require.Equal(t, existing.ID(), issuer.startedFor)
	})

	t.Run("creates nothing for a token the provider rejects", func(t *testing.T) {
		users, issuer := &fakeUsers{}, &fakeIssuer{}

		_, err := NewInteractor(fakeVerifier{err: model.ErrInvalidProviderToken}, users, issuer, clock).Execute(ctx, Command{Provider: model.ProviderApple})

		require.True(t, errors.Is(err, model.ErrInvalidProviderToken))
		require.Nil(t, users.created)
		require.Equal(t, uuid.Nil, issuer.startedFor)
	})
}
