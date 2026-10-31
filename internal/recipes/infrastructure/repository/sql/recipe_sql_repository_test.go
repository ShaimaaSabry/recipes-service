package sql

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/platform/database"
	"github.com/mamienellie/recipes-service/internal/platform/database/databasetest"
	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type repositories struct {
	recipes     *RecipeSqlRepository
	catalog     *CatalogSqlRepository
	groceryList *GroceryListSqlRepository
	units       model.Units
	unit        map[string]uuid.UUID
}

// newRepositories gives "owner-1" and "owner-2" their starter data.
func newRepositories(t *testing.T) repositories {
	db := databasetest.New(t)
	require.NoError(t, database.Migrate(context.Background(), db, "recipes", Migrations()))
	catalog := NewCatalogSqlRepository(db)
	copyStarterData(t, catalog, "owner-1")
	copyStarterData(t, catalog, "owner-2")
	all, code := units(t, catalog)
	return repositories{
		recipes: NewRecipeSqlRepository(db), catalog: catalog, groceryList: NewGroceryListSqlRepository(db),
		units: all, unit: code,
	}
}

func (r repositories) item(t *testing.T, o owner.ID, name string) *model.GroceryItem {
	t.Helper()
	item, err := r.catalog.FindItemByName(context.Background(), o, name)
	require.NoError(t, err)
	require.NotNil(t, item, name)
	return item
}

func (r repositories) shakshuka(t *testing.T, o owner.ID) *model.Recipe {
	t.Helper()
	items, err := r.catalog.ItemsByID(context.Background(), o)
	require.NoError(t, err)
	details := model.RecipeDetails{
		Name: "Shakshuka", Servings: 2,
		Ingredients: []model.Ingredient{
			{ItemID: r.item(t, o, "Eggs").ID(), UnitID: r.unit["piece"], Quantity: 4},
			{ItemID: r.item(t, o, "Garlic").ID(), UnitID: r.unit["clove"], Quantity: 2},
		},
		Steps: []string{"Soften the onion.", "Crack in the eggs."},
	}
	recipe, err := model.NewRecipe(uuid.New(), o, details, items, r.units)
	require.NoError(t, err)
	require.NoError(t, r.recipes.CreateRecipe(context.Background(), recipe))
	return recipe
}

func TestRecipeSqlRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("saves a recipe with its ingredients and steps, in order", func(t *testing.T) {
		repos := newRepositories(t)
		recipe := repos.shakshuka(t, "owner-1")

		found, err := repos.recipes.FindRecipe(ctx, "owner-1", recipe.ID())

		require.NoError(t, err)
		require.Equal(t, recipe.Ingredients(), found.Ingredients())
		require.Equal(t, []string{"Soften the onion.", "Crack in the eggs."}, found.Steps())
		require.Equal(t, 2, found.Servings())
	})

	t.Run("another account can't find it", func(t *testing.T) {
		repos := newRepositories(t)
		recipe := repos.shakshuka(t, "owner-1")

		found, err := repos.recipes.FindRecipe(ctx, "owner-2", recipe.ID())

		require.NoError(t, err)
		require.Nil(t, found)
	})

	t.Run("saving replaces the ingredients and steps; the photo is saved on its own", func(t *testing.T) {
		repos := newRepositories(t)
		recipe := repos.shakshuka(t, "owner-1")
		items, err := repos.catalog.ItemsByID(ctx, "owner-1")
		require.NoError(t, err)
		require.NoError(t, recipe.Edit(model.RecipeDetails{Name: "Eggs", Servings: 1, Ingredients: recipe.Ingredients()[:1]}, items, repos.units))
		require.NoError(t, repos.recipes.SaveRecipe(ctx, recipe))
		recipe.SetPhotoKey("recipes/owner-1/x/photo.jpg")
		require.NoError(t, repos.recipes.SaveRecipePhoto(ctx, recipe))

		found, err := repos.recipes.FindRecipe(ctx, "owner-1", recipe.ID())

		require.NoError(t, err)
		require.Equal(t, "Eggs", found.Name())
		require.Len(t, found.Ingredients(), 1)
		require.Empty(t, found.Steps())
		require.Equal(t, "recipes/owner-1/x/photo.jpg", found.PhotoKey())
	})

	t.Run("an item in a recipe can't be deleted", func(t *testing.T) {
		repos := newRepositories(t)
		repos.shakshuka(t, "owner-1")
		garlic := repos.item(t, "owner-1", "Garlic")

		usage, err := repos.catalog.ItemUsage(ctx, "owner-1", garlic.ID())
		require.NoError(t, err)
		require.Equal(t, []uuid.UUID{repos.unit["clove"]}, usage.RecipeUnitIDs)
		require.ErrorIs(t, repos.catalog.DeleteItem(ctx, "owner-1", garlic.ID()), model.ErrItemInUse)
	})

	t.Run("meals: saving twice updates, another account's id is refused", func(t *testing.T) {
		repos := newRepositories(t)
		recipe := repos.shakshuka(t, "owner-1")
		meal, err := model.NewPlannedMeal(uuid.New(), "owner-1", recipe.ID(), 2)
		require.NoError(t, err)

		_, err = repos.recipes.SaveMeal(ctx, meal)
		require.NoError(t, err)
		again, _ := model.NewPlannedMeal(meal.ID(), "owner-1", recipe.ID(), 6)
		saved, err := repos.recipes.SaveMeal(ctx, again)
		require.NoError(t, err)
		require.Equal(t, 6, saved.Servings())

		theirRecipe := repos.shakshuka(t, "owner-2")
		stolen, _ := model.NewPlannedMeal(meal.ID(), "owner-2", theirRecipe.ID(), 1)
		_, err = repos.recipes.SaveMeal(ctx, stolen)
		require.ErrorIs(t, err, model.ErrMealIDTaken)

		meals, err := repos.recipes.GetMeals(ctx, "owner-1")
		require.NoError(t, err)
		require.Len(t, meals, 1)
		require.Equal(t, 6, meals[0].Servings())
	})

	t.Run("a meal for another account's recipe is refused", func(t *testing.T) {
		repos := newRepositories(t)
		theirs := repos.shakshuka(t, "owner-2")
		meal, _ := model.NewPlannedMeal(uuid.New(), "owner-1", theirs.ID(), 2)

		_, err := repos.recipes.SaveMeal(ctx, meal)

		require.ErrorIs(t, err, model.ErrNoSuchRecipe)
	})

	t.Run("deleting a recipe takes its meals", func(t *testing.T) {
		repos := newRepositories(t)
		recipe := repos.shakshuka(t, "owner-1")
		meal, _ := model.NewPlannedMeal(uuid.New(), "owner-1", recipe.ID(), 2)
		_, err := repos.recipes.SaveMeal(ctx, meal)
		require.NoError(t, err)

		require.NoError(t, repos.recipes.DeleteRecipe(ctx, "owner-1", recipe.ID()))

		meals, err := repos.recipes.GetMeals(ctx, "owner-1")
		require.NoError(t, err)
		require.Empty(t, meals)
	})

	t.Run("deleting an owner's data clears recipes, meals and list, then the catalog can go", func(t *testing.T) {
		repos := newRepositories(t)
		repos.shakshuka(t, "owner-1")
		addEggs(t, repos, "owner-1", uuid.New())
		repos.shakshuka(t, "owner-2")

		require.NoError(t, repos.recipes.DeleteOwnerData(ctx, "owner-1"))
		require.NoError(t, repos.catalog.DeleteOwnerData(ctx, "owner-1"))

		mine, err := repos.recipes.GetRecipes(ctx, "owner-1")
		require.NoError(t, err)
		require.Empty(t, mine)
		lines, err := repos.groceryList.GetLines(ctx, "owner-1")
		require.NoError(t, err)
		require.Empty(t, lines)
		theirs, err := repos.recipes.GetRecipes(ctx, "owner-2")
		require.NoError(t, err)
		require.Len(t, theirs, 1)
	})
}

