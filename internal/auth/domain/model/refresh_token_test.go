package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewRefreshToken(t *testing.T) {
	userID, familyID, expiresAt := uuid.New(), uuid.New(), time.Now().Add(time.Hour)

	token, record, err := NewRefreshToken(userID, familyID, expiresAt)

	require.NoError(t, err)
	require.Len(t, token, 43, "32 random bytes, base64url without padding")
	require.Equal(t, HashRefreshToken(token), record.Hash())
	require.NotContains(t, string(record.Hash()), token, "only the hash is kept")
	require.Equal(t, userID, record.UserID())
	require.Equal(t, familyID, record.FamilyID())
	require.Equal(t, expiresAt, record.ExpiresAt())

	other, _, err := NewRefreshToken(userID, familyID, expiresAt)
	require.NoError(t, err)
	require.NotEqual(t, token, other)
}

func TestUsabilityAt(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	token := func(expiresAt time.Time, usedAt, revokedAt *time.Time) RefreshToken {
		return *RefreshTokenOf(uuid.New(), uuid.New(), uuid.New(), nil, expiresAt, usedAt, revokedAt)
	}
	at := func(t time.Time) *time.Time { return &t }
	later := now.Add(time.Hour)

	for name, tc := range map[string]struct {
		token RefreshToken
		want  Usability
	}{
		"unused and unexpired":                {token(later, nil, nil), Usable},
		"expired":                             {token(now, nil, nil), Refused},
		"signed out":                          {token(later, nil, at(now.Add(-time.Minute))), Refused},
		"used a moment ago (an app retrying)": {token(later, at(now.Add(-ReuseGrace)), nil), Refused},
		"used longer ago (stolen?)":           {token(later, at(now.Add(-ReuseGrace-time.Second)), nil), Reused},
		"used long ago but signed out since":  {token(later, at(now.Add(-time.Hour)), at(now.Add(-time.Minute))), Refused},
	} {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.token.UsabilityAt(now))
		})
	}
}
