package model

import (
	"fmt"
	"math"
	"slices"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

var (
	ErrNotOnList       = fmt.Errorf("%w: that item isn't on the grocery list", apperrors.ErrNotFound)
	ErrInvalidQuantity = fmt.Errorf("%w: the amount must be more than 0", apperrors.ErrInvalidInput)
)

// GroceryLine is the grocery list's one line for an item: how much of it to buy, in one of the
// units it's bought in. Quantity is exact; BuyQuantity is what the list shows.
type GroceryLine struct {
	id       uuid.UUID
	owner    owner.ID
	itemID   uuid.UUID
	unitID   uuid.UUID
	quantity float64
	checked  bool
}

// GroceryLineOf rebuilds a line that was already validated, e.g. one read back from the database.
func GroceryLineOf(id uuid.UUID, owner owner.ID, itemID, unitID uuid.UUID, quantity float64, checked bool) *GroceryLine {
	return &GroceryLine{id: id, owner: owner, itemID: itemID, unitID: unitID, quantity: quantity, checked: checked}
}

func (l GroceryLine) ID() uuid.UUID     { return l.id }
func (l GroceryLine) Owner() owner.ID   { return l.owner }
func (l GroceryLine) ItemID() uuid.UUID { return l.itemID }
func (l GroceryLine) UnitID() uuid.UUID { return l.unitID }
func (l GroceryLine) Quantity() float64 { return l.quantity }
func (l GroceryLine) Checked() bool     { return l.checked }

// MissingConversionError is an amount that can't join the item's line because the item has no
// conversion between the two units. The app asks "1 From = ? To", saves the answer as a
// conversion, and tries again.
type MissingConversionError struct {
	ItemID     uuid.UUID
	FromUnitID uuid.UUID
	ToUnitID   uuid.UUID
	message    string
}

func (e *MissingConversionError) Error() string { return e.message }
func (e *MissingConversionError) Unwrap() error { return apperrors.ErrConflict }

// missingConversion asks the question the way round that reads best: a unit without a fixed size
// on the "1 ___" side ("1 box = ? piece", not "1 piece = ? box").
func missingConversion(item *GroceryItem, units Units, a, b uuid.UUID) *MissingConversionError {
	if _, _, aSized := mustFind(units, a).Base(); aSized {
		if _, _, bSized := mustFind(units, b).Base(); !bSized {
			a, b = b, a
		}
	}
	return &MissingConversionError{
		ItemID: item.ID(), FromUnitID: a, ToUnitID: b,
		message: fmt.Sprintf("%s: %s doesn't convert to %s yet. How many %s are in 1 %s?",
			item.Name(), units.name(a), units.name(b), units.name(b), units.name(a)),
	}
}

func mustFind(units Units, id uuid.UUID) Unit {
	unit, _ := units.Find(id)
	return unit
}

// AddToList adds an amount of item to its line, converting it into the line's unit. line is the
// item's current line, or nil. A line still to buy grows; a bought (checked) one starts again,
// unchecked, with only the new amount. A new line uses the amount's unit if the item is bought in
// it, else the item's first buy unit. newID names a new line.
func AddToList(line *GroceryLine, item *GroceryItem, units Units, unitID uuid.UUID, quantity float64, newID uuid.UUID) (*GroceryLine, error) {
	if err := validateQuantity(quantity); err != nil {
		return nil, err
	}
	if _, ok := units.Find(unitID); !ok {
		return nil, fmt.Errorf("%w: %s is not a unit", apperrors.ErrInvalidInput, unitID)
	}

	toBuy := line != nil && !line.checked
	target := item.BuyUnitIDs()[0]
	switch {
	case toBuy:
		target = line.unitID
	case slices.Contains(item.BuyUnitIDs(), unitID):
		target = unitID
	}
	factor, ok := item.Factor(units, unitID, target)
	if !ok {
		return nil, missingConversion(item, units, unitID, target)
	}

	var next GroceryLine
	switch {
	case toBuy:
		next = *line
		next.quantity = RoundQuantity(line.quantity + quantity*factor)
	case line != nil:
		next = GroceryLine{id: line.id, owner: item.Owner(), itemID: item.ID(), unitID: target, quantity: RoundQuantity(quantity * factor)}
	default:
		next = GroceryLine{id: newID, owner: item.Owner(), itemID: item.ID(), unitID: target, quantity: RoundQuantity(quantity * factor)}
	}
	next.scaleUp(item, units)
	return &next, nil
}

// scaleUp moves a line from g or ml to kg or L once it reaches 1000, if the item is bought in
// that: 1500 g reads better as 1.5 kg.
func (l *GroceryLine) scaleUp(item *GroceryItem, units Units) {
	unit, ok := units.Find(l.unitID)
	base, size, sized := unit.Base()
	if !ok || !sized || base != unit.ID() || size != 1 || unit.SoldWhole() {
		return
	}
	for _, id := range item.BuyUnitIDs() {
		big, _ := units.Find(id)
		if bigBase, bigSize, ok := big.Base(); ok && bigBase == base && bigSize == 1000 && l.quantity >= bigSize {
			l.unitID, l.quantity = id, RoundQuantity(l.quantity/bigSize)
			return
		}
	}
}

// SetQuantity sets the amount by hand.
func (l *GroceryLine) SetQuantity(quantity float64) error {
	if err := validateQuantity(quantity); err != nil {
		return err
	}
	l.quantity = RoundQuantity(quantity)
	return nil
}

// SetUnit switches the line to another of the item's buy units, converting its amount.
func (l *GroceryLine) SetUnit(unitID uuid.UUID, item *GroceryItem, units Units) error {
	if !slices.Contains(item.BuyUnitIDs(), unitID) {
		return fmt.Errorf("%w: %s isn't bought in %s", apperrors.ErrInvalidInput, item.Name(), units.name(unitID))
	}
	factor, ok := item.Factor(units, l.unitID, unitID)
	if !ok {
		return missingConversion(item, units, l.unitID, unitID)
	}
	l.unitID, l.quantity = unitID, RoundQuantity(l.quantity*factor)
	return nil
}

func (l *GroceryLine) SetChecked(checked bool) { l.checked = checked }

// BuyQuantity is what the list asks you to buy: amounts in units sold whole rounded up (0.04
// bottle is 1 bottle, 1.25 packs is 2), weights and volumes as they are (1.3 kg).
func (l GroceryLine) BuyQuantity(units Units) float64 {
	exact := RoundQuantity(l.quantity)
	if unit, ok := units.Find(l.unitID); ok && unit.SoldWhole() {
		// The tolerance keeps 3.0000001 from a conversion at 3, not 4.
		return math.Max(1, math.Ceil(exact-1e-6))
	}
	return exact
}

func validateQuantity(quantity float64) error {
	if !(quantity > 0) || math.IsInf(quantity, 0) {
		return ErrInvalidQuantity
	}
	return nil
}

// GroceryList is the owner's lines, with the units needed to show them.
type GroceryList struct {
	Lines []*GroceryLine
	Units Units
}
