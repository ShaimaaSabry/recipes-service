// Package api is the recipes module's HTTP entrypoint.
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mamienellie/recipes-service/internal/platform/httpserver"
	"github.com/mamienellie/recipes-service/internal/recipes/application/createrecipe"
	"github.com/mamienellie/recipes-service/internal/recipes/application/getrecipes"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

type getRecipesInteractor interface {
	Execute(ctx context.Context, query getrecipes.Query) ([]model.Recipe, error)
}

type createRecipeInteractor interface {
	Execute(ctx context.Context, command createrecipe.Command) (*model.Recipe, error)
}

type RecipeController struct {
	getRecipesInteractor   getRecipesInteractor
	createRecipeInteractor createRecipeInteractor
}

func NewRecipeController(
	getRecipesInteractor getRecipesInteractor,
	createRecipeInteractor createRecipeInteractor,
) *RecipeController {
	return &RecipeController{
		getRecipesInteractor:   getRecipesInteractor,
		createRecipeInteractor: createRecipeInteractor,
	}
}

// RegisterRoutes adds the recipe routes to r, which must sit behind httpserver.RequireOwner.
func (h *RecipeController) RegisterRoutes(r gin.IRouter) {
	r.GET("/recipes", h.GetRecipes)
	r.POST("/recipes", h.CreateRecipe)
}

// GetRecipes godoc
// @Summary     List recipes
// @Tags        recipes
// @Security    BearerAuth
// @Success     200 {array}  recipeDTO
// @Failure     401 {object} map[string]any
// @Router      /recipes [get]
func (h *RecipeController) GetRecipes(ctx *gin.Context) {
	recipes, err := h.getRecipesInteractor.Execute(ctx.Request.Context(), getrecipes.Query{Owner: httpserver.OwnerOf(ctx)})
	if err != nil {
		_ = ctx.Error(err)
		return
	}

	dtos := make([]recipeDTO, 0, len(recipes))
	for i := range recipes {
		dtos = append(dtos, toRecipeDTO(&recipes[i]))
	}
	ctx.JSON(http.StatusOK, dtos)
}

// CreateRecipe godoc
// @Summary     Create a recipe
// @Tags        recipes
// @Security    BearerAuth
// @Param       payload body createRecipeRequest true "Create recipe payload"
// @Success     201 {object} recipeDTO
// @Failure     400 {object} map[string]any
// @Failure     401 {object} map[string]any
// @Failure     409 {object} map[string]any
// @Router      /recipes [post]
func (h *RecipeController) CreateRecipe(ctx *gin.Context) {
	var request createRecipeRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		_ = ctx.Error(err)
		return
	}

	recipe, err := h.createRecipeInteractor.Execute(ctx.Request.Context(), createrecipe.Command{
		Owner: httpserver.OwnerOf(ctx),
		ID:    request.ID,
		Name:  request.Name,
	})
	if err != nil {
		_ = ctx.Error(err)
		return
	}
	ctx.JSON(http.StatusCreated, toRecipeDTO(recipe))
}
