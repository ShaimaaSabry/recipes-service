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
	"github.com/mamienellie/recipes-service/internal/recipes/infrastructure/starterdata"
	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

func newCatalogRepository(t *testing.T) *CatalogSqlRepository {
	db := databasetest.New(t)
	require.NoError(t, database.Migrate(context.Background(), db, "recipes", Migrations()))
	return NewCatalogSqlRepository(db)
}

func units(t *testing.T, repo *CatalogSqlRepository) (model.Units, map[string]uuid.UUID) {
	t.Helper()
	list, err := repo.GetUnits(context.Background())
	require.NoError(t, err)
	byCode := map[string]uuid.UUID{}
	for _, u := range list {
		byCode[u.Code()] = u.ID()
	}
	return model.NewUnits(list), byCode
}

func copyStarterData(t *testing.T, repo *CatalogSqlRepository, o owner.ID) bool {
	t.Helper()
	data, err := starterdata.Load()
	require.NoError(t, err)
	all, _ := units(t, repo)
	categories, items, err := data.CopyFor(o, all, uuid.NewV7)
	require.NoError(t, err)
	copied, err := repo.SaveStarterData(context.Background(), o, categories, items)
	require.NoError(t, err)
	return copied
}

func TestCatalogSqlRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("seeds the units, every base unit its own base", func(t *testing.T) {
		all, code := units(t, newCatalogRepository(t))

		require.Len(t, all.All(), 19)
		require.Equal(t, "piece", all.All()[0].Code())
		for _, u := range all.All() {
			base, size, ok := u.Base()
			if !ok {
				continue
			}
			baseUnit, found := all.Find(base)
			require.True(t, found)
			baseOfBase, baseSize, _ := baseUnit.Base()
			require.Equal(t, base, baseOfBase, "%s's base must be a base unit", u.Code())
			require.InDelta(t, 1.0, baseSize, 0)
			require.Positive(t, size)
		}
		factor, ok := model.Factor(all, nil, code["cup"], code["tbsp"])
		require.True(t, ok)
		require.InEpsilon(t, 16, factor, 1e-9)
	})

	t.Run("copies the starter data once, and reads it back", func(t *testing.T) {
		repo := newCatalogRepository(t)
		_, code := units(t, repo)

		require.True(t, copyStarterData(t, repo, "owner-1"))
		require.False(t, copyStarterData(t, repo, "owner-1"), "a second copy saves nothing")

		categories, err := repo.GetCategories(ctx, "owner-1")
		require.NoError(t, err)
		require.Len(t, categories, 10)
		require.Equal(t, "Bakery", categories[0].Name(), "by name")
		require.Equal(t, model.SourceSystem, categories[0].Source())

		items, err := repo.GetItems(ctx, "owner-1")
		require.NoError(t, err)
		require.Len(t, items, 32)
		garlic, err := repo.FindItemByName(ctx, "owner-1", "GARLIC")
		require.NoError(t, err)
		require.Equal(t, "garlic", garlic.SystemKey())
		require.Equal(t, []uuid.UUID{code["head"], code["g"]}, garlic.BuyUnitIDs())
		require.Equal(t, []uuid.UUID{code["clove"], code["g"]}, garlic.CookUnitIDs())
		require.Equal(t, []model.Conversion{
			{From: code["head"], To: code["clove"], Factor: 10},
			{From: code["clove"], To: code["g"], Factor: 5},
		}, garlic.Conversions())

		copied, err := repo.StarterDataCopied(ctx, "owner-1")
		require.NoError(t, err)
		require.True(t, copied)
	})

	t.Run("two sign-ins at once copy the starter data once", func(t *testing.T) {
		repo := newCatalogRepository(t)
		data, err := starterdata.Load()
		require.NoError(t, err)
		all, _ := units(t, repo)
		var wg sync.WaitGroup
		results, errs := make([]bool, 4), make([]error, 4)
		for n := range results {
			// Each with IDs of its own, as separate sign-ins would build them.
			categories, items, err := data.CopyFor("owner-1", all, uuid.NewV7)
			require.NoError(t, err)
			wg.Go(func() { results[n], errs[n] = repo.SaveStarterData(ctx, "owner-1", categories, items) })
		}
		wg.Wait()

		require.Equal(t, make([]error, 4), errs)
		require.ElementsMatch(t, []bool{true, false, false, false}, results)
		items, err := repo.GetItems(ctx, "owner-1")
		require.NoError(t, err)
		require.Len(t, items, 32)
	})

	t.Run("keeps each owner's records apart", func(t *testing.T) {
		repo := newCatalogRepository(t)
		copyStarterData(t, repo, "owner-1")
		copyStarterData(t, repo, "owner-2")
		mine, err := repo.FindCategoryByName(ctx, "owner-1", "Produce")
		require.NoError(t, err)

		theirs, err := repo.FindCategory(ctx, "owner-2", mine.ID())

		require.NoError(t, err)
		require.Nil(t, theirs)
	})

	t.Run("creates, saves and deletes an owner's own item", func(t *testing.T) {
		repo := newCatalogRepository(t)
		all, code := units(t, repo)
		category, err := model.NewCategory(uuid.New(), "owner-1", "Snacks", "🍿")
		require.NoError(t, err)
		require.NoError(t, repo.CreateCategory(ctx, category))
		details := model.ItemDetails{Name: "Crisps", Emoji: "🥔", CategoryID: category.ID(), BuyUnitIDs: []uuid.UUID{code["pack"]}}
		item, err := model.NewGroceryItem(uuid.New(), "owner-1", details, nil, all)
		require.NoError(t, err)
		require.NoError(t, repo.CreateItem(ctx, item))

		require.NoError(t, item.SetConversion(model.Conversion{From: code["pack"], To: code["g"], Factor: 150}, all))
		item.SetHidden(true)
		require.NoError(t, repo.SaveItem(ctx, item))
		saved, err := repo.FindItem(ctx, "owner-1", item.ID())
		require.NoError(t, err)
		require.True(t, saved.Hidden())
		require.Empty(t, saved.CookUnitIDs())
		require.Len(t, saved.Conversions(), 1)

		count, err := repo.CountItemsInCategory(ctx, "owner-1", category.ID())
		require.NoError(t, err)
		require.Equal(t, int64(1), count)
		require.ErrorIs(t, repo.DeleteCategory(ctx, "owner-1", category.ID()), ErrCategoryInUse)

		require.NoError(t, repo.DeleteItem(ctx, "owner-1", item.ID()))
		require.NoError(t, repo.DeleteCategory(ctx, "owner-1", category.ID()))
		gone, err := repo.FindCategory(ctx, "owner-1", category.ID())
		require.NoError(t, err)
		require.Nil(t, gone)
	})

	t.Run("refuses a second category with the same name, in any case", func(t *testing.T) {
		repo := newCatalogRepository(t)
		first, _ := model.NewCategory(uuid.New(), "owner-1", "Snacks", "🍿")
		second, _ := model.NewCategory(uuid.New(), "owner-1", "SNACKS", "🍿")
		require.NoError(t, repo.CreateCategory(ctx, first))

		err := repo.CreateCategory(ctx, second)

		require.ErrorIs(t, err, apperrors.ErrConflict)
	})

	t.Run("deletes an owner's catalog and no one else's", func(t *testing.T) {
		repo := newCatalogRepository(t)
		copyStarterData(t, repo, "owner-1")
		copyStarterData(t, repo, "owner-2")

		require.NoError(t, repo.DeleteOwnerData(ctx, "owner-1"))

		mine, err := repo.GetItems(ctx, "owner-1")
		require.NoError(t, err)
		require.Empty(t, mine)
		copied, err := repo.StarterDataCopied(ctx, "owner-1")
		require.NoError(t, err)
		require.False(t, copied)
		theirs, err := repo.GetCategories(ctx, "owner-2")
		require.NoError(t, err)
		require.Len(t, theirs, 10)
	})
}
