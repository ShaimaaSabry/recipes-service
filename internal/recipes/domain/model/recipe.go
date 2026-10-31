// Package model holds the recipes module's entities and the rules that keep them valid.
package model

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

const (
	MinServings       = 1
	MaxServings       = 99
	MaxIngredients    = 100
	MaxSteps          = 100
	MaxStepLength     = 2000
	quantityPrecision = 1e6
)

var (
	ErrInvalidRecipeName = fmt.Errorf("%w: please give the recipe a name", apperrors.ErrInvalidInput)
	ErrInvalidServings   = fmt.Errorf("%w: servings must be %d to %d", apperrors.ErrInvalidInput, MinServings, MaxServings)
	ErrRecipeNotFound    = fmt.Errorf("%w: no such recipe", apperrors.ErrNotFound)
)

// Ingredient is an amount of one of the owner's grocery items, in a unit it's cooked in.
type Ingredient struct {
	ItemID   uuid.UUID
	UnitID   uuid.UUID
	Quantity float64
}

// RecipeDetails is what the owner fills in for a recipe.
type RecipeDetails struct {
	Name string
	// Servings is how many people the ingredient amounts are for.
	Servings    int
	Ingredients []Ingredient
	Steps       []string
}

// Recipe is a dish the owner cooks.
type Recipe struct {
	id       uuid.UUID
	owner    owner.ID
	details  RecipeDetails
	photoKey string
}

// NewRecipe validates a new recipe. items are the owner's grocery items, which its ingredients
// must be among.
func NewRecipe(id uuid.UUID, owner owner.ID, details RecipeDetails, items map[uuid.UUID]*GroceryItem, units Units) (*Recipe, error) {
	recipe := &Recipe{id: id, owner: owner}
	if err := recipe.Edit(details, items, units); err != nil {
		return nil, err
	}
	return recipe, nil
}

// RecipeOf rebuilds a recipe that was already validated, e.g. one read back from the database.
func RecipeOf(id uuid.UUID, owner owner.ID, details RecipeDetails, photoKey string) *Recipe {
	return &Recipe{id: id, owner: owner, details: details, photoKey: photoKey}
}

func (r Recipe) ID() uuid.UUID                { return r.id }
func (r Recipe) Owner() owner.ID              { return r.owner }
func (r Recipe) Name() string                 { return r.details.Name }
func (r Recipe) Servings() int                { return r.details.Servings }
func (r Recipe) Ingredients() []Ingredient    { return r.details.Ingredients }
func (r Recipe) Steps() []string              { return r.details.Steps }
func (r Recipe) PhotoKey() string             { return r.photoKey }
func (r *Recipe) SetPhotoKey(photoKey string) { r.photoKey = photoKey }

// Edit replaces what the owner fills in. The name is tidied; blank steps are dropped.
func (r *Recipe) Edit(details RecipeDetails, items map[uuid.UUID]*GroceryItem, units Units) error {
	details.Name = normalizeName(details.Name)
	if details.Name == "" {
		return ErrInvalidRecipeName
	}
	if utf8.RuneCountInString(details.Name) > MaxNameLength {
		return fmt.Errorf("%w: the recipe's name must be at most %d characters", apperrors.ErrInvalidInput, MaxNameLength)
	}
	if err := ValidateServings(details.Servings); err != nil {
		return err
	}
	if len(details.Ingredients) > MaxIngredients {
		return fmt.Errorf("%w: a recipe can have at most %d ingredients", apperrors.ErrInvalidInput, MaxIngredients)
	}
	for _, ingredient := range details.Ingredients {
		if err := validateIngredient(ingredient, items, units); err != nil {
			return err
		}
	}

	steps := make([]string, 0, len(details.Steps))
	for _, step := range details.Steps {
		if step = strings.TrimSpace(step); step != "" {
			steps = append(steps, step)
		}
	}
	if len(steps) > MaxSteps {
		return fmt.Errorf("%w: a recipe can have at most %d steps", apperrors.ErrInvalidInput, MaxSteps)
	}
	for _, step := range steps {
		if utf8.RuneCountInString(step) > MaxStepLength {
			return fmt.Errorf("%w: each step must be at most %d characters", apperrors.ErrInvalidInput, MaxStepLength)
		}
	}
	details.Steps = steps
	r.details = details
	return nil
}

func validateIngredient(ingredient Ingredient, items map[uuid.UUID]*GroceryItem, units Units) error {
	item, ok := items[ingredient.ItemID]
	if !ok {
		return fmt.Errorf("%w: %s is not one of your grocery items", apperrors.ErrInvalidInput, ingredient.ItemID)
	}
	if !(ingredient.Quantity > 0) || math.IsInf(ingredient.Quantity, 0) {
		return fmt.Errorf("%w: %s needs an amount more than 0", apperrors.ErrInvalidInput, item.Name())
	}
	allowed := item.RecipeUnitIDs()
	if !slices.Contains(allowed, ingredient.UnitID) {
		names := make([]string, 0, len(allowed))
		for _, id := range allowed {
			names = append(names, units.name(id))
		}
		return fmt.Errorf("%w: %s isn't measured in %s in recipes; use %s",
			apperrors.ErrInvalidInput, item.Name(), units.name(ingredient.UnitID), strings.Join(names, " or "))
	}
	return nil
}

// ValidateServings checks a number of people is one a recipe or meal can be for.
func ValidateServings(servings int) error {
	if servings < MinServings || servings > MaxServings {
		return ErrInvalidServings
	}
	return nil
}

// IngredientsFor returns the ingredients scaled from the recipe's servings to servings.
func (r Recipe) IngredientsFor(servings int) []Ingredient {
	if servings == r.details.Servings {
		return r.details.Ingredients
	}
	scaled := make([]Ingredient, 0, len(r.details.Ingredients))
	for _, ingredient := range r.details.Ingredients {
		ingredient.Quantity = RoundQuantity(ingredient.Quantity * float64(servings) / float64(r.details.Servings))
		scaled = append(scaled, ingredient)
	}
	return scaled
}

// RoundQuantity rounds away the noise conversions leave (0.30000000000000004 → 0.3), to the
// millionth, like the app.
func RoundQuantity(quantity float64) float64 {
	return math.Round(quantity*quantityPrecision) / quantityPrecision
}
