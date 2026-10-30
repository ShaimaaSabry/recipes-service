package sql

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
	"github.com/mamienellie/recipes-service/internal/platform/database"
	"github.com/mamienellie/recipes-service/internal/platform/database/databasetest"
)

func newRepository(t *testing.T) *UserSqlRepository {
	db := databasetest.New(t)
	require.NoError(t, database.Migrate(context.Background(), db, "auth", Migrations()))
	return NewUserSqlRepository(db)
}

func newUser(name string) *model.User {
	return model.NewUser(uuid.Must(uuid.NewV7()), name+"@example.com", name, time.Now().UTC().Truncate(time.Microsecond))
}

func TestUsers(t *testing.T) {
	ctx := context.Background()
	apple := model.Identity{Provider: model.ProviderApple, Subject: "apple-1", Email: "ada@example.com"}

	t.Run("creates a user on the first sign-in and finds them after", func(t *testing.T) {
		repo := newRepository(t)
		ada := newUser("ada")

		created, isNew, err := repo.FindOrCreateUser(ctx, apple, ada)
		require.NoError(t, err)
		require.True(t, isNew)
		require.Equal(t, ada.ID(), created.ID())

		found, isNew, err := repo.FindOrCreateUser(ctx, apple, newUser("someone-else"))
		require.NoError(t, err)
		require.False(t, isNew)
		require.Equal(t, ada.ID(), found.ID())
		require.Equal(t, "ada", found.Name())
	})

	t.Run("keeps Apple and Google sign-ins with the same email apart", func(t *testing.T) {
		repo := newRepository(t)
		google := model.Identity{Provider: model.ProviderGoogle, Subject: "google-1", Email: apple.Email}

		a, _, err := repo.FindOrCreateUser(ctx, apple, newUser("a"))
		require.NoError(t, err)
		g, _, err := repo.FindOrCreateUser(ctx, google, newUser("g"))
		require.NoError(t, err)

		require.NotEqual(t, a.ID(), g.ID())
	})

	t.Run("creates one account when the first two sign-ins race", func(t *testing.T) {
		repo := newRepository(t)
		ids := make([]uuid.UUID, 8)
		var wg sync.WaitGroup
		for i := range ids {
			wg.Add(1)
			go func() {
				defer wg.Done()
				user, _, err := repo.FindOrCreateUser(ctx, apple, newUser("racer"))
				require.NoError(t, err)
				ids[i] = user.ID()
			}()
		}
		wg.Wait()

		for _, id := range ids {
			require.Equal(t, ids[0], id)
		}
	})

	t.Run("finds, checks and deletes a user with their sessions", func(t *testing.T) {
		repo := newRepository(t)
		ada := newUser("ada")
		_, _, err := repo.FindOrCreateUser(ctx, apple, ada)
		require.NoError(t, err)
		token, record, err := model.NewRefreshToken(ada.ID(), uuid.New(), time.Now().Add(time.Hour))
		require.NoError(t, err)
		require.NoError(t, repo.SaveRefreshToken(ctx, record))

		found, err := repo.FindUser(ctx, ada.ID())
		require.NoError(t, err)
		require.Equal(t, "ada@example.com", found.Email())
		exists, err := repo.UserExists(ctx, ada.ID())
		require.NoError(t, err)
		require.True(t, exists)

		require.NoError(t, repo.DeleteUser(ctx, ada.ID()))

		found, err = repo.FindUser(ctx, ada.ID())
		require.NoError(t, err)
		require.Nil(t, found)
		exists, err = repo.UserExists(ctx, ada.ID())
		require.NoError(t, err)
		require.False(t, exists)
		session, err := repo.FindRefreshToken(ctx, model.HashRefreshToken(token))
		require.NoError(t, err)
		require.Nil(t, session, "sessions go with the account")
		again, isNew, err := repo.FindOrCreateUser(ctx, apple, newUser("ada-again"))
		require.NoError(t, err)
		require.True(t, isNew, "signing in again starts a fresh account")
		require.NotEqual(t, ada.ID(), again.ID())
	})
}

func TestRefreshTokens(t *testing.T) {
	ctx := context.Background()
	setup := func(t *testing.T) (*UserSqlRepository, *model.User) {
		repo := newRepository(t)
		ada := newUser("ada")
		_, _, err := repo.FindOrCreateUser(ctx, model.Identity{Provider: model.ProviderDev, Subject: "ada"}, ada)
		require.NoError(t, err)
		return repo, ada
	}

	t.Run("finds a token by its hash, marks it used once, and revokes its family", func(t *testing.T) {
		repo, ada := setup(t)
		family := uuid.New()
		first, firstRecord, err := model.NewRefreshToken(ada.ID(), family, time.Now().Add(time.Hour))
		require.NoError(t, err)
		_, secondRecord, err := model.NewRefreshToken(ada.ID(), family, time.Now().Add(time.Hour))
		require.NoError(t, err)
		require.NoError(t, repo.SaveRefreshToken(ctx, firstRecord))
		require.NoError(t, repo.SaveRefreshToken(ctx, secondRecord))

		found, err := repo.FindRefreshToken(ctx, model.HashRefreshToken(first))
		require.NoError(t, err)
		require.Equal(t, firstRecord.ID(), found.ID())
		require.Nil(t, found.UsedAt())

		marked, err := repo.MarkRefreshTokenUsed(ctx, found.ID(), time.Now())
		require.NoError(t, err)
		require.True(t, marked)
		marked, err = repo.MarkRefreshTokenUsed(ctx, found.ID(), time.Now())
		require.NoError(t, err)
		require.False(t, marked, "only the first use counts")

		require.NoError(t, repo.RevokeRefreshTokenFamily(ctx, family, time.Now()))
		second, err := repo.FindRefreshToken(ctx, secondRecord.Hash())
		require.NoError(t, err)
		require.NotNil(t, second.RevokedAt())
	})

	t.Run("clears the user's expired tokens when saving a new one", func(t *testing.T) {
		repo, ada := setup(t)
		expired, expiredRecord, err := model.NewRefreshToken(ada.ID(), uuid.New(), time.Now().Add(-time.Hour))
		require.NoError(t, err)
		require.NoError(t, repo.SaveRefreshToken(ctx, expiredRecord))
		_, fresh, err := model.NewRefreshToken(ada.ID(), uuid.New(), time.Now().Add(time.Hour))
		require.NoError(t, err)

		require.NoError(t, repo.SaveRefreshToken(ctx, fresh))

		gone, err := repo.FindRefreshToken(ctx, model.HashRefreshToken(expired))
		require.NoError(t, err)
		require.Nil(t, gone)
	})

	t.Run("migrating twice changes nothing", func(t *testing.T) {
		db := databasetest.New(t)
		require.NoError(t, database.Migrate(ctx, db, "auth", Migrations()))
		require.NoError(t, database.Migrate(ctx, db, "auth", Migrations()))
	})
}
