package api

import "github.com/mamienellie/recipes-service/internal/recipes/domain/model"

type createRecipeRequest struct {
	// ID is optional; the app sends the one it created, possibly while offline.
	ID   string `json:"id" binding:"max=64"`
	Name string `json:"name" binding:"required"`
}

type recipeDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func toRecipeDTO(recipe *model.Recipe) recipeDTO {
	return recipeDTO{ID: recipe.ID(), Name: recipe.Name()}
}
