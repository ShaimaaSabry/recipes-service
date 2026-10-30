// Package authenticate is the use case behind every signed-in request: it turns an access token
// into the account it belongs to.
package authenticate

import (
	"context"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type accessTokenVerifier interface {
	// Verify returns the user the token was issued to, or model.ErrInvalidAccessToken.
	Verify(token string) (uuid.UUID, error)
}

type userRepository interface {
	UserExists(ctx context.Context, id uuid.UUID) (bool, error)
}

type Interactor struct {
	verifier       accessTokenVerifier
	userRepository userRepository
}

func NewInteractor(verifier accessTokenVerifier, userRepository userRepository) *Interactor {
	return &Interactor{verifier: verifier, userRepository: userRepository}
}

// Execute also checks the account still exists, so a token issued just before the account was
// deleted can't write data for it in the minutes it has left.
func (i *Interactor) Execute(ctx context.Context, accessToken string) (owner.ID, error) {
	userID, err := i.verifier.Verify(accessToken)
	if err != nil {
		return "", err
	}
	exists, err := i.userRepository.UserExists(ctx, userID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", model.ErrInvalidAccessToken
	}
	return owner.ID(userID.String()), nil
}
