package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

const importer owner.ID = "owner-1"

// importState is a fresh account: its copies of the starter Produce and Pantry categories, and
// starter Garlic (head, clove) and Olive oil (bottle, tbsp).
func importState(t *testing.T) (ImportState, map[string]uuid.UUID) {
	t.Helper()
	units, u := realUnits()
	produce, err := NewStarterCategory(uuid.New(), importer, "produce", "Produce", "🥕")
	require.NoError(t, err)
	pantry, err := NewStarterCategory(uuid.New(), importer, "pantry", "Pantry", "🫙")
	require.NoError(t, err)
	garlic, err := NewStarterGroceryItem(uuid.New(), importer, "garlic", ItemDetails{
		Name: "Garlic", Emoji: "🧄", CategoryID: produce.ID(),
		BuyUnitIDs: []uuid.UUID{u["head"]}, CookUnitIDs: []uuid.UUID{u["clove"]},
	}, []Conversion{{From: u["head"], To: u["clove"], Factor: 10}}, units)
	require.NoError(t, err)
	oil, err := NewStarterGroceryItem(uuid.New(), importer, "olive-oil", ItemDetails{
		Name: "Olive oil", Emoji: "🫒", CategoryID: pantry.ID(),
		BuyUnitIDs: []uuid.UUID{u["bottle"]}, CookUnitIDs: []uuid.UUID{u["tbsp"]},
	}, []Conversion{{From: u["bottle"], To: u["ml"], Factor: 750}}, units)
	require.NoError(t, err)
	return ImportState{
		Units: units, Categories: []*Category{produce, pantry}, Items: []*GroceryItem{garlic, oil},
		Usage: map[uuid.UUID]ItemUsage{},
	}, u
}

// localData is the app's data: the starter records, a renamed and hidden starter item, an item
// and a category of the user's own, a recipe, two meals and a list.
func localData() LocalData {
	return LocalData{
		Categories: []LocalCategory{
			{ID: "cat-produce", Name: "Veg", Emoji: "🥦", Source: SourceSystem},
			{ID: "cat-pantry", Name: "Pantry", Emoji: "🫙", Source: SourceSystem, Hidden: true},
			{ID: "c1", Name: "Baking", Emoji: "🧁", Source: SourceUser},
		},
		Units: []LocalUnit{
			{ID: "unit-head", Name: "head"}, {ID: "unit-clove", Name: "clove"}, {ID: "unit-bottle", Name: "bottle"},
			{ID: "unit-tbsp", Name: "tbsp"}, {ID: "unit-ml", Name: "ml"}, {ID: "unit-g", Name: "g"},
			{ID: "u1", Name: "Sachet"}, {ID: "u2", Name: "Kg"},
		},
		Items: []LocalItem{
			{ID: "item-garlic", Name: "Garlic", Emoji: "🧄", CategoryID: "cat-produce", Source: SourceSystem,
				BuyUnitIDs: []string{"unit-head"}, CookUnitIDs: []string{"unit-clove"},
				Conversions: []LocalConversion{{FromUnitID: "unit-head", ToUnitID: "unit-clove", Factor: 10}}},
			{ID: "item-olive-oil", Name: "Good oil", Emoji: "🫒", CategoryID: "cat-pantry", Source: SourceSystem, Hidden: true,
				BuyUnitIDs: []string{"unit-bottle"}, CookUnitIDs: []string{"unit-tbsp", "unit-ml"},
				Conversions: []LocalConversion{{FromUnitID: "unit-bottle", ToUnitID: "unit-ml", Factor: 750}}},
			{ID: "i1", Name: "Yeast", Emoji: "🍞", CategoryID: "c1", Source: SourceUser,
				BuyUnitIDs: []string{"u1"}, CookUnitIDs: []string{"unit-g"},
				Conversions: []LocalConversion{{FromUnitID: "u1", ToUnitID: "unit-g", Factor: 7}}},
			{ID: "i2", Name: "Flour", Emoji: "🌾", CategoryID: "c1", Source: SourceUser,
				BuyUnitIDs: []string{"u2"}, CookUnitIDs: []string{"unit-g"}},
		},
		Recipes: []LocalRecipe{{
			ID: "r1", Name: "  Garlic bread ", Servings: 4,
			Ingredients: []LocalIngredient{
				{ItemID: "item-garlic", UnitID: "unit-clove", Quantity: 3},
				{ItemID: "i2", UnitID: "unit-g", Quantity: 250},
				{ItemID: "i1", UnitID: "u1", Quantity: 1},
				{ItemID: "gone", UnitID: "unit-g", Quantity: 1},
			},
			Steps: []string{"Bake", "  "},
		}},
		Meals: []LocalMeal{{RecipeID: "r1", Servings: 2}, {RecipeID: "r1", Servings: 6, Cooked: true}},
		GroceryList: []LocalLine{
			{ItemID: "item-olive-oil", UnitID: "unit-bottle", Quantity: 0.04},
			{ItemID: "i2", UnitID: "u2", Quantity: 1.5, Checked: true},
		},
	}
}

