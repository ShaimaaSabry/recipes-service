package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/platform/database/databasetest"
	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// A signed-in route of another module, to check the token gets through.
func whoAmI(c *gin.Context) { c.String(http.StatusOK, string(httpserver.OwnerOf(c))) }

func newServer(t *testing.T, deleted *[]owner.ID) *gin.Engine {
	ctx := context.Background()
	module, err := New(ctx, databasetest.New(t), Settings{
		SigningKey:     []byte("0123456789abcdef0123456789abcdef"),
		AllowDevSignIn: true,
	}, Hooks{DeleteOwnerData: []OwnerDataDeleter{func(_ context.Context, o owner.ID) error {
		*deleted = append(*deleted, o)
		return nil
	}}})
	require.NoError(t, err)
	require.NoError(t, module.Migrate(ctx))

	r := httpserver.New(false)
	module.RegisterPublicRoutes(r.Group("/v1"))
	signedIn := r.Group("/v1", httpserver.RequireOwner(module.Authenticate))
	module.RegisterRoutes(signedIn)
	signedIn.GET("/whoami", whoAmI)
	return r
}

func call(r *gin.Engine, method, path, body, accessToken string) (int, map[string]any, string) {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	var decoded map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &decoded)
	return recorder.Code, decoded, recorder.Body.String()
}

func TestSessionLifecycle(t *testing.T) {
	var deleted []owner.ID
	r := newServer(t, &deleted)

	// Sign in: a new account.
	status, signIn, _ := call(r, http.MethodPost, "/v1/auth/dev", `{"name":"ada"}`, "")
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, true, signIn["newAccount"])
	access, refresh := signIn["accessToken"].(string), signIn["refreshToken"].(string)
	userID := signIn["user"].(map[string]any)["id"].(string)

	// The access token works on other modules' routes, as the account.
	status, _, body := call(r, http.MethodGet, "/v1/whoami", "", access)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, userID, body)
	status, me, _ := call(r, http.MethodGet, "/v1/me", "", access)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "ada", me["name"])

	// Signing in again finds the same account.
	_, again, _ := call(r, http.MethodPost, "/v1/auth/dev", `{"name":"ada"}`, "")
	require.Equal(t, false, again["newAccount"])
	require.Equal(t, userID, again["user"].(map[string]any)["id"])

	// Refresh: new tokens; the old refresh token is spent.
	status, next, _ := call(r, http.MethodPost, "/v1/auth/refresh", `{"refreshToken":"`+refresh+`"}`, "")
	require.Equal(t, http.StatusOK, status)
	require.NotEqual(t, refresh, next["refreshToken"])
	status, _, _ = call(r, http.MethodPost, "/v1/auth/refresh", `{"refreshToken":"`+refresh+`"}`, "")
	require.Equal(t, http.StatusUnauthorized, status)

	// Sign out: the session's refresh token stops working.
	status, _, _ = call(r, http.MethodPost, "/v1/auth/sign-out", `{"refreshToken":"`+next["refreshToken"].(string)+`"}`, "")
	require.Equal(t, http.StatusNoContent, status)
	status, _, _ = call(r, http.MethodPost, "/v1/auth/refresh", `{"refreshToken":"`+next["refreshToken"].(string)+`"}`, "")
	require.Equal(t, http.StatusUnauthorized, status)

	// Delete the account: other modules' data first, then even a fresh access token stops working.
	status, _, _ = call(r, http.MethodDelete, "/v1/me", "", again["accessToken"].(string))
	require.Equal(t, http.StatusNoContent, status)
	require.Equal(t, []owner.ID{owner.ID(userID)}, deleted)
	status, _, _ = call(r, http.MethodGet, "/v1/whoami", "", again["accessToken"].(string))
	require.Equal(t, http.StatusUnauthorized, status)
}

func TestSignedInRoutesNeedAToken(t *testing.T) {
	r := newServer(t, new([]owner.ID))

	for _, token := range []string{"", "not-a-token"} {
		status, _, _ := call(r, http.MethodGet, "/v1/me", "", token)
		require.Equal(t, http.StatusUnauthorized, status)
	}
}

func TestBadRequests(t *testing.T) {
	r := newServer(t, new([]owner.ID))

	status, _, body := call(r, http.MethodPost, "/v1/auth/apple", `{}`, "")
	require.Equal(t, http.StatusBadRequest, status)
	require.JSONEq(t, `{"errors":{"idToken":"is required"}}`, body)

	status, _, body = call(r, http.MethodPost, "/v1/auth/google", `{"idToken":"forged"}`, "")
	require.Equal(t, http.StatusUnauthorized, status, body)
}
