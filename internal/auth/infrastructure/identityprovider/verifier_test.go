package identityprovider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

const (
	testIssuer = "https://appleid.apple.com"
	clientID   = "com.mamienellie.app"
	keyID      = "key-1"
)

// fakeProvider publishes one RSA key the way Apple and Google do, and signs tokens with it.
type fakeProvider struct {
	key    *rsa.PrivateKey
	server *httptest.Server
}

func newFakeProvider(t *testing.T) *fakeProvider {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "use": "sig", "alg": "RS256", "kid": keyID,
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(server.Close)
	return &fakeProvider{key: key, server: server}
}

func (p *fakeProvider) sign(t *testing.T, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = keyID
	signed, err := token.SignedString(p.key)
	require.NoError(t, err)
	return signed
}

func validClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"iss": testIssuer, "aud": clientID, "sub": "apple-user-1",
		"email": "ada@privaterelay.appleid.com", "email_verified": "true",
		"iat": time.Now().Unix(), "exp": time.Now().Add(10 * time.Minute).Unix(),
	}
}

func TestVerify(t *testing.T) {
	ctx := context.Background()
	provider := newFakeProvider(t)
	verifier := New(ctx, map[model.Provider]Provider{
		model.ProviderApple: {Endpoints: Endpoints{Issuers: []string{testIssuer}, JWKSURL: provider.server.URL}, ClientIDs: []string{clientID}},
	}, false)

	t.Run("accepts a valid token", func(t *testing.T) {
		identity, err := verifier.Verify(ctx, model.ProviderApple, provider.sign(t, validClaims()))

		require.NoError(t, err)
		require.Equal(t, model.Identity{Provider: model.ProviderApple, Subject: "apple-user-1", Email: "ada@privaterelay.appleid.com"}, identity)
	})

	for name, change := range map[string]func(jwt.MapClaims){
		"for another app":     func(c jwt.MapClaims) { c["aud"] = "com.someone.else" },
		"from another issuer": func(c jwt.MapClaims) { c["iss"] = "https://evil.example" },
		"that has expired":    func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
	} {
		t.Run("refuses a token "+name, func(t *testing.T) {
			claims := validClaims()
			change(claims)

			_, err := verifier.Verify(ctx, model.ProviderApple, provider.sign(t, claims))

			require.ErrorIs(t, err, model.ErrInvalidProviderToken)
		})
	}

	t.Run("refuses a token signed with another key", func(t *testing.T) {
		impostor := newFakeProvider(t)

		_, err := verifier.Verify(ctx, model.ProviderApple, impostor.sign(t, validClaims()))

		require.ErrorIs(t, err, model.ErrInvalidProviderToken)
	})

	t.Run("refuses a provider that isn't set up", func(t *testing.T) {
		_, err := verifier.Verify(ctx, model.ProviderGoogle, provider.sign(t, validClaims()))

		require.ErrorIs(t, err, model.ErrInvalidProviderToken)
	})

	t.Run("refuses dev sign-in unless allowed", func(t *testing.T) {
		_, err := verifier.Verify(ctx, model.ProviderDev, "ada")

		require.ErrorIs(t, err, model.ErrInvalidProviderToken)
	})
}

func TestVerifyDev(t *testing.T) {
	verifier := New(context.Background(), nil, true)

	identity, err := verifier.Verify(context.Background(), model.ProviderDev, " ada ")

	require.NoError(t, err)
	require.Equal(t, model.Identity{Provider: model.ProviderDev, Subject: "ada", Name: "ada"}, identity)

	_, err = verifier.Verify(context.Background(), model.ProviderDev, " ")
	require.ErrorIs(t, err, model.ErrInvalidProviderToken)
}