// addEggs applies "add 2 eggs" as the action actionID, and says whether it was applied.
func addEggs(t *testing.T, repos repositories, o owner.ID, actionID uuid.UUID) bool {
	t.Helper()
	eggs := repos.item(t, o, "Eggs").ID()
	applied, err := repos.groceryList.ApplyAction(context.Background(), o, actionID, addTwo(o, eggs, repos.unit["piece"]))
	require.NoError(t, err)
	return applied
}

func addTwo(o owner.ID, itemID, unitID uuid.UUID) func(model.GroceryListStore) error {
	return func(store model.GroceryListStore) error {
		ctx := context.Background()
		units, err := store.Units(ctx)
		if err != nil {
			return err
		}
		item, err := store.FindItem(ctx, o, itemID)
		if err != nil {
			return err
		}
		line, err := store.FindLine(ctx, o, itemID)
		if err != nil {
			return err
		}
		if line, err = model.AddToList(line, item, units, unitID, 2, uuid.New()); err != nil {
			return err
		}
		return store.SaveLine(ctx, line)
	}
}

func TestGroceryListSqlRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("an action resent with the same id is applied once", func(t *testing.T) {
		repos := newRepositories(t)
		action := uuid.New()

		require.True(t, addEggs(t, repos, "owner-1", action))
		require.False(t, addEggs(t, repos, "owner-1", action))
		require.True(t, addEggs(t, repos, "owner-1", uuid.New()))

		lines, err := repos.groceryList.GetLines(ctx, "owner-1")
		require.NoError(t, err)
		require.Len(t, lines, 1)
		require.InDelta(t, 4.0, lines[0].Quantity(), 0, "2 eggs twice, not three times")
	})

	t.Run("a failed action saves nothing, not even its id", func(t *testing.T) {
		repos := newRepositories(t)
		action := uuid.New()

		_, err := repos.groceryList.ApplyAction(ctx, "owner-1", action, func(model.GroceryListStore) error { return model.ErrNotOnList })
		require.ErrorIs(t, err, model.ErrNotOnList)

		require.True(t, addEggs(t, repos, "owner-1", action), "the same action can be sent again")
	})

	t.Run("two phones adding the same item at once share one line", func(t *testing.T) {
		repos := newRepositories(t)
		eggs := repos.item(t, "owner-1", "Eggs").ID()
		var wg sync.WaitGroup
		errs := make([]error, 8)
		for n := range errs {
			wg.Go(func() {
				_, errs[n] = repos.groceryList.ApplyAction(ctx, "owner-1", uuid.New(), addTwo("owner-1", eggs, repos.unit["piece"]))
			})
		}
		wg.Wait()

		require.Equal(t, make([]error, 8), errs)
		lines, err := repos.groceryList.GetLines(ctx, "owner-1")
		require.NoError(t, err)
		require.Len(t, lines, 1)
		require.InDelta(t, 16.0, lines[0].Quantity(), 0)
	})

	t.Run("checked lines are cleared, and an item on the list can't be deleted", func(t *testing.T) {
		repos := newRepositories(t)
		addEggs(t, repos, "owner-1", uuid.New())
		eggs := repos.item(t, "owner-1", "Eggs").ID()
		require.ErrorIs(t, repos.catalog.DeleteItem(ctx, "owner-1", eggs), model.ErrItemInUse)

		_, err := repos.groceryList.ApplyAction(ctx, "owner-1", uuid.New(), func(store model.GroceryListStore) error {
			line, err := store.FindLine(ctx, "owner-1", eggs)
			if err != nil {
				return err
			}
			line.SetChecked(true)
			if err := store.SaveLine(ctx, line); err != nil {
				return err
			}
			return store.DeleteCheckedLines(ctx, "owner-1")
		})
		require.NoError(t, err)

		lines, err := repos.groceryList.GetLines(ctx, "owner-1")
		require.NoError(t, err)
		require.Empty(t, lines)
	})
}
