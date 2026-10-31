package api

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/recipes/application/applygroceryactions"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

type ingredientDTO struct {
	ItemID   string  `json:"itemId" binding:"required,uuid"`
	UnitID   string  `json:"unitId" binding:"required,uuid"`
	Quantity float64 `json:"quantity" binding:"required,gt=0" example:"2"`
}

type recipeRequest struct {
	Name string `json:"name" binding:"required" example:"Shakshuka"`
	// Servings is how many people the ingredient amounts are for.
	Servings    int             `json:"servings" binding:"required,min=1,max=99" example:"2"`
	Ingredients []ingredientDTO `json:"ingredients" binding:"dive"`
	Steps       []string        `json:"steps" example:"Warm the olive oil in a wide pan."`
}

type recipeDTO struct {
	ID       string `json:"id"`
	Name     string `json:"name" example:"Shakshuka"`
	Servings int    `json:"servings" example:"2"`
	// PhotoURL is where the photo is served from, or null.
	PhotoURL    *string         `json:"photoUrl"`
	Ingredients []ingredientDTO `json:"ingredients"`
	Steps       []string        `json:"steps"`
}

type photoUploadRequest struct {
	ContentType string `json:"contentType" binding:"required,oneof=image/jpeg image/png image/heic image/webp" example:"image/jpeg"`
}

type photoUploadDTO struct {
	// PhotoKey is what to attach to the recipe once the upload is done.
	PhotoKey string `json:"photoKey"`
	// Upload the image with this method, URL and headers (the body is the image itself).
	UploadURL string            `json:"uploadUrl"`
	Method    string            `json:"method" example:"PUT"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type attachPhotoRequest struct {
	PhotoKey string `json:"photoKey" binding:"required"`
}

type planMealRequest struct {
	RecipeID string `json:"recipeId" binding:"required,uuid"`
	Servings int    `json:"servings" binding:"required,min=1,max=99" example:"4"`
}

type mealDTO struct {
	ID        string    `json:"id"`
	RecipeID  string    `json:"recipeId"`
	Servings  int       `json:"servings" example:"4"`
	PlannedAt time.Time `json:"plannedAt"`
}

type groceryLineDTO struct {
	ItemID string `json:"itemId"`
	UnitID string `json:"unitId"`
	// Quantity is exact (0.04 after 2 tbsp of oil); BuyQuantity is what to show, rounded up to
	// whole numbers for units sold whole (1 bottle).
	Quantity    float64 `json:"quantity" example:"0.04"`
	BuyQuantity float64 `json:"buyQuantity" example:"1"`
	Checked     bool    `json:"checked"`
}

type groceryListDTO struct {
	Lines []groceryLineDTO `json:"lines"`
}

type groceryActionDTO struct {
	// ID is made by the app, once per action: resending the same action is then harmless.
	ID   string `json:"id" binding:"required,uuid"`
	Type string `json:"type" binding:"required,oneof=add setQuantity setUnit check uncheck remove clearChecked" example:"add"`
	// ItemID names the line, for every type but clearChecked.
	ItemID string `json:"itemId" binding:"omitempty,uuid"`
	// UnitID is the amount's unit for add, and the new unit for setUnit.
	UnitID string `json:"unitId" binding:"omitempty,uuid"`
	// Quantity is the amount for add and setQuantity.
	Quantity float64 `json:"quantity" example:"2"`
}

type groceryActionsRequest struct {
	Actions []groceryActionDTO `json:"actions" binding:"required,max=200,dive"`
}

type missingConversionDTO struct {
	ItemID string `json:"itemId"`
	// Ask the user "1 fromUnit = ? toUnit", then save it with POST /items/{id}/conversions.
	FromUnitID string `json:"fromUnitId"`
	ToUnitID   string `json:"toUnitId"`
}

type groceryActionResultDTO struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome" enums:"applied,alreadyApplied,rejected"`
	// Error says why a rejected action was refused.
	Error *string `json:"error,omitempty"`
	// MissingConversion is set when the action needs a conversion the item doesn't have yet.
	MissingConversion *missingConversionDTO `json:"missingConversion,omitempty"`
}

type groceryActionsDTO struct {
	Results []groceryActionResultDTO `json:"results"`
	// Lines is the list after the actions; it replaces the app's preview.
	Lines []groceryLineDTO `json:"lines"`
}

func toRecipeDTO(recipe *model.Recipe, servings int, ingredients []model.Ingredient, photosURL string) recipeDTO {
	dtos := make([]ingredientDTO, 0, len(ingredients))
	for _, i := range ingredients {
		dtos = append(dtos, ingredientDTO{ItemID: i.ItemID.String(), UnitID: i.UnitID.String(), Quantity: i.Quantity})
	}
	return recipeDTO{
		ID: recipe.ID().String(), Name: recipe.Name(), Servings: servings,
		PhotoURL: photoURL(photosURL, recipe.PhotoKey()), Ingredients: dtos, Steps: orEmpty(recipe.Steps()),
	}
}

func photoURL(base, key string) *string {
	if base == "" || key == "" {
		return nil
	}
	url := strings.TrimRight(base, "/") + "/" + key
	return &url
}

func toRecipeDetails(request recipeRequest) model.RecipeDetails {
	ingredients := make([]model.Ingredient, 0, len(request.Ingredients))
	for _, i := range request.Ingredients {
		ingredients = append(ingredients, model.Ingredient{ItemID: uuid.MustParse(i.ItemID), UnitID: uuid.MustParse(i.UnitID), Quantity: i.Quantity})
	}
	return model.RecipeDetails{Name: request.Name, Servings: request.Servings, Ingredients: ingredients, Steps: request.Steps}
}

func toMealDTO(meal *model.PlannedMeal) mealDTO {
	return mealDTO{ID: meal.ID().String(), RecipeID: meal.RecipeID().String(), Servings: meal.Servings(), PlannedAt: meal.CreatedAt()}
}

func toGroceryLineDTOs(list model.GroceryList) []groceryLineDTO {
	lines := make([]groceryLineDTO, 0, len(list.Lines))
	for _, l := range list.Lines {
		lines = append(lines, groceryLineDTO{
			ItemID: l.ItemID().String(), UnitID: l.UnitID().String(),
			Quantity: l.Quantity(), BuyQuantity: l.BuyQuantity(list.Units), Checked: l.Checked(),
		})
	}
	return lines
}

func toGroceryActions(dtos []groceryActionDTO) []applygroceryactions.Action {
	actions := make([]applygroceryactions.Action, 0, len(dtos))
	for _, a := range dtos {
		actions = append(actions, applygroceryactions.Action{
			ID: uuid.MustParse(a.ID), Type: applygroceryactions.ActionType(a.Type),
			ItemID: parseOptionalID(a.ItemID), UnitID: parseOptionalID(a.UnitID), Quantity: a.Quantity,
		})
	}
	return actions
}

// parseOptionalID parses an ID the binding tags already checked, or returns uuid.Nil for none.
func parseOptionalID(id string) uuid.UUID {
	if id == "" {
		return uuid.Nil
	}
	return uuid.MustParse(id)
}

func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}
