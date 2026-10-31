package model

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

func TestGroceryItem(t *testing.T) {
	_, units, id := loadConversionCases(t)
	category := uuid.New()
	garlic := func() ItemDetails {
		return ItemDetails{
			Name: "  Garlic  ", Emoji: "🧄", CategoryID: category,
			BuyUnitIDs: []uuid.UUID{id["head"], id["g"]}, CookUnitIDs: []uuid.UUID{id["clove"], id["g"]},
		}
	}
	rules := []Conversion{{From: id["head"], To: id["clove"], Factor: 10}, {From: id["clove"], To: id["g"], Factor: 5}}

	t.Run("a new item is the owner's, with its name tidied", func(t *testing.T) {
		item, err := NewGroceryItem(uuid.New(), "owner-1", garlic(), rules, units)

		require.NoError(t, err)
		require.Equal(t, "Garlic", item.Name())
		require.Equal(t, SourceUser, item.Source())
		require.NoError(t, item.CanDelete())
		factor, ok := item.Factor(units, id["head"], id["kg"])
		require.True(t, ok)
		require.InEpsilon(t, 0.05, factor, 1e-9)
	})

	t.Run("a starter item can be hidden but not deleted", func(t *testing.T) {
		item, err := NewStarterGroceryItem(uuid.New(), "owner-1", "garlic", garlic(), rules, units)
		require.NoError(t, err)

		item.SetHidden(true)

		require.True(t, item.Hidden())
		require.Equal(t, "garlic", item.SystemKey())
		err = item.CanDelete()
		require.ErrorIs(t, err, apperrors.ErrConflict)
		require.Contains(t, err.Error(), "“Garlic” came with the app")
	})

	invalid := map[string]func(d *ItemDetails){
		"no name":           func(d *ItemDetails) { d.Name = "   " },
		"a very long name":  func(d *ItemDetails) { d.Name = strings.Repeat("a", MaxNameLength+1) },
		"no emoji":          func(d *ItemDetails) { d.Emoji = "" },
		"no buy unit":       func(d *ItemDetails) { d.BuyUnitIDs = nil },
		"an unknown unit":   func(d *ItemDetails) { d.CookUnitIDs = []uuid.UUID{uuid.New()} },
		"a unit used twice": func(d *ItemDetails) { d.BuyUnitIDs = []uuid.UUID{id["g"], id["g"]} },
	}
	for name, change := range invalid {
		t.Run("refuses "+name, func(t *testing.T) {
			details := garlic()
			change(&details)

			_, err := NewGroceryItem(uuid.New(), "owner-1", details, nil, units)

			require.ErrorIs(t, err, apperrors.ErrInvalidInput)
		})
	}

	t.Run("cook units may be empty", func(t *testing.T) {
		details := garlic()
		details.CookUnitIDs = nil

		_, err := NewGroceryItem(uuid.New(), "owner-1", details, nil, units)

		require.NoError(t, err)
	})

	t.Run("editing without conversions keeps them, minus the removed units'", func(t *testing.T) {
		pepper := ItemDetails{Name: "Pepper", Emoji: "🫑", CategoryID: category, BuyUnitIDs: []uuid.UUID{id["piece"], id["kg"]}}
		item, err := NewGroceryItem(uuid.New(), "owner-1", pepper, []Conversion{{From: id["piece"], To: id["g"], Factor: 150}}, units)
		require.NoError(t, err)

		// g was only ever a stepping stone, so the rule stays while the item keeps piece.
		pepper.BuyUnitIDs = []uuid.UUID{id["piece"]}
		require.NoError(t, item.EditKeepingConversions(pepper, units))
		require.Len(t, item.Conversions(), 1)

		// Without piece, the rule goes.
		pepper.BuyUnitIDs = []uuid.UUID{id["kg"]}
		require.NoError(t, item.EditKeepingConversions(pepper, units))
		require.Empty(t, item.Conversions())
	})

	t.Run("setting a conversion replaces the one for the same units, either way round", func(t *testing.T) {
		item, err := NewGroceryItem(uuid.New(), "owner-1", garlic(), rules, units)
		require.NoError(t, err)

		require.NoError(t, item.SetConversion(Conversion{From: id["clove"], To: id["head"], Factor: 0.125}, units))

		require.Len(t, item.Conversions(), 2)
		factor, _ := item.Factor(units, id["head"], id["clove"])
		require.InEpsilon(t, 8, factor, 1e-9)
	})

	t.Run("setting a conversion that already follows is refused", func(t *testing.T) {
		item, err := NewGroceryItem(uuid.New(), "owner-1", garlic(), rules, units)
		require.NoError(t, err)

		err = item.SetConversion(Conversion{From: id["head"], To: id["g"], Factor: 40}, units)

		require.ErrorIs(t, err, ErrInvalidConversion)
		require.Len(t, item.Conversions(), 2)
	})
}

func TestCategory(t *testing.T) {
	t.Run("a new category is the owner's, with its name tidied", func(t *testing.T) {
		category, err := NewCategory(uuid.New(), "owner-1", " Frozen   food ", "🧊")

		require.NoError(t, err)
		require.Equal(t, "Frozen food", category.Name())
		require.NoError(t, category.CanDelete())
	})

	t.Run("a starter category stays one when edited, and can't be deleted", func(t *testing.T) {
		category, err := NewStarterCategory(uuid.New(), "owner-1", "bakery", "Bakery", "🍞")
		require.NoError(t, err)

		require.NoError(t, category.Edit("Bread", "🥖"))

		require.Equal(t, SourceSystem, category.Source())
		require.ErrorIs(t, category.CanDelete(), apperrors.ErrConflict)
	})

	t.Run("refuses a blank name", func(t *testing.T) {
		_, err := NewCategory(uuid.New(), "owner-1", " ", "🧊")

		require.ErrorIs(t, err, apperrors.ErrInvalidInput)
	})

	t.Run("names match ignoring case and spacing", func(t *testing.T) {
		require.True(t, SameName("Frozen  Food", " frozen food"))
		require.False(t, SameName("Frozen", "Fresh"))
	})
}
