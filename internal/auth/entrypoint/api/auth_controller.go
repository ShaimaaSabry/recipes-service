// Package api is the auth module's HTTP entrypoint.
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mamienellie/recipes-service/internal/auth/application/deleteaccount"
	"github.com/mamienellie/recipes-service/internal/auth/application/getme"
	"github.com/mamienellie/recipes-service/internal/auth/application/refreshsession"
	"github.com/mamienellie/recipes-service/internal/auth/application/session"
	"github.com/mamienellie/recipes-service/internal/auth/application/signin"
	"github.com/mamienellie/recipes-service/internal/auth/application/signout"
	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
)

type signInInteractor interface {
	Execute(ctx context.Context, command signin.Command) (signin.Result, error)
}

type refreshSessionInteractor interface {
	Execute(ctx context.Context, command refreshsession.Command) (session.Tokens, error)
}

type signOutInteractor interface {
	Execute(ctx context.Context, command signout.Command) error
}

type getMeInteractor interface {
	Execute(ctx context.Context, query getme.Query) (*model.User, error)
}

type deleteAccountInteractor interface {
	Execute(ctx context.Context, command deleteaccount.Command) error
}

type AuthController struct {
	signIn         signInInteractor
	refreshSession refreshSessionInteractor
	signOut        signOutInteractor
	getMe          getMeInteractor
	deleteAccount  deleteAccountInteractor
}

func NewAuthController(
	signIn signInInteractor,
	refreshSession refreshSessionInteractor,
	signOut signOutInteractor,
	getMe getMeInteractor,
	deleteAccount deleteAccountInteractor,
) *AuthController {
	return &AuthController{signIn: signIn, refreshSession: refreshSession, signOut: signOut, getMe: getMe, deleteAccount: deleteAccount}
}

// RegisterPublicRoutes adds the routes used before there is an access token. withDevSignIn adds
// POST /auth/dev; only ever pass true when running locally.
func (h *AuthController) RegisterPublicRoutes(r gin.IRouter, withDevSignIn bool) {
	r.POST("/auth/apple", h.SignInWithApple)
	r.POST("/auth/google", h.SignInWithGoogle)
	if withDevSignIn {
		r.POST("/auth/dev", h.SignInForDev)
	}
	r.POST("/auth/refresh", h.Refresh)
	r.POST("/auth/sign-out", h.SignOut)
}

// RegisterRoutes adds the account routes to r, which must sit behind httpserver.RequireOwner.
func (h *AuthController) RegisterRoutes(r gin.IRouter) {
	r.GET("/me", h.GetMe)
	r.DELETE("/me", h.DeleteMe)
}

// SignInWithApple godoc
// @Summary     Sign in with Apple
// @Description Creates the account on the first sign-in. Send the name Apple shows on that first sign-in; Apple never repeats it.
// @Tags        auth
// @Param       payload body appleSignInRequest true "Apple identity token"
// @Success     200 {object} signInResponse
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Router      /auth/apple [post]
func (h *AuthController) SignInWithApple(ctx *gin.Context) {
	var request appleSignInRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	h.respondSignIn(ctx, signin.Command{Provider: model.ProviderApple, IDToken: request.IDToken, Name: request.Name})
}

// SignInWithGoogle godoc
// @Summary     Sign in with Google
// @Description Creates the account on the first sign-in.
// @Tags        auth
// @Param       payload body googleSignInRequest true "Google ID token"
// @Success     200 {object} signInResponse
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Router      /auth/google [post]
func (h *AuthController) SignInWithGoogle(ctx *gin.Context) {
	var request googleSignInRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	h.respondSignIn(ctx, signin.Command{Provider: model.ProviderGoogle, IDToken: request.IDToken})
}

// SignInForDev godoc
// @Summary     Sign in without Apple or Google (local only)
// @Description Only exists when running locally, for trying the API from Swagger. The same name signs into the same test account.
// @Tags        auth
// @Param       payload body devSignInRequest true "Test account name"
// @Success     200 {object} signInResponse
// @Failure     400 {object} map[string]any
// @Router      /auth/dev [post]
func (h *AuthController) SignInForDev(ctx *gin.Context) {
	var request devSignInRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	h.respondSignIn(ctx, signin.Command{Provider: model.ProviderDev, IDToken: request.Name})
}

func (h *AuthController) respondSignIn(ctx *gin.Context, command signin.Command) {
	result, err := h.signIn.Execute(ctx.Request.Context(), command)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, signInResponse{
		tokensDTO:  toTokensDTO(result.Tokens),
		User:       toUserDTO(result.User),
		NewAccount: result.Created,
	})
}

// Refresh godoc
// @Summary     Get new tokens
// @Description Trades a refresh token for a new access token and a new refresh token. Each refresh token works once; keep the new one.
// @Tags        auth
// @Param       payload body refreshTokenRequest true "Refresh token"
// @Success     200 {object} tokensDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Router      /auth/refresh [post]
func (h *AuthController) Refresh(ctx *gin.Context) {
	var request refreshTokenRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	tokens, err := h.refreshSession.Execute(ctx.Request.Context(), refreshsession.Command{RefreshToken: request.RefreshToken})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toTokensDTO(tokens))
}

// SignOut godoc
// @Summary     Sign out
// @Description Ends the session the refresh token belongs to. Succeeds for an already-ended session too.
// @Tags        auth
// @Param       payload body refreshTokenRequest true "Refresh token"
// @Success     204
// @Failure     400 {object} map[string]any
// @Router      /auth/sign-out [post]
func (h *AuthController) SignOut(ctx *gin.Context) {
	var request refreshTokenRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	if err := h.signOut.Execute(ctx.Request.Context(), signout.Command{RefreshToken: request.RefreshToken}); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// GetMe godoc
// @Summary     The signed-in account
// @Tags        account
// @Security    BearerAuth
// @Success     200 {object} userDTO
// @Failure     401 {object} map[string]any
// @Router      /me [get]
func (h *AuthController) GetMe(ctx *gin.Context) {
	user, err := h.getMe.Execute(ctx.Request.Context(), getme.Query{Owner: httpserver.OwnerOf(ctx)})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toUserDTO(user))
}

// DeleteMe godoc
// @Summary     Delete the account
// @Description Deletes the account and everything saved with it, and ends all its sessions. It can't be undone.
// @Tags        account
// @Security    BearerAuth
// @Success     204
// @Failure     401 {object} map[string]any
// @Router      /me [delete]
func (h *AuthController) DeleteMe(ctx *gin.Context) {
	if err := h.deleteAccount.Execute(ctx.Request.Context(), deleteaccount.Command{Owner: httpserver.OwnerOf(ctx)}); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}
