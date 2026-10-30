package deleteaccount

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type fakeUsers struct{ deleted []uuid.UUID }

func (f *fakeUsers) DeleteUser(_ context.Context, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return nil
}

func TestExecute(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	me := owner.ID(userID.String())

	t.Run("deletes every module's data, then the account", func(t *testing.T) {
		users := &fakeUsers{}
		var steps []string
		deleter := func(name string) OwnerDataDeleter {
			return func(_ context.Context, o owner.ID) error {
				require.Empty(t, users.deleted, "data goes before the account")
				steps = append(steps, name+":"+string(o))
				return nil
			}
		}

		err := NewInteractor(users, deleter("recipes"), deleter("lists")).Execute(ctx, Command{Owner: me})

		require.NoError(t, err)
		require.Equal(t, []string{"recipes:" + string(me), "lists:" + string(me)}, steps)
		require.Equal(t, []uuid.UUID{userID}, users.deleted)
	})

	t.Run("keeps the account when a module fails, so it can be retried", func(t *testing.T) {
		users := &fakeUsers{}
		boom := errors.New("boom")

		err := NewInteractor(users, func(context.Context, owner.ID) error { return boom }).Execute(ctx, Command{Owner: me})

		require.ErrorIs(t, err, boom)
		require.Empty(t, users.deleted)
	})

	t.Run("refuses an owner that isn't an account", func(t *testing.T) {
		err := NewInteractor(&fakeUsers{}).Execute(ctx, Command{Owner: "device-1"})

		require.ErrorIs(t, err, model.ErrInvalidUserID)
	})
}
