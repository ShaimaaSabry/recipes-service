// Package api is the recipes module's HTTP entrypoint.
package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/recipes/application/attachphoto"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/deleterecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/editrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getrecipes"
	"github.com/mamienellie/recipes-service/internal/recipes/application/removephoto"
	"github.com/mamienellie/recipes-service/internal/recipes/application/requestphotoupload"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

type (
	getRecipesInteractor interface {
		Execute(ctx context.Context, query getrecipes.Query) ([]*model.Recipe, error)
	}
	getRecipeInteractor interface {
		Execute(ctx context.Context, query getrecipe.Query) (getrecipe.Result, error)
	}
	createRecipeInteractor interface {
		Execute(ctx context.Context, command createrecipe.Command) (*model.Recipe, error)
	}
	editRecipeInteractor interface {
		Execute(ctx context.Context, command editrecipe.Command) (*model.Recipe, error)
	}
	deleteRecipeInteractor interface {
		Execute(ctx context.Context, command deleterecipe.Command) error
	}
	requestPhotoUploadInteractor interface {
		Execute(ctx context.Context, command requestphotoupload.Command) (model.PhotoUpload, error)
	}
	attachPhotoInteractor interface {
		Execute(ctx context.Context, command attachphoto.Command) (*model.Recipe, error)
	}
	removePhotoInteractor interface {
		Execute(ctx context.Context, command removephoto.Command) (*model.Recipe, error)
	}
)

// RecipeInteractors are the use cases behind the recipe routes.
type RecipeInteractors struct {
	GetRecipes         getRecipesInteractor
	GetRecipe          getRecipeInteractor
	CreateRecipe       createRecipeInteractor
	EditRecipe         editRecipeInteractor
	DeleteRecipe       deleteRecipeInteractor
	RequestPhotoUpload requestPhotoUploadInteractor
	AttachPhoto        attachPhotoInteractor
	RemovePhoto        removePhotoInteractor
}

type RecipeController struct {
	use RecipeInteractors
	// photosURL is the CloudFront address photos are served from; empty without photos.
	photosURL string
}

func NewRecipeController(interactors RecipeInteractors, photosURL string) *RecipeController {
	return &RecipeController{use: interactors, photosURL: photosURL}
}

// RegisterRoutes adds the recipe routes to r, which must sit behind httpserver.RequireOwner.
func (h *RecipeController) RegisterRoutes(r gin.IRouter) {
	r.GET("/recipes", h.GetRecipes)
	r.POST("/recipes", h.CreateRecipe)
	r.GET("/recipes/:id", h.GetRecipe)
	r.PUT("/recipes/:id", h.EditRecipe)
	r.DELETE("/recipes/:id", h.DeleteRecipe)
	r.POST("/recipes/:id/photo/upload", h.RequestPhotoUpload)
	r.PUT("/recipes/:id/photo", h.AttachPhoto)
	r.DELETE("/recipes/:id/photo", h.RemovePhoto)
}

func (h *RecipeController) dto(recipe *model.Recipe) recipeDTO {
	return toRecipeDTO(recipe, recipe.Servings(), recipe.Ingredients(), h.photosURL)
}

// GetRecipes godoc
// @Summary     List recipes
// @Description By name, each with its ingredients and steps.
// @Tags        recipes
// @Security    BearerAuth
// @Success     200 {array}  recipeDTO
// @Failure     401 {object} map[string]any
// @Router      /recipes [get]
func (h *RecipeController) GetRecipes(ctx *gin.Context) {
	recipes, err := h.use.GetRecipes.Execute(ctx.Request.Context(), getrecipes.Query{Owner: httpserver.OwnerOf(ctx)})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	dtos := make([]recipeDTO, 0, len(recipes))
	for _, recipe := range recipes {
		dtos = append(dtos, h.dto(recipe))
	}
	ctx.JSON(http.StatusOK, dtos)
}

// GetRecipe godoc
// @Summary     Get a recipe
// @Description Pass servings to get the ingredient amounts for that many people.
// @Tags        recipes
// @Security    BearerAuth
// @Param       id       path  string true  "Recipe ID"
// @Param       servings query int    false "Scale the ingredients to this many people (1 to 99)"
// @Success     200 {object} recipeDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Router      /recipes/{id} [get]
func (h *RecipeController) GetRecipe(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrRecipeNotFound)
	if !ok {
		return
	}
	query := getrecipe.Query{Owner: httpserver.OwnerOf(ctx), ID: id}
	if raw := ctx.Query("servings"); raw != "" {
		servings, err := strconv.Atoi(raw)
		if err != nil {
			_ = ctx.Error(model.ErrInvalidServings)
			return
		}
		query.Servings = &servings
	}
	result, err := h.use.GetRecipe.Execute(ctx.Request.Context(), query)
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, toRecipeDTO(result.Recipe, result.Servings, result.Ingredients, h.photosURL))
}

