package authenticate

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

type fakeVerifier struct {
	userID uuid.UUID
	err    error
}

func (f fakeVerifier) Verify(string) (uuid.UUID, error) { return f.userID, f.err }

type fakeUsers struct{ exists bool }

func (f fakeUsers) UserExists(context.Context, uuid.UUID) (bool, error) { return f.exists, nil }

func TestExecute(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()

	t.Run("returns the token's owner", func(t *testing.T) {
		got, err := NewInteractor(fakeVerifier{userID: userID}, fakeUsers{exists: true}).Execute(ctx, "token")

		require.NoError(t, err)
		require.Equal(t, userID.String(), string(got))
	})

	t.Run("refuses an invalid token", func(t *testing.T) {
		_, err := NewInteractor(fakeVerifier{err: model.ErrInvalidAccessToken}, fakeUsers{exists: true}).Execute(ctx, "token")

		require.ErrorIs(t, err, model.ErrInvalidAccessToken)
	})

	t.Run("refuses a valid token for a deleted account", func(t *testing.T) {
		_, err := NewInteractor(fakeVerifier{userID: userID}, fakeUsers{exists: false}).Execute(ctx, "token")

		require.ErrorIs(t, err, model.ErrInvalidAccessToken)
	})
}
