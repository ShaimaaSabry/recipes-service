package api

import (
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
)

// importRequest carries the app's stored data as it is, in the shape of its data version 4.
type importRequest struct {
	// ImportID is made by the app once and kept until the import is done, so a resend is
	// recognised.
	ImportID string       `json:"importId" binding:"required,uuid"`
	Data     localDataDTO `json:"data" binding:"required"`
}

type localDataDTO struct {
	// Version is the app's data version; the app migrates its data to 4 before sending it.
	Version     int                `json:"version" binding:"required,oneof=4" example:"4"`
	Categories  []localCategoryDTO `json:"categories" binding:"max=1000"`
	Units       []localUnitDTO     `json:"units" binding:"max=1000"`
	Items       []localItemDTO     `json:"items" binding:"max=5000"`
	Recipes     []localRecipeDTO   `json:"recipes" binding:"max=5000"`
	Meals       []localMealDTO     `json:"plannedMeals" binding:"max=1000"`
	GroceryList []localLineDTO     `json:"groceryList" binding:"max=5000"`
}

type localCategoryDTO struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Emoji  string `json:"emoji"`
	Source string `json:"source" enums:"system,user"`
	Hidden bool   `json:"hidden"`
}

type localUnitDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type localItemDTO struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Emoji       string               `json:"emoji"`
	CategoryID  string               `json:"categoryId"`
	BuyUnitIDs  []string             `json:"buyUnitIds"`
	CookUnitIDs []string             `json:"cookUnitIds"`
	Conversions []localConversionDTO `json:"conversions"`
	Source      string               `json:"source" enums:"system,user"`
	Hidden      bool                 `json:"hidden"`
}

type localConversionDTO struct {
	FromUnitID string  `json:"fromUnitId"`
	ToUnitID   string  `json:"toUnitId"`
	Factor     float64 `json:"factor"`
}

type localRecipeDTO struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Servings    int                  `json:"servings"`
	Ingredients []localIngredientDTO `json:"ingredients"`
	Steps       []string             `json:"steps"`
}

type localIngredientDTO struct {
	ItemID   string  `json:"itemId"`
	UnitID   string  `json:"unitId"`
	Quantity float64 `json:"quantity"`
}

type localMealDTO struct {
	RecipeID string `json:"recipeId"`
	Servings int    `json:"servings"`
	Cooked   bool   `json:"cooked"`
}

type localLineDTO struct {
	ItemID   string  `json:"itemId"`
	UnitID   string  `json:"unitId"`
	Quantity float64 `json:"quantity"`
	Checked  bool    `json:"checked"`
}

type importDTO struct {
	// Imported is false when this import was done before; the result is the first one's.
	Imported bool `json:"imported"`
	// RecipeIDs maps each local recipe ID to its new ID: upload each recipe's photo to it.
	RecipeIDs map[string]string `json:"recipeIds"`
	// Notes say what the import had to change or leave out, to show the user.
	Notes []string `json:"notes"`
}

func toLocalData(dto localDataDTO) model.LocalData {
	data := model.LocalData{}
	for _, c := range dto.Categories {
		data.Categories = append(data.Categories, model.LocalCategory{
			ID: c.ID, Name: c.Name, Emoji: c.Emoji, Source: localSource(c.Source), Hidden: c.Hidden,
		})
	}
	for _, u := range dto.Units {
		data.Units = append(data.Units, model.LocalUnit{ID: u.ID, Name: u.Name})
	}
	for _, i := range dto.Items {
		conversions := make([]model.LocalConversion, 0, len(i.Conversions))
		for _, c := range i.Conversions {
			conversions = append(conversions, model.LocalConversion{FromUnitID: c.FromUnitID, ToUnitID: c.ToUnitID, Factor: c.Factor})
		}
		data.Items = append(data.Items, model.LocalItem{
			ID: i.ID, Name: i.Name, Emoji: i.Emoji, CategoryID: i.CategoryID,
			BuyUnitIDs: i.BuyUnitIDs, CookUnitIDs: i.CookUnitIDs, Conversions: conversions,
			Source: localSource(i.Source), Hidden: i.Hidden,
		})
	}
	for _, r := range dto.Recipes {
		ingredients := make([]model.LocalIngredient, 0, len(r.Ingredients))
		for _, i := range r.Ingredients {
			ingredients = append(ingredients, model.LocalIngredient{ItemID: i.ItemID, UnitID: i.UnitID, Quantity: i.Quantity})
		}
		data.Recipes = append(data.Recipes, model.LocalRecipe{
			ID: r.ID, Name: r.Name, Servings: r.Servings, Ingredients: ingredients, Steps: r.Steps,
		})
	}
	for _, m := range dto.Meals {
		data.Meals = append(data.Meals, model.LocalMeal{RecipeID: m.RecipeID, Servings: m.Servings, Cooked: m.Cooked})
	}
	for _, l := range dto.GroceryList {
		data.GroceryList = append(data.GroceryList, model.LocalLine{ItemID: l.ItemID, UnitID: l.UnitID, Quantity: l.Quantity, Checked: l.Checked})
	}
	return data
}

// localSource reads the app's source; a missing one means the user's, as in the app.
func localSource(source string) model.Source {
	if source == string(model.SourceSystem) {
		return model.SourceSystem
	}
	return model.SourceUser
}

func toImportDTO(result model.ImportResult, imported bool) importDTO {
	ids := make(map[string]string, len(result.RecipeIDs))
	for local, id := range result.RecipeIDs {
		ids[local] = id.String()
	}
	return importDTO{Imported: imported, RecipeIDs: ids, Notes: orEmpty(result.Notes)}
}
