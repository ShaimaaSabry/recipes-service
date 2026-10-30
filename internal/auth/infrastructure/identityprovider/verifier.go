// Package identityprovider checks the ID tokens Apple and Google give the app at sign-in. Each
// token is a JWT signed with the provider's published keys (fetched and cached from its JWKS
// URL), for one of our app's client IDs, and not yet expired.
package identityprovider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

// Endpoints is where a provider's tokens come from and where its signing keys are published.
type Endpoints struct {
	Issuers []string
	JWKSURL string
}

var (
	// Apple's documented issuer and keys.
	Apple = Endpoints{Issuers: []string{"https://appleid.apple.com"}, JWKSURL: "https://appleid.apple.com/auth/keys"}
	// Google signs with either form of its issuer.
	Google = Endpoints{
		Issuers: []string{"https://accounts.google.com", "accounts.google.com"},
		JWKSURL: "https://www.googleapis.com/oauth2/v3/certs",
	}
)

// Provider is one provider the service accepts, and the client IDs (Apple: the app's bundle ID;
// Google: its iOS, Android and web client IDs) its tokens must be issued for.
type Provider struct {
	Endpoints Endpoints
	ClientIDs []string
}

type configured struct {
	verifier  *oidc.IDTokenVerifier
	issuers   []string
	clientIDs []string
}

type Verifier struct {
	providers map[model.Provider]configured
	allowDev  bool
}

// New sets up the providers. ctx must outlive the verifier: the key sets fetch with it. allowDev
// accepts model.ProviderDev, whose "token" is just a name; never allow it outside local runs.
func New(ctx context.Context, providers map[model.Provider]Provider, allowDev bool) *Verifier {
	v := &Verifier{providers: make(map[model.Provider]configured, len(providers)), allowDev: allowDev}
	for name, p := range providers {
		keys := oidc.NewRemoteKeySet(ctx, p.Endpoints.JWKSURL)
		v.providers[name] = configured{
			// Issuer and audience are checked below: each provider has more than one.
			verifier:  oidc.NewVerifier("", keys, &oidc.Config{SkipIssuerCheck: true, SkipClientIDCheck: true}),
			issuers:   p.Endpoints.Issuers,
			clientIDs: p.ClientIDs,
		}
	}
	return v
}

type claims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (v *Verifier) Verify(ctx context.Context, provider model.Provider, idToken string) (model.Identity, error) {
	if provider == model.ProviderDev {
		return v.verifyDev(idToken)
	}
	p, ok := v.providers[provider]
	if !ok || len(p.clientIDs) == 0 {
		return model.Identity{}, fmt.Errorf("%w: signing in with %s isn't set up", model.ErrInvalidProviderToken, provider)
	}

	token, err := p.verifier.Verify(ctx, idToken)
	if err != nil || !slices.Contains(p.issuers, token.Issuer) || !slices.ContainsFunc(token.Audience, func(aud string) bool {
		return slices.Contains(p.clientIDs, aud)
	}) {
		// The reason (expired, wrong app, bad signature…) stays out of the response.
		return model.Identity{}, model.ErrInvalidProviderToken
	}
	var c claims
	if err := token.Claims(&c); err != nil {
		return model.Identity{}, model.ErrInvalidProviderToken
	}
	return model.Identity{Provider: provider, Subject: token.Subject, Email: c.Email, Name: c.Name}, nil
}

// MaxDevNameLength keeps local test accounts' subjects short.
const MaxDevNameLength = 64

func (v *Verifier) verifyDev(name string) (model.Identity, error) {
	name = strings.TrimSpace(name)
	if !v.allowDev || name == "" || len(name) > MaxDevNameLength {
		return model.Identity{}, model.ErrInvalidProviderToken
	}
	return model.Identity{Provider: model.ProviderDev, Subject: name, Name: name}, nil
}
