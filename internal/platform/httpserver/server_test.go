package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

func serve(r *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, request)
	return recorder
}

func TestHealthz(t *testing.T) {
	response := serve(New(false), http.MethodGet, "/healthz", "", nil)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"status":"ok"}`, response.Body.String())
}

func TestRequestLogger(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := New(false)
	r.GET("/v1/recipes/:id", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	serve(r, http.MethodGet, "/healthz", "", nil)
	serve(r, http.MethodGet, "/v1/recipes/secret-id", "", nil)

	var entry map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &entry), "exactly one JSON line, none for /healthz")
	require.Equal(t, "request", entry["msg"])
	require.Equal(t, "GET", entry["method"])
	// The route, not the path, so ids never end up in the logs.
	require.Equal(t, "/v1/recipes/:id", entry["route"])
	require.EqualValues(t, http.StatusNoContent, entry["status"])
}

func TestRequireOwner(t *testing.T) {
	r := New(false)
	r.GET("/mine", RequireOwner(), func(c *gin.Context) { c.String(http.StatusOK, string(OwnerOf(c))) })

	t.Run("passes the owner to the handler", func(t *testing.T) {
		response := serve(r, http.MethodGet, "/mine", "", map[string]string{owner.Header: "device-1"})

		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, "device-1", response.Body.String())
	})

	t.Run("rejects a request without one", func(t *testing.T) {
		response := serve(r, http.MethodGet, "/mine", "", nil)

		require.Equal(t, http.StatusBadRequest, response.Code)
		require.JSONEq(t, `{"error":"invalid input: the X-Device-Id header is required"}`, response.Body.String())
	})
}

func TestErrorHandler(t *testing.T) {
	type body struct {
		Name  string `json:"name" binding:"required"`
		Count int    `json:"count" binding:"max=3"`
	}
	r := New(false)
	r.POST("/bind", func(c *gin.Context) {
		var b body
		if err := c.ShouldBindJSON(&b); err != nil {
			_ = c.Error(err)
			return
		}
		c.Status(http.StatusNoContent)
	})
	failWith := func(err error) gin.HandlerFunc { return func(c *gin.Context) { _ = c.Error(err) } }
	r.GET("/invalid", failWith(fmt.Errorf("%w: bad name", apperrors.ErrInvalidInput)))
	r.GET("/missing", failWith(fmt.Errorf("%w: no such recipe", apperrors.ErrNotFound)))
	r.GET("/conflict", failWith(fmt.Errorf("%w: taken", apperrors.ErrConflict)))
	r.GET("/unknown", failWith(errors.New("database password is hunter2")))

	tests := []struct {
		name, method, path, body string
		wantStatus               int
		wantBody                 string
	}{
		{"malformed JSON", http.MethodPost, "/bind", `{"name":}`, 400, `{"errors":{"body":"invalid JSON at byte offset 9"}}`},
		{"cut-off JSON", http.MethodPost, "/bind", `{"name":`, 400, `{"errors":{"body":"invalid JSON: unexpected end"}}`},
		{"empty body", http.MethodPost, "/bind", ``, 400, `{"errors":{"body":"is required"}}`},
		{"wrong JSON type", http.MethodPost, "/bind", `{"name":1}`, 400, `{"errors":{"name":"invalid type: got \"number\", expected string"}}`},
		{"validation, by JSON field name", http.MethodPost, "/bind", `{"count":5}`, 400, `{"errors":{"name":"is required","count":"must be at most 3"}}`},
		{"invalid input", http.MethodGet, "/invalid", "", 400, `{"error":"invalid input: bad name"}`},
		{"not found", http.MethodGet, "/missing", "", 404, `{"error":"not found: no such recipe"}`},
		{"conflict", http.MethodGet, "/conflict", "", 409, `{"error":"conflict: taken"}`},
		{"anything else hides the details", http.MethodGet, "/unknown", "", 500, `{"error":"internal server error"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := serve(r, tt.method, tt.path, tt.body, nil)

			require.Equal(t, tt.wantStatus, response.Code)
			require.JSONEq(t, tt.wantBody, response.Body.String())
		})
	}
}
