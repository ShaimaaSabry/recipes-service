package model

import (
	"fmt"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// Provider is who vouches for a user's identity at sign-in.
type Provider string

const (
	ProviderApple  Provider = "apple"
	ProviderGoogle Provider = "google"
	// ProviderDev signs in without Apple or Google. It exists only when running locally.
	ProviderDev Provider = "dev"
)

// ErrInvalidProviderToken is returned when Apple or Google doesn't vouch for the token.
var ErrInvalidProviderToken = fmt.Errorf("%w: the sign-in token is invalid or expired", apperrors.ErrUnauthorized)

// Identity is what a provider says about the person signing in. Subject is the provider's
// permanent ID for them, which is how they're recognised next time; the email can change, and
// Apple may hide it behind a relay address or leave it out.
type Identity struct {
	Provider Provider
	Subject  string
	Email    string
	Name     string
}
