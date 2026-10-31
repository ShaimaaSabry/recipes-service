package model

import (
	"fmt"
	"slices"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

var ErrNoBuyUnit = fmt.Errorf("%w: please choose at least one unit to buy it in", apperrors.ErrInvalidInput)

// ItemDetails is what the owner fills in for a grocery item.
type ItemDetails struct {
	Name       string
	Emoji      string
	CategoryID uuid.UUID
	// BuyUnitIDs are the units the grocery list offers, in order. At least one.
	BuyUnitIDs []uuid.UUID
	// CookUnitIDs are the units recipes measure it in, in order. May be empty, for things nobody
	// cooks with (dish soap).
	CookUnitIDs []uuid.UUID
}

// GroceryItem is something the owner buys, and that recipes are made from.
type GroceryItem struct {
	id          uuid.UUID
	owner       owner.ID
	details     ItemDetails
	conversions []Conversion
	source      Source
	systemKey   string
	hidden      bool
}

// NewGroceryItem validates an item the owner makes. Checking the category is the owner's is left
// to the caller.
func NewGroceryItem(id uuid.UUID, owner owner.ID, details ItemDetails, conversions []Conversion, units Units) (*GroceryItem, error) {
	item := &GroceryItem{id: id, owner: owner, source: SourceUser}
	if err := item.Edit(details, conversions, units); err != nil {
		return nil, err
	}
	return item, nil
}

// NewStarterGroceryItem validates an item copied from the starter data entry systemKey.
func NewStarterGroceryItem(id uuid.UUID, owner owner.ID, systemKey string, details ItemDetails, conversions []Conversion, units Units) (*GroceryItem, error) {
	item := &GroceryItem{id: id, owner: owner, source: SourceSystem, systemKey: systemKey}
	if err := item.Edit(details, conversions, units); err != nil {
		return nil, err
	}
	return item, nil
}

// GroceryItemOf rebuilds an item that was already validated, e.g. one read back from the database.
func GroceryItemOf(id uuid.UUID, owner owner.ID, details ItemDetails, conversions []Conversion, source Source, systemKey string, hidden bool) *GroceryItem {
	return &GroceryItem{
		id: id, owner: owner, details: details, conversions: conversions,
		source: source, systemKey: systemKey, hidden: hidden,
	}
}

func (i GroceryItem) ID() uuid.UUID             { return i.id }
func (i GroceryItem) Owner() owner.ID           { return i.owner }
func (i GroceryItem) Name() string              { return i.details.Name }
func (i GroceryItem) Emoji() string             { return i.details.Emoji }
func (i GroceryItem) CategoryID() uuid.UUID     { return i.details.CategoryID }
func (i GroceryItem) BuyUnitIDs() []uuid.UUID   { return i.details.BuyUnitIDs }
func (i GroceryItem) CookUnitIDs() []uuid.UUID  { return i.details.CookUnitIDs }
func (i GroceryItem) Conversions() []Conversion { return i.conversions }
func (i GroceryItem) Source() Source            { return i.source }
func (i GroceryItem) SystemKey() string         { return i.systemKey }
func (i GroceryItem) Hidden() bool              { return i.hidden }
func (i GroceryItem) Factor(units Units, from, to uuid.UUID) (float64, bool) {
	return Factor(units, i.conversions, from, to)
}

// Edit replaces what the owner fills in, and the item's conversions. Starter items can be edited
// too, and stay starter ones.
func (i *GroceryItem) Edit(details ItemDetails, conversions []Conversion, units Units) error {
	details.Name = normalizeName(details.Name)
	if err := validateNameAndEmoji("grocery item", details.Name, details.Emoji); err != nil {
		return err
	}
	if len(details.BuyUnitIDs) == 0 {
		return ErrNoBuyUnit
	}
	for _, list := range []struct {
		purpose string
		ids     []uuid.UUID
	}{{"buy", details.BuyUnitIDs}, {"cook", details.CookUnitIDs}} {
		purpose, ids := list.purpose, list.ids
		for n, id := range ids {
			if _, ok := units.Find(id); !ok {
				return fmt.Errorf("%w: %s is not a unit", apperrors.ErrInvalidInput, id)
			}
			if slices.Contains(ids[:n], id) {
				return fmt.Errorf("%w: %s is listed twice as a unit to %s it in", apperrors.ErrInvalidInput, units.name(id), purpose)
			}
		}
	}
	if err := validateConversions(units, conversions); err != nil {
		return err
	}
	i.details, i.conversions = details, conversions
	return nil
}

// EditKeepingConversions is Edit for a form that doesn't show the conversions: the saved ones
// stay, except those naming a unit the item no longer has. A unit the item never had stays
// usable in them, as a stepping stone (1 piece = 150 g, for something bought in kg).
func (i *GroceryItem) EditKeepingConversions(details ItemDetails, units Units) error {
	removed := func(id uuid.UUID) bool {
		had := slices.Contains(i.details.BuyUnitIDs, id) || slices.Contains(i.details.CookUnitIDs, id)
		has := slices.Contains(details.BuyUnitIDs, id) || slices.Contains(details.CookUnitIDs, id)
		return had && !has
	}
	kept := make([]Conversion, 0, len(i.conversions))
	for _, c := range i.conversions {
		if !removed(c.From) && !removed(c.To) {
			kept = append(kept, c)
		}
	}
	return i.Edit(details, kept, units)
}

// SetConversion adds a conversion, replacing the one for the same two units if there is one: the
// app's answer to "1 box = ? piece".
func (i *GroceryItem) SetConversion(conversion Conversion, units Units) error {
	conversions := make([]Conversion, 0, len(i.conversions)+1)
	for _, c := range i.conversions {
		samePair := (c.From == conversion.From && c.To == conversion.To) || (c.From == conversion.To && c.To == conversion.From)
		if !samePair {
			conversions = append(conversions, c)
		}
	}
	conversions = append(conversions, conversion)
	if err := validateConversions(units, conversions); err != nil {
		return err
	}
	i.conversions = conversions
	return nil
}

// SetHidden hides the item from pickers, or shows it again.
func (i *GroceryItem) SetHidden(hidden bool) { i.hidden = hidden }

// CanDelete refuses for a starter item, which can only be hidden.
func (i GroceryItem) CanDelete() error { return refuseStarterDelete(i.source, i.details.Name) }
