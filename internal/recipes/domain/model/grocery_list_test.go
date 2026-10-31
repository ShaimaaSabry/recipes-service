package model

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// realUnits are the migration's units: sized ones with their base, and sold whole as there.
func realUnits() (Units, map[string]uuid.UUID) {
	type spec struct {
		code, base string
		size       float64
		whole      bool
	}
	specs := []spec{
		{"piece", "piece", 1, true}, {"dozen", "piece", 12, true}, {"kg", "g", 1000, false}, {"g", "g", 1, false},
		{"l", "ml", 1000, false}, {"ml", "ml", 1, false}, {"pack", "", 0, true}, {"box", "", 0, true},
		{"bottle", "", 0, true}, {"can", "", 0, true}, {"cup", "ml", 240, false}, {"tbsp", "ml", 15, false},
		{"tsp", "ml", 5, false}, {"head", "", 0, true}, {"clove", "", 0, true},
	}
	id := map[string]uuid.UUID{}
	for _, s := range specs {
		id[s.code] = uuid.New()
	}
	list := make([]Unit, 0, len(specs))
	for _, s := range specs {
		list = append(list, UnitOf(id[s.code], s.code, s.code, id[s.base], s.size, s.whole))
	}
	return NewUnits(list), id
}

func item(t *testing.T, units Units, name string, buy, cook []uuid.UUID, conversions ...Conversion) *GroceryItem {
	t.Helper()
	details := ItemDetails{Name: name, Emoji: "🛒", CategoryID: uuid.New(), BuyUnitIDs: buy, CookUnitIDs: cook}
	item, err := NewGroceryItem(uuid.New(), "owner-1", details, conversions, units)
	require.NoError(t, err)
	return item
}

func TestAddToList(t *testing.T) {
	units, u := realUnits()
	oil := item(t, units, "Olive oil", []uuid.UUID{u["bottle"]}, []uuid.UUID{u["tbsp"], u["ml"]},
		Conversion{From: u["bottle"], To: u["ml"], Factor: 750})
	potatoes := item(t, units, "Potatoes", []uuid.UUID{u["kg"]}, []uuid.UUID{u["piece"], u["g"]},
		Conversion{From: u["piece"], To: u["g"], Factor: 200})
	beef := item(t, units, "Ground beef", []uuid.UUID{u["kg"], u["g"]}, []uuid.UUID{u["g"]})
	eggs := item(t, units, "Eggs", []uuid.UUID{u["dozen"], u["piece"]}, []uuid.UUID{u["piece"]})

	t.Run("a new line is in the item's first buy unit, rounded up to buy", func(t *testing.T) {
		line, err := AddToList(nil, oil, units, u["tbsp"], 2, uuid.New())

		require.NoError(t, err)
		require.Equal(t, u["bottle"], line.UnitID())
		require.InEpsilon(t, 0.04, line.Quantity(), 1e-9)
		require.InDelta(t, 1.0, line.BuyQuantity(units), 0)
	})

	t.Run("a new line keeps the amount's unit when the item is bought in it", func(t *testing.T) {
		line, err := AddToList(nil, eggs, units, u["piece"], 3, uuid.New())

		require.NoError(t, err)
		require.Equal(t, u["piece"], line.UnitID())
		require.InDelta(t, 3.0, line.Quantity(), 0)
	})

	t.Run("adding to a line still to buy converts into its unit", func(t *testing.T) {
		line, err := AddToList(nil, potatoes, units, u["g"], 300, uuid.New())
		require.NoError(t, err)

		line, err = AddToList(line, potatoes, units, u["piece"], 5, uuid.New())

		require.NoError(t, err)
		require.Equal(t, u["kg"], line.UnitID())
		require.InEpsilon(t, 1.3, line.Quantity(), 1e-9)
		require.InEpsilon(t, 1.3, line.BuyQuantity(units), 1e-9, "weights aren't rounded up")
	})

	t.Run("a bought line starts again with only the new amount", func(t *testing.T) {
		line, err := AddToList(nil, eggs, units, u["dozen"], 1, uuid.New())
		require.NoError(t, err)
		line.SetChecked(true)
		id := line.ID()

		line, err = AddToList(line, eggs, units, u["piece"], 4, uuid.New())

		require.NoError(t, err)
		require.Equal(t, id, line.ID())
		require.False(t, line.Checked())
		require.Equal(t, u["piece"], line.UnitID())
		require.InDelta(t, 4.0, line.Quantity(), 0)
	})

	t.Run("1500 g becomes 1.5 kg when the item is bought in kg", func(t *testing.T) {
		line, err := AddToList(nil, beef, units, u["g"], 700, uuid.New())
		require.NoError(t, err)
		require.Equal(t, u["g"], line.UnitID())

		line, err = AddToList(line, beef, units, u["g"], 800, uuid.New())

		require.NoError(t, err)
		require.Equal(t, u["kg"], line.UnitID())
		require.InEpsilon(t, 1.5, line.Quantity(), 1e-9)
	})

	t.Run("an amount that doesn't convert names the conversion to ask for", func(t *testing.T) {
		line, err := AddToList(nil, eggs, units, u["piece"], 3, uuid.New())
		require.NoError(t, err)

		_, err = AddToList(line, eggs, units, u["box"], 1, uuid.New())

		var missing *MissingConversionError
		require.True(t, errors.As(err, &missing))
		require.ErrorIs(t, err, apperrors.ErrConflict)
		require.Equal(t, u["box"], missing.FromUnitID, "1 box = ? piece")
		require.Equal(t, u["piece"], missing.ToUnitID)
		require.Equal(t, "Eggs: box doesn't convert to piece yet. How many piece are in 1 box?", err.Error())
	})

	t.Run("refuses an amount that isn't more than 0", func(t *testing.T) {
		_, err := AddToList(nil, eggs, units, u["piece"], 0, uuid.New())

		require.ErrorIs(t, err, ErrInvalidQuantity)
	})
}

