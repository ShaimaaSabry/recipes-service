package copystarterdata

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/recipes/domain/model"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

type fakeCatalog struct {
	copied     bool
	categories []*model.Category
	items      []*model.GroceryItem
}

var kg = model.UnitOf(uuid.New(), "kg", "kg", uuid.Nil, 0, false)

func (f *fakeCatalog) GetUnits(context.Context) ([]model.Unit, error) { return []model.Unit{kg}, nil }

func (f *fakeCatalog) StarterDataCopied(context.Context, owner.ID) (bool, error) {
	return f.copied, nil
}

func (f *fakeCatalog) SaveStarterData(_ context.Context, _ owner.ID, categories []*model.Category, items []*model.GroceryItem) (bool, error) {
	f.categories, f.items, f.copied = categories, items, true
	return true, nil
}

var starter = model.StarterData{
	Categories: []model.StarterCategory{{Key: "produce", Name: "Produce", Emoji: "🥕"}},
	Items:      []model.StarterItem{{Key: "potatoes", Name: "Potatoes", Emoji: "🥔", Category: "produce", Buy: []string{"kg"}}},
}

func TestExecute(t *testing.T) {
	ctx := context.Background()

	t.Run("copies the starter data, with IDs of the account's own", func(t *testing.T) {
		catalog := &fakeCatalog{}

		require.NoError(t, NewInteractor(catalog, starter).Execute(ctx, Command{Owner: "ada"}))

		require.Len(t, catalog.categories, 1)
		require.Len(t, catalog.items, 1)
		potatoes := catalog.items[0]
		require.Equal(t, owner.ID("ada"), potatoes.Owner())
		require.Equal(t, "potatoes", potatoes.SystemKey())
		require.Equal(t, catalog.categories[0].ID(), potatoes.CategoryID())
		require.Equal(t, []uuid.UUID{kg.ID()}, potatoes.BuyUnitIDs())
		require.Equal(t, uuid.Version(7), potatoes.ID().Version())
	})

	t.Run("does nothing for an account that has its copies", func(t *testing.T) {
		catalog := &fakeCatalog{copied: true}

		require.NoError(t, NewInteractor(catalog, starter).Execute(ctx, Command{Owner: "ada"}))

		require.Nil(t, catalog.items)
	})

	t.Run("fails for starter data naming a unit that doesn't exist", func(t *testing.T) {
		broken := model.StarterData{
			Categories: starter.Categories,
			Items:      []model.StarterItem{{Key: "milk", Name: "Milk", Emoji: "🥛", Category: "produce", Buy: []string{"quart"}}},
		}

		err := NewInteractor(&fakeCatalog{}, broken).Execute(ctx, Command{Owner: "ada"})

		require.ErrorContains(t, err, `starter item "milk": no unit "quart"`)
	})
}
