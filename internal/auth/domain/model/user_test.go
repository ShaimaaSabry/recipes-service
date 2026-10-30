package model

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewUser(t *testing.T) {
	id, now := uuid.New(), time.Now()

	t.Run("tidies the name and email", func(t *testing.T) {
		user := NewUser(id, " ada@example.com ", "  Ada   Lovelace ", now)

		require.Equal(t, "ada@example.com", user.Email())
		require.Equal(t, "Ada Lovelace", user.Name())
		require.Equal(t, id.String(), string(user.Owner()))
	})

	t.Run("cuts a long name to MaxNameLength characters", func(t *testing.T) {
		user := NewUser(id, "", strings.Repeat("é", MaxNameLength+10), now)

		require.Equal(t, MaxNameLength, len([]rune(user.Name())))
	})

	t.Run("allows no email and no name", func(t *testing.T) {
		user := NewUser(id, "", "", now)

		require.Empty(t, user.Email())
		require.Empty(t, user.Name())
	})
}

func TestOwnerOf(t *testing.T) {
	id := uuid.New()

	got, err := OwnerOf(NewUser(id, "", "", time.Now()).Owner())
	require.NoError(t, err)
	require.Equal(t, id, got)

	_, err = OwnerOf("device-1")
	require.ErrorIs(t, err, ErrInvalidUserID)
}
