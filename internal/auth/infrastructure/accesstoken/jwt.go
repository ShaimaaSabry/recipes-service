// Package accesstoken signs and checks the access tokens the app sends with every request: JWTs
// signed with HMAC-SHA256 under a key only this service knows.
package accesstoken

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

const (
	issuer   = "mamie-nellie"
	audience = "mamie-nellie-app"
	// MinKeyLength is the shortest signing key accepted: 32 bytes, the size of the HMAC-SHA256 output.
	MinKeyLength = 32
)

type JWT struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

// New fails for a key shorter than MinKeyLength.
func New(key []byte, ttl time.Duration, now func() time.Time) (*JWT, error) {
	if len(key) < MinKeyLength {
		return nil, fmt.Errorf("access token signing key must be at least %d bytes", MinKeyLength)
	}
	return &JWT{key: key, ttl: ttl, now: now}, nil
}

func (j *JWT) Sign(userID uuid.UUID, now time.Time) (string, time.Time, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	expiresAt := now.Add(j.ttl).Truncate(time.Second)
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ID:        id.String(),
		Issuer:    issuer,
		Audience:  jwt.ClaimStrings{audience},
		Subject:   userID.String(),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}).SignedString(j.key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return token, expiresAt, nil
}

// Verify accepts only HS256 tokens this service signed for this app that haven't expired.
func (j *JWT) Verify(token string) (uuid.UUID, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return j.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(j.now),
	)
	if err != nil {
		// The library's reason (expired, bad signature…) stays out of the response.
		return uuid.Nil, model.ErrInvalidAccessToken
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, model.ErrInvalidAccessToken
	}
	return userID, nil
}
