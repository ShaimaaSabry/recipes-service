package api

import (
	"time"

	"github.com/mamienellie/recipes-service/internal/auth/application/session"
	"github.com/mamienellie/recipes-service/internal/auth/domain/model"
)

type appleSignInRequest struct {
	// IDToken is the identityToken from Sign in with Apple.
	IDToken string `json:"idToken" binding:"required"`
	// Name is the full name Apple shows the app on the first sign-in only. Optional.
	Name string `json:"name" binding:"max=200"`
}

type googleSignInRequest struct {
	// IDToken is the idToken from Google Sign-In.
	IDToken string `json:"idToken" binding:"required"`
}

type devSignInRequest struct {
	// Name picks the local test account; the same name signs into the same account.
	Name string `json:"name" binding:"required,max=64"`
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type userDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type tokensDTO struct {
	AccessToken           string    `json:"accessToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshToken          string    `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

type signInResponse struct {
	tokensDTO
	User userDTO `json:"user"`
	// NewAccount is true when this sign-in created the account, e.g. to show onboarding.
	NewAccount bool `json:"newAccount"`
}

func toUserDTO(user *model.User) userDTO {
	return userDTO{ID: user.ID().String(), Email: user.Email(), Name: user.Name()}
}

func toTokensDTO(tokens session.Tokens) tokensDTO {
	return tokensDTO{
		AccessToken:           tokens.AccessToken,
		AccessTokenExpiresAt:  tokens.AccessTokenExpiresAt.UTC(),
		RefreshToken:          tokens.RefreshToken,
		RefreshTokenExpiresAt: tokens.RefreshTokenExpiresAt.UTC(),
	}
}
