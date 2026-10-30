// Package signin is the use case for signing in with Apple or Google. The first sign-in with an
// identity creates the account; later ones find it again by the provider's subject.
package signin

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/auth/application/session"
	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

type identityVerifier interface {
	// Verify checks the provider's ID token, failing with model.ErrInvalidProviderToken.
	Verify(ctx context.Context, provider model.Provider, idToken string) (model.Identity, error)
}

type userRepository interface {
	// FindOrCreateUser returns the account the identity belongs to, creating newUser for it when
	// there is none; created says which happened.
	FindOrCreateUser(ctx context.Context, identity model.Identity, newUser *model.User) (user *model.User, created bool, err error)
}

type sessionIssuer interface {
	Start(ctx context.Context, userID uuid.UUID) (session.Tokens, error)
}

type Interactor struct {
	verifier       identityVerifier
	userRepository userRepository
	issuer         sessionIssuer
	now            func() time.Time
}

func NewInteractor(verifier identityVerifier, userRepository userRepository, issuer sessionIssuer, now func() time.Time) *Interactor {
	return &Interactor{verifier: verifier, userRepository: userRepository, issuer: issuer, now: now}
}

type Command struct {
	Provider model.Provider
	IDToken  string
	// Name is what the app got from the provider's sign-in sheet. Apple shares it only on the
	// first sign-in and never puts it in the token, so the app passes it on.
	Name string
}

type Result struct {
	User    *model.User
	Created bool
	Tokens  session.Tokens
}

func (i *Interactor) Execute(ctx context.Context, command Command) (Result, error) {
	identity, err := i.verifier.Verify(ctx, command.Provider, command.IDToken)
	if err != nil {
		return Result{}, err
	}
	name := identity.Name
	if name == "" {
		name = command.Name
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Result{}, fmt.Errorf("sign in: %w", err)
	}

	user, created, err := i.userRepository.FindOrCreateUser(ctx, identity, model.NewUser(id, identity.Email, name, i.now()))
	if err != nil {
		return Result{}, err
	}
	tokens, err := i.issuer.Start(ctx, user.ID())
	if err != nil {
		return Result{}, err
	}
	return Result{User: user, Created: created, Tokens: tokens}, nil
}
