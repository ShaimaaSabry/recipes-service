package accesstoken

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

var key = []byte("0123456789abcdef0123456789abcdef")

func TestJWT(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	clock := now
	tokens, err := New(key, 15*time.Minute, func() time.Time { return clock })
	require.NoError(t, err)
	userID := uuid.New()

	token, expiresAt, err := tokens.Sign(userID, now)
	require.NoError(t, err)
	require.Equal(t, now.Add(15*time.Minute), expiresAt)

	t.Run("verifies a token it signed", func(t *testing.T) {
		got, err := tokens.Verify(token)

		require.NoError(t, err)
		require.Equal(t, userID, got)
	})

	t.Run("refuses an expired token", func(t *testing.T) {
		clock = expiresAt.Add(time.Second)
		t.Cleanup(func() { clock = now })

		_, err := tokens.Verify(token)

		require.ErrorIs(t, err, model.ErrInvalidAccessToken)
	})

	t.Run("refuses a token signed with another key", func(t *testing.T) {
		other, err := New([]byte("another-key-another-key-another-key"), 15*time.Minute, func() time.Time { return now })
		require.NoError(t, err)
		forged, _, err := other.Sign(userID, now)
		require.NoError(t, err)

		_, err = tokens.Verify(forged)

		require.ErrorIs(t, err, model.ErrInvalidAccessToken)
	})

	t.Run("refuses an unsigned token", func(t *testing.T) {
		unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.RegisteredClaims{
			Issuer: issuer, Audience: jwt.ClaimStrings{audience}, Subject: userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		}).SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		_, err = tokens.Verify(unsigned)

		require.ErrorIs(t, err, model.ErrInvalidAccessToken)
	})

	t.Run("refuses a token for another audience", func(t *testing.T) {
		foreign, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Issuer: issuer, Audience: jwt.ClaimStrings{"another-app"}, Subject: userID.String(),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		}).SignedString(key)
		require.NoError(t, err)

		_, err = tokens.Verify(foreign)

		require.ErrorIs(t, err, model.ErrInvalidAccessToken)
	})

	t.Run("refuses garbage", func(t *testing.T) {
		_, err := tokens.Verify("not-a-jwt")

		require.ErrorIs(t, err, model.ErrInvalidAccessToken)
	})
}

func TestNewRefusesAShortKey(t *testing.T) {
	_, err := New([]byte("short"), time.Minute, time.Now)

	require.ErrorContains(t, err, "at least 32 bytes")
}