func TestGroceryLine(t *testing.T) {
	units, u := realUnits()
	butter := item(t, units, "Butter", []uuid.UUID{u["pack"], u["kg"]}, []uuid.UUID{u["g"]},
		Conversion{From: u["pack"], To: u["g"], Factor: 200})
	line, err := AddToList(nil, butter, units, u["g"], 250, uuid.New())
	require.NoError(t, err)

	t.Run("packs are rounded up to buy", func(t *testing.T) {
		require.InEpsilon(t, 1.25, line.Quantity(), 1e-9)
		require.InDelta(t, 2.0, line.BuyQuantity(units), 0)
	})

	t.Run("changing the unit converts the amount", func(t *testing.T) {
		changed := *line

		require.NoError(t, changed.SetUnit(u["kg"], butter, units))

		require.InDelta(t, 0.25, changed.Quantity(), 1e-9)
	})

	t.Run("a line's unit must be one the item is bought in", func(t *testing.T) {
		changed := *line

		require.ErrorIs(t, changed.SetUnit(u["g"], butter, units), apperrors.ErrInvalidInput)
	})

	t.Run("setting the amount by hand replaces it", func(t *testing.T) {
		changed := *line

		require.NoError(t, changed.SetQuantity(3))
		require.InDelta(t, 3.0, changed.Quantity(), 0)
		require.ErrorIs(t, changed.SetQuantity(-1), ErrInvalidQuantity)
	})
}

func TestRecipe(t *testing.T) {
	units, u := realUnits()
	garlic := item(t, units, "Garlic", []uuid.UUID{u["head"], u["g"]}, []uuid.UUID{u["clove"], u["g"]})
	soap := item(t, units, "Dish soap", []uuid.UUID{u["bottle"]}, nil)
	items := map[uuid.UUID]*GroceryItem{garlic.ID(): garlic, soap.ID(): soap}
	soup := func() RecipeDetails {
		return RecipeDetails{
			Name: " Garlic  soup ", Servings: 2,
			Ingredients: []Ingredient{{ItemID: garlic.ID(), UnitID: u["clove"], Quantity: 3}},
			Steps:       []string{"  Peel the garlic. ", "", "Simmer."},
		}
	}

	t.Run("tidies the name and drops blank steps", func(t *testing.T) {
		recipe, err := NewRecipe(uuid.New(), "owner-1", soup(), items, units)

		require.NoError(t, err)
		require.Equal(t, "Garlic soup", recipe.Name())
		require.Equal(t, []string{"Peel the garlic.", "Simmer."}, recipe.Steps())
	})

	t.Run("scales its ingredients to a number of people", func(t *testing.T) {
		recipe, err := NewRecipe(uuid.New(), "owner-1", soup(), items, units)
		require.NoError(t, err)

		scaled := recipe.IngredientsFor(3)

		require.InDelta(t, 4.5, scaled[0].Quantity, 1e-9)
		require.InDelta(t, 3.0, recipe.Ingredients()[0].Quantity, 0, "the recipe itself is unchanged")
	})

	t.Run("an item nobody cooks with is measured in its buy units", func(t *testing.T) {
		details := soup()
		details.Ingredients = []Ingredient{{ItemID: soap.ID(), UnitID: u["bottle"], Quantity: 1}}

		_, err := NewRecipe(uuid.New(), "owner-1", details, items, units)

		require.NoError(t, err)
	})

	invalid := map[string]func(d *RecipeDetails){
		"no name":                    func(d *RecipeDetails) { d.Name = " " },
		"no servings":                func(d *RecipeDetails) { d.Servings = 0 },
		"too many servings":          func(d *RecipeDetails) { d.Servings = 100 },
		"another account's item":     func(d *RecipeDetails) { d.Ingredients[0].ItemID = uuid.New() },
		"a unit it isn't cooked in":  func(d *RecipeDetails) { d.Ingredients[0].UnitID = u["head"] },
		"an amount that isn't above": func(d *RecipeDetails) { d.Ingredients[0].Quantity = 0 },
	}
	for name, change := range invalid {
		t.Run("refuses "+name, func(t *testing.T) {
			details := soup()
			change(&details)

			_, err := NewRecipe(uuid.New(), "owner-1", details, items, units)

			require.ErrorIs(t, err, apperrors.ErrInvalidInput)
		})
	}

	t.Run("names the units to use instead", func(t *testing.T) {
		details := soup()
		details.Ingredients[0].UnitID = u["head"]

		_, err := NewRecipe(uuid.New(), "owner-1", details, items, units)

		require.ErrorContains(t, err, "Garlic isn't measured in head in recipes; use clove or g")
	})
}

