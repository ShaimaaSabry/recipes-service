// Package httpserver builds the Gin engine every module registers its routes on, and runs it.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

const ownerKey = "owner"

// New returns an engine with logging, panic recovery, JSON errors and a GET /healthz the app
// uses to tell whether the server is reachable.
func New(verbose bool) *gin.Engine {
	if !verbose {
		gin.SetMode(gin.ReleaseMode)
	}
	useJSONFieldNames()

	r := gin.New()
	if verbose {
		r.Use(gin.Logger())
	} else {
		r.Use(requestLogger())
	}
	r.Use(gin.Recovery(), ErrorHandler())
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	return r
}

// requestLogger logs each request through slog, which main makes JSON in prod.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		if c.FullPath() == "/healthz" {
			return // the ALB calls it every 30 seconds
		}
		slog.InfoContext(c.Request.Context(), "request",
			"method", c.Request.Method,
			"route", c.FullPath(),
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

// Authenticator turns an access token into the owner it was issued to, or fails with
// apperrors.ErrUnauthorized. The auth module provides it.
type Authenticator func(ctx context.Context, accessToken string) (owner.ID, error)

// ErrMissingToken is returned for a request without an "Authorization: Bearer <token>" header.
var ErrMissingToken = fmt.Errorf("%w: sign in and send the access token as \"Authorization: Bearer <token>\"", apperrors.ErrUnauthorized)

// RequireOwner rejects requests without a valid access token, and keeps the owner for OwnerOf.
func RequireOwner(authenticate Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		scheme, token, _ := strings.Cut(c.GetHeader("Authorization"), " ")
		token = strings.TrimSpace(token)
		if !strings.EqualFold(scheme, "Bearer") || token == "" {
			_ = c.Error(ErrMissingToken)
			c.Abort()
			return
		}
		id, err := authenticate(c.Request.Context(), token)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}
		c.Set(ownerKey, id)
		c.Next()
	}
}

// OwnerOf returns the owner RequireOwner found. Only call it on routes behind RequireOwner.
func OwnerOf(c *gin.Context) owner.ID {
	return c.MustGet(ownerKey).(owner.ID)
}

// Run serves until ctx is cancelled (e.g. Ctrl-C or a SIGTERM from the platform), then lets
// requests in flight finish for up to 10 seconds.
func Run(ctx context.Context, handler http.Handler, port string) error {
	server := &http.Server{Addr: ":" + port, Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	errs := make(chan error, 1)
	go func() {
		slog.Info("listening", "port", port)
		errs <- server.ListenAndServe()
	}()

	select {
	case err := <-errs:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errs; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

// useJSONFieldNames makes validation errors name fields as the JSON does ("name", not "Name").
func useJSONFieldNames() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			return field.Name
		}
		return name
	})
}
