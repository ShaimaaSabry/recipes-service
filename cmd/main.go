package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	// Registers the generated OpenAPI spec for the Swagger UI.
	_ "github.com/mamienellie/recipes-service/docs"
	"github.com/mamienellie/recipes-service/internal/auth"
	"github.com/mamienellie/recipes-service/internal/platform/config"
	"github.com/mamienellie/recipes-service/internal/platform/database"
	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/recipes"
)

// @title        MamieNellie API
// @version      1.0
// @description  Recipes, grocery items and the grocery list for the MamieNellie app.
// @description  Sign in (locally: POST /auth/dev), then click Authorize and enter "Bearer <accessToken>".
// @BasePath     /v1
//
// @securityDefinitions.apikey BearerAuth
// @in                         header
// @name                       Authorization
// @description                "Bearer " followed by the accessToken from signing in.
func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Locally the settings come from .env.local; in prod the platform sets real env vars.
	if os.Getenv("APP_ENV") == "" {
		_ = godotenv.Load(".env.local")
	}
	cfg, err := config.FromEnvironment()
	if err != nil {
		return err
	}
	if !cfg.IsLocal() {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(cfg.DatabaseURL, cfg.IsLocal())
	if err != nil {
		return err
	}
	recipesModule := recipes.New(db)
	authModule, err := auth.New(ctx, db, auth.Settings{
		SigningKey:      []byte(cfg.JWTSigningKey),
		AppleClientIDs:  cfg.AppleClientIDs,
		GoogleClientIDs: cfg.GoogleClientIDs,
		AllowDevSignIn:  cfg.IsLocal(),
	}, recipesModule.DeleteOwnerData)
	if err != nil {
		return err
	}
	if err := authModule.Migrate(ctx); err != nil {
		return err
	}
	if err := recipesModule.Migrate(ctx); err != nil {
		return err
	}

	r := httpserver.New(cfg.IsLocal())
	public := r.Group("/v1")
	authModule.RegisterPublicRoutes(public)
	signedIn := r.Group("/v1", httpserver.RequireOwner(authModule.Authenticate))
	authModule.RegisterRoutes(signedIn)
	recipesModule.RegisterRoutes(signedIn)
	if cfg.IsLocal() {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	return httpserver.Run(ctx, r, cfg.Port)
}
