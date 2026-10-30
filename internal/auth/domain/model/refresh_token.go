package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// ReuseGrace is how long after a refresh token is used that using it again is treated as the app
// retrying (e.g. two requests refreshing at once), not as a stolen token.
const ReuseGrace = 30 * time.Second

var (
	// ErrInvalidRefreshToken covers an unknown, expired, signed-out or already-used token alike, so
	// the response never tells an attacker which.
	ErrInvalidRefreshToken = fmt.Errorf("%w: the session has ended, please sign in again", apperrors.ErrUnauthorized)
	// ErrInvalidAccessToken is returned for an access token that is malformed, forged, expired, or
	// belongs to an account that no longer exists.
	ErrInvalidAccessToken = fmt.Errorf("%w: the access token is invalid or expired", apperrors.ErrUnauthorized)
)

// RefreshToken lets the app get a new access token without signing in again. Each one works once:
// using it issues the next one in the same family (one family per sign-in). Only its SHA-256 hash
// is stored, so a database leak doesn't leak working tokens.
type RefreshToken struct {
	id        uuid.UUID
	userID    uuid.UUID
	familyID  uuid.UUID
	hash      []byte
	expiresAt time.Time
	usedAt    *time.Time
	revokedAt *time.Time
}

// NewRefreshToken makes a random token for userID in familyID. It returns the token to give the
// app, which is never stored, and the record to store.
func NewRefreshToken(userID, familyID uuid.UUID, expiresAt time.Time) (string, *RefreshToken, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", nil, fmt.Errorf("new refresh token: %w", err)
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", nil, fmt.Errorf("new refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(secret)
	return token, &RefreshToken{id: id, userID: userID, familyID: familyID, hash: HashRefreshToken(token), expiresAt: expiresAt}, nil
}

// HashRefreshToken is how a token the app sends is looked up.
func HashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// RefreshTokenOf rebuilds a stored token.
func RefreshTokenOf(id, userID, familyID uuid.UUID, hash []byte, expiresAt time.Time, usedAt, revokedAt *time.Time) *RefreshToken {
	return &RefreshToken{id: id, userID: userID, familyID: familyID, hash: hash, expiresAt: expiresAt, usedAt: usedAt, revokedAt: revokedAt}
}

func (t RefreshToken) ID() uuid.UUID         { return t.id }
func (t RefreshToken) UserID() uuid.UUID     { return t.userID }
func (t RefreshToken) FamilyID() uuid.UUID   { return t.familyID }
func (t RefreshToken) Hash() []byte          { return t.hash }
func (t RefreshToken) ExpiresAt() time.Time  { return t.expiresAt }
func (t RefreshToken) UsedAt() *time.Time    { return t.usedAt }
func (t RefreshToken) RevokedAt() *time.Time { return t.revokedAt }

// Usability says what may be done with a token the app presents.
type Usability int

const (
	// Usable: issue the next token.
	Usable Usability = iota
	// Refused: answer 401 and change nothing (expired, signed out, or a retry within ReuseGrace).
	Refused
	// Reused: someone presented a token that was already exchanged. Either it was stolen or the
	// app misbehaved; end the whole family so a thief's copy stops working too.
	Reused
)

// UsabilityAt decides what using the token at now means.
func (t RefreshToken) UsabilityAt(now time.Time) Usability {
	switch {
	case t.revokedAt != nil, !now.Before(t.expiresAt):
		return Refused
	case t.usedAt == nil:
		return Usable
	case now.Sub(*t.usedAt) <= ReuseGrace:
		return Refused
	default:
		return Reused
	}
}
