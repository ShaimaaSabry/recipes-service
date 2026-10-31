// Package auth wires the auth module: accounts, Sign in with Apple and Google, and the sessions
// (access and refresh tokens) every other module's routes rely on. Other modules and main use
// only this package, never its layers directly.
package auth

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/mamienellie/recipes-service/internal/auth/application/authenticate"
	"github.com/mamienellie/recipes-service/internal/auth/application/deleteaccount"
	"github.com/mamienellie/recipes-service/internal/auth/application/getme"
	"github.com/mamienellie/recipes-service/internal/auth/application/refreshsession"
	"github.com/mamienellie/recipes-service/internal/auth/application/session"
	"github.com/mamienellie/recipes-service/internal/auth/application/signin"
	"github.com/mamienellie/recipes-service/internal/auth/application/signout"
	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
	"github.com/mamienellie/recipes-service/internal/auth/entrypoint/api"
	"github.com/mamienellie/recipes-service/internal/auth/infrastructure/accesstoken"
	"github.com/mamienellie/recipes-service/internal/auth/infrastructure/identityprovider"
	"github.com/mamienellie/recipes-service/internal/auth/infrastructure/repository/sql"
	"github.com/mamienellie/recipes-service/internal/platform/database"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

const (
	// AccessTokenTTL is short: a stolen access token works for minutes at most.
	AccessTokenTTL = 15 * time.Minute
	// RefreshTokenTTL is how long the app stays signed in without being opened.
	RefreshTokenTTL = 60 * 24 * time.Hour
)

// OwnerDataDeleter deletes everything a module stores for an owner when the account is deleted.
type OwnerDataDeleter = deleteaccount.OwnerDataDeleter

// AccountPreparer readies what a module keeps for an account (its starter data) on every sign-in.
type AccountPreparer = signin.AccountPreparer

// Hooks are how other modules take part in an account's life.
type Hooks struct {
	// PrepareAccount run, in order, on every sign-in before the tokens are issued.
	PrepareAccount []AccountPreparer
	// DeleteOwnerData run, in order, before an account is deleted.
	DeleteOwnerData []OwnerDataDeleter
}

type Settings struct {
	// SigningKey signs access tokens: at least 32 random bytes, the same on every server.
	SigningKey      []byte
	AppleClientIDs  []string
	GoogleClientIDs []string
	// AllowDevSignIn adds POST /v1/auth/dev. Only ever true when running locally.
	AllowDevSignIn bool
}

type Module struct {
	db             *gorm.DB
	allowDevSignIn bool
	authenticate   *authenticate.Interactor
	controller     *api.AuthController
}

// New wires the module. ctx must last as long as the server: the Apple and Google key sets fetch
// with it.
func New(ctx context.Context, db *gorm.DB, settings Settings, hooks Hooks) (*Module, error) {
	now := time.Now
	tokens, err := accesstoken.New(settings.SigningKey, AccessTokenTTL, now)
	if err != nil {
		return nil, err
	}
	repository := sql.NewUserSqlRepository(db)
	verifier := identityprovider.New(ctx, map[model.Provider]identityprovider.Provider{
		model.ProviderApple:  {Endpoints: identityprovider.Apple, ClientIDs: settings.AppleClientIDs},
		model.ProviderGoogle: {Endpoints: identityprovider.Google, ClientIDs: settings.GoogleClientIDs},
	}, settings.AllowDevSignIn)
	issuer := session.NewIssuer(tokens, repository, RefreshTokenTTL, now)

	return &Module{
		db:             db,
		allowDevSignIn: settings.AllowDevSignIn,
		authenticate:   authenticate.NewInteractor(tokens, repository),
		controller: api.NewAuthController(
			signin.NewInteractor(verifier, repository, issuer, now, hooks.PrepareAccount...),
			refreshsession.NewInteractor(repository, issuer, now),
			signout.NewInteractor(repository, now),
			getme.NewInteractor(repository),
			deleteaccount.NewInteractor(repository, hooks.DeleteOwnerData...),
		),
	}, nil
}

// Migrate brings the module's tables up to date.
func (m *Module) Migrate(ctx context.Context) error {
	return database.Migrate(ctx, m.db, "auth", sql.Migrations())
}

// Authenticate is the httpserver.Authenticator every signed-in route uses.
func (m *Module) Authenticate(ctx context.Context, accessToken string) (owner.ID, error) {
	return m.authenticate.Execute(ctx, accessToken)
}

// RegisterPublicRoutes adds sign-in, refresh and sign-out, which work without an access token.
func (m *Module) RegisterPublicRoutes(r gin.IRouter) {
	m.controller.RegisterPublicRoutes(r, m.allowDevSignIn)
}

// RegisterRoutes adds the account routes to r, which must sit behind httpserver.RequireOwner.
func (m *Module) RegisterRoutes(r gin.IRouter) {
	m.controller.RegisterRoutes(r)
}