func TestPlanImport(t *testing.T) {
	t.Run("maps starter records by key, and creates the user's own", func(t *testing.T) {
		state, u := importState(t)

		plan, err := PlanImport(importer, localData(), state, uuid.New)

		require.NoError(t, err)
		produce, pantry := state.Categories[0], state.Categories[1]
		require.Equal(t, "Veg", produce.Name())
		require.True(t, pantry.Hidden())
		require.ElementsMatch(t, []*Category{produce, pantry}, plan.ChangedCategories)
		require.Len(t, plan.NewCategories, 1)
		baking := plan.NewCategories[0]
		require.Equal(t, "Baking", baking.Name())

		oil := state.Items[1]
		require.Equal(t, "Good oil", oil.Name())
		require.Equal(t, "olive-oil", oil.SystemKey())
		require.True(t, oil.Hidden())
		require.Equal(t, []uuid.UUID{u["tbsp"], u["ml"]}, oil.CookUnitIDs())
		require.Contains(t, plan.ChangedItems, oil)

		require.Len(t, plan.NewItems, 2)
		yeast, flour := plan.NewItems[0], plan.NewItems[1]
		require.Equal(t, baking.ID(), yeast.CategoryID())
		require.Equal(t, []uuid.UUID{u["piece"]}, yeast.BuyUnitIDs(), "Sachet isn't a unit, so it's piece")
		require.Equal(t, []Conversion{{From: u["piece"], To: u["g"], Factor: 7}}, yeast.Conversions())
		require.Equal(t, []uuid.UUID{u["kg"]}, flour.BuyUnitIDs(), "Kg is matched by name")
		require.Contains(t, plan.Result.Notes, "The unit “Sachet” isn't one the app has any more, so amounts in it are now in pieces.")
	})

	t.Run("creates the recipes, plans the meals not cooked yet and fills the list", func(t *testing.T) {
		state, u := importState(t)

		plan, err := PlanImport(importer, localData(), state, uuid.New)

		require.NoError(t, err)
		require.Len(t, plan.Recipes, 1)
		bread := plan.Recipes[0]
		require.Equal(t, map[string]uuid.UUID{"r1": bread.ID()}, plan.Result.RecipeIDs)
		require.Equal(t, "Garlic bread", bread.Name())
		require.Equal(t, []string{"Bake"}, bread.Steps())
		require.Len(t, bread.Ingredients(), 3)
		require.Equal(t, Ingredient{ItemID: state.Items[0].ID(), UnitID: u["clove"], Quantity: 3}, bread.Ingredients()[0])
		require.Contains(t, plan.Result.Notes, "Garlic bread: left out an ingredient without an item or an amount.")

		yeast := plan.NewItems[0]
		require.Equal(t, []uuid.UUID{u["g"], u["piece"]}, yeast.CookUnitIDs(), "the recipe's unit is added to the item")

		require.Len(t, plan.Meals, 1)
		require.Equal(t, bread.ID(), plan.Meals[0].RecipeID())
		require.Equal(t, 2, plan.Meals[0].Servings())

		require.Len(t, plan.Lines, 2)
		require.Equal(t, state.Items[1].ID(), plan.Lines[0].ItemID())
		require.InDelta(t, 0.04, plan.Lines[0].Quantity(), 1e-9)
		require.Equal(t, u["kg"], plan.Lines[1].UnitID())
		require.True(t, plan.Lines[1].Checked())
	})

	t.Run("joins records the account already has by name, and adds to its lines", func(t *testing.T) {
		state, u := importState(t)
		first, err := PlanImport(importer, localData(), state, uuid.New)
		require.NoError(t, err)
		// What the first phone's import saved is now the account's.
		state.Categories = append(state.Categories, first.NewCategories...)
		state.Items = append(state.Items, first.NewItems...)
		state.Lines = first.Lines

		second, err := PlanImport(importer, localData(), state, uuid.New)

		require.NoError(t, err)
		require.Empty(t, second.NewCategories)
		require.Empty(t, second.NewItems)
		require.Len(t, second.Recipes, 1, "recipes are always created")
		require.Len(t, second.Lines, 1, "the bought flour isn't added again")
		require.Equal(t, first.Lines[0].ID(), second.Lines[0].ID())
		require.Equal(t, u["bottle"], second.Lines[0].UnitID())
		require.InDelta(t, 0.08, second.Lines[0].Quantity(), 1e-9)
	})

	t.Run("keeps units the account uses, and conversions that don't clash", func(t *testing.T) {
		state, u := importState(t)
		garlic := state.Items[0]
		state.Usage[garlic.ID()] = ItemUsage{RecipeUnitIDs: []uuid.UUID{u["g"]}}
		local := localData()
		local.Items[0].Conversions = []LocalConversion{
			{FromUnitID: "unit-head", ToUnitID: "unit-clove", Factor: 12},
			{FromUnitID: "unit-clove", ToUnitID: "unit-g", Factor: 5},
		}

		plan, err := PlanImport(importer, local, state, uuid.New)

		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{u["clove"], u["g"]}, garlic.CookUnitIDs())
		require.Equal(t, []Conversion{
			{From: u["head"], To: u["clove"], Factor: 12},
			{From: u["clove"], To: u["g"], Factor: 5},
		}, garlic.Conversions(), "the local rule wins over the account's own")
		for _, note := range plan.Result.Notes {
			require.NotContains(t, note, "conversion")
		}
	})

	t.Run("leaves out a local conversion that clashes, with a note", func(t *testing.T) {
		state, u := importState(t)
		local := localData()
		local.Items[0].Conversions = []LocalConversion{
			{FromUnitID: "unit-head", ToUnitID: "unit-clove", Factor: 10},
			{FromUnitID: "unit-head", ToUnitID: "unit-clove", Factor: 12},
			{FromUnitID: "unit-kg", ToUnitID: "unit-g", Factor: 1000},
		}

		plan, err := PlanImport(importer, local, state, uuid.New)

		require.NoError(t, err)
		require.Equal(t, []Conversion{{From: u["head"], To: u["clove"], Factor: 10}}, state.Items[0].Conversions())
		require.Contains(t, plan.Result.Notes, "Garlic: left out the conversion 1 head = 12 clove, which clashes with the item's others.")
		require.NotContains(t, plan.Result.Notes, "Garlic: left out the conversion 1 kg = 1000 g, which clashes with the item's others.",
			"a standard conversion is the same rule, so it goes quietly")
	})

	t.Run("tidies what a form would refuse", func(t *testing.T) {
		state, _ := importState(t)
		local := LocalData{
			Items: []LocalItem{{ID: "i1", Name: "", Emoji: "", CategoryID: "missing", BuyUnitIDs: nil}},
			Recipes: []LocalRecipe{{ID: "r1", Name: " ", Servings: 0, Ingredients: []LocalIngredient{
				{ItemID: "i1", UnitID: "unit-piece", Quantity: 2},
			}}},
			Meals:       []LocalMeal{{RecipeID: "r1", Servings: 500}, {RecipeID: "gone", Servings: 2}},
			GroceryList: []LocalLine{{ItemID: "i1", UnitID: "unit-piece", Quantity: 0}},
		}

		plan, err := PlanImport(importer, local, state, uuid.New)

		require.NoError(t, err)
		item := plan.NewItems[0]
		require.Equal(t, "Unnamed item", item.Name())
		require.Equal(t, "🛒", item.Emoji())
		require.Equal(t, state.Categories[1].ID(), item.CategoryID(), "Pantry comes first by name")
		require.Equal(t, "Untitled recipe", plan.Recipes[0].Name())
		require.Equal(t, 1, plan.Recipes[0].Servings())
		require.Len(t, plan.Meals, 1)
		require.Equal(t, 99, plan.Meals[0].Servings())
		require.Empty(t, plan.Lines)
		require.Contains(t, plan.Result.Notes, "Grocery list: left out a line without an item or an amount.")
	})

	t.Run("a renamed starter item keeps its name when another item has the new one", func(t *testing.T) {
		state, _ := importState(t)
		local := localData()
		local.Items[1].Name = "garlic"

		plan, err := PlanImport(importer, local, state, uuid.New)

		require.NoError(t, err)
		require.Equal(t, "Olive oil", state.Items[1].Name())
		require.Contains(t, plan.Result.Notes, "The item “Olive oil” kept its name: you already have an item called “garlic”.")
	})
}