// CreateRecipe godoc
// @Summary     Create a recipe
// @Description Each ingredient is one of the account's grocery items, in a unit it's cooked in (or,
// @Description for an item nobody cooks with, one it's bought in).
// @Tags        recipes
// @Security    BearerAuth
// @Param       payload body recipeRequest true "The recipe"
// @Success     201 {object} recipeDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Router      /recipes [post]
func (h *RecipeController) CreateRecipe(ctx *gin.Context) {
	var request recipeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	recipe, err := h.use.CreateRecipe.Execute(ctx.Request.Context(), createrecipe.Command{
		Owner: httpserver.OwnerOf(ctx), Details: toRecipeDetails(request),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusCreated, h.dto(recipe))
}

// EditRecipe godoc
// @Summary     Change a recipe
// @Description Replaces its name, servings, ingredients and steps. The photo stays.
// @Tags        recipes
// @Security    BearerAuth
// @Param       id      path string        true "Recipe ID"
// @Param       payload body recipeRequest true "The recipe"
// @Success     200 {object} recipeDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Router      /recipes/{id} [put]
func (h *RecipeController) EditRecipe(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrRecipeNotFound)
	if !ok {
		return
	}
	var request recipeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	recipe, err := h.use.EditRecipe.Execute(ctx.Request.Context(), editrecipe.Command{
		Owner: httpserver.OwnerOf(ctx), ID: id, Details: toRecipeDetails(request),
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, h.dto(recipe))
}

// DeleteRecipe godoc
// @Summary     Delete a recipe
// @Description Its planned meals and photo go with it. What it added to the grocery list stays.
// @Tags        recipes
// @Security    BearerAuth
// @Param       id path string true "Recipe ID"
// @Success     204
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Router      /recipes/{id} [delete]
func (h *RecipeController) DeleteRecipe(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrRecipeNotFound)
	if !ok {
		return
	}
	if err := h.use.DeleteRecipe.Execute(ctx.Request.Context(), deleterecipe.Command{Owner: httpserver.OwnerOf(ctx), ID: id}); err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.Status(http.StatusNoContent)
}

// RequestPhotoUpload godoc
// @Summary     Get a link to upload a recipe photo to
// @Description Upload the image itself with the returned method, URL and headers, within 15
// @Description minutes, then attach it with PUT /recipes/{id}/photo. Photos up to 10 MB.
// @Tags        recipes
// @Security    BearerAuth
// @Param       id      path string             true "Recipe ID"
// @Param       payload body photoUploadRequest true "The photo's type"
// @Success     200 {object} photoUploadDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Failure     503 {object} map[string]any "Photos aren't set up where the service runs"
// @Router      /recipes/{id}/photo/upload [post]
func (h *RecipeController) RequestPhotoUpload(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrRecipeNotFound)
	if !ok {
		return
	}
	var request photoUploadRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	upload, err := h.use.RequestPhotoUpload.Execute(ctx.Request.Context(), requestphotoupload.Command{
		Owner: httpserver.OwnerOf(ctx), RecipeID: id, ContentType: request.ContentType,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, photoUploadDTO{
		PhotoKey: upload.Key, UploadURL: upload.URL, Method: http.MethodPut, Headers: upload.Headers, ExpiresAt: upload.ExpiresAt,
	})
}

// AttachPhoto godoc
// @Summary     Set a recipe's photo to one just uploaded
// @Description The previous photo is deleted.
// @Tags        recipes
// @Security    BearerAuth
// @Param       id      path string             true "Recipe ID"
// @Param       payload body attachPhotoRequest true "The photoKey from the upload link"
// @Success     200 {object} recipeDTO
// @Failure     400 {object} map[string]any "Not uploaded, too large, or not an image"
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Failure     503 {object} map[string]any "Photos aren't set up where the service runs"
// @Router      /recipes/{id}/photo [put]
func (h *RecipeController) AttachPhoto(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrRecipeNotFound)
	if !ok {
		return
	}
	var request attachPhotoRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}
	recipe, err := h.use.AttachPhoto.Execute(ctx.Request.Context(), attachphoto.Command{
		Owner: httpserver.OwnerOf(ctx), RecipeID: id, PhotoKey: request.PhotoKey,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, h.dto(recipe))
}

// RemovePhoto godoc
// @Summary     Remove a recipe's photo
// @Tags        recipes
// @Security    BearerAuth
// @Param       id path string true "Recipe ID"
// @Success     200 {object} recipeDTO
// @Failure     401 {object} map[string]any
// @Failure     404 {object} map[string]any
// @Router      /recipes/{id}/photo [delete]
func (h *RecipeController) RemovePhoto(ctx *gin.Context) {
	id, ok := pathID(ctx, model.ErrRecipeNotFound)
	if !ok {
		return
	}
	recipe, err := h.use.RemovePhoto.Execute(ctx.Request.Context(), removephoto.Command{Owner: httpserver.OwnerOf(ctx), RecipeID: id})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusOK, h.dto(recipe))
}
