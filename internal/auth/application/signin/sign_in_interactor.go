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
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// AccountPreparer readies what one module keeps for an account, e.g. its starter data. It runs on
// every sign-in, before the tokens are issued, so one that failed on the first sign-in is retried
// on the next: it must do nothing for an account it has already prepared.
type AccountPreparer func(ctx context.Context, owner owner.ID) error

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
	preparers      []AccountPreparer
}

func NewInteractor(verifier identityVerifier, userRepository userRepository, issuer sessionIssuer, now func() time.Time, preparers ...AccountPreparer) *Interactor {
	return &Interactor{verifier: verifier, userRepository: userRepository, issuer: issuer, now: now, preparers: preparers}
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
	// Without tokens the app can't go on, so a failure here fails the sign-in and the app retries.
	for _, prepare := range i.preparers {
		if err := prepare(ctx, user.Owner()); err != nil {
			return Result{}, fmt.Errorf("prepare account: %w", err)
		}
	}
	tokens, err := i.issuer.Start(ctx, user.ID())
	if err != nil {
		return Result{}, err
	}
	return Result{User: user, Created: created, Tokens: tokens}, nil
}