type groceryListCases struct {
	Units []struct {
		Code      string  `json:"code"`
		Base      string  `json:"base"`
		BaseSize  float64 `json:"baseSize"`
		SoldWhole bool    `json:"soldWhole"`
	} `json:"units"`
	Cases []struct {
		Name string `json:"name"`
		Item struct {
			Buy         []string `json:"buy"`
			Conversions [][3]any `json:"conversions"`
		} `json:"item"`
		Steps  [][]any `json:"steps"`
		Expect struct {
			Unit        string   `json:"unit"`
			Quantity    float64  `json:"quantity"`
			BuyQuantity float64  `json:"buyQuantity"`
			Missing     []string `json:"missing"`
		} `json:"expect"`
	} `json:"cases"`
}

func TestAddToList_SharedCases(t *testing.T) {
	data, err := os.ReadFile("testdata/grocery_list_cases.json")
	require.NoError(t, err)
	var cases groceryListCases
	require.NoError(t, json.Unmarshal(data, &cases))
	id := map[string]uuid.UUID{}
	for _, u := range cases.Units {
		id[u.Code] = uuid.New()
	}
	list := make([]Unit, 0, len(cases.Units))
	for _, u := range cases.Units {
		list = append(list, UnitOf(id[u.Code], u.Code, u.Code, id[u.Base], u.BaseSize, u.SoldWhole))
	}
	units := NewUnits(list)
	codes := func(names []string) []uuid.UUID {
		out := make([]uuid.UUID, 0, len(names))
		for _, name := range names {
			require.Contains(t, id, name)
			out = append(out, id[name])
		}
		return out
	}

	for _, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			conversions := make([]Conversion, 0, len(c.Item.Conversions))
			for _, raw := range c.Item.Conversions {
				conversions = append(conversions, Conversion{From: id[raw[0].(string)], To: id[raw[1].(string)], Factor: raw[2].(float64)})
			}
			groceryItem := item(t, units, "Item", codes(c.Item.Buy), nil, conversions...)

			var line *GroceryLine
			var err error
			for _, step := range c.Steps {
				switch step[0] {
				case "add":
					line, err = AddToList(line, groceryItem, units, id[step[2].(string)], step[1].(float64), uuid.New())
				case "check":
					line.SetChecked(true)
				default:
					t.Fatalf("unknown step %v", step[0])
				}
				if err != nil {
					break
				}
			}

			if c.Expect.Missing != nil {
				var missing *MissingConversionError
				require.True(t, errors.As(err, &missing), "expected a missing conversion, got %v", err)
				require.Equal(t, codes(c.Expect.Missing), []uuid.UUID{missing.FromUnitID, missing.ToUnitID})
				return
			}
			require.NoError(t, err)
			require.Equal(t, id[c.Expect.Unit], line.UnitID())
			require.InDelta(t, c.Expect.Quantity, line.Quantity(), 1e-9)
			require.InDelta(t, c.Expect.BuyQuantity, line.BuyQuantity(units), 1e-9)
		})
	}
}
