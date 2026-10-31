package model

import "github.com/google/uuid"

// Unit is a unit things are bought or cooked in. Units are the same for every account and only
// change with a migration.
type Unit struct {
	id         uuid.UUID
	code       string
	name       string
	baseUnitID uuid.UUID
	baseSize   float64
	soldWhole  bool
}

// UnitOf rebuilds a unit read back from the database. baseUnitID is uuid.Nil, and baseSize 0, for
// a unit whose size depends on the item (pack, head…); a base unit has itself as its base.
func UnitOf(id uuid.UUID, code, name string, baseUnitID uuid.UUID, baseSize float64, soldWhole bool) Unit {
	return Unit{id: id, code: code, name: name, baseUnitID: baseUnitID, baseSize: baseSize, soldWhole: soldWhole}
}

func (u Unit) ID() uuid.UUID   { return u.id }
func (u Unit) Code() string    { return u.code }
func (u Unit) Name() string    { return u.name }
func (u Unit) SoldWhole() bool { return u.soldWhole }

// Base is the unit this one's size is measured in, and how many of it one of these is: cup is
// 240 of ml. ok is false for a unit without a fixed size.
func (u Unit) Base() (baseUnitID uuid.UUID, size float64, ok bool) {
	return u.baseUnitID, u.baseSize, u.baseUnitID != uuid.Nil
}

// Units is every unit, in the order pickers list them.
type Units struct {
	list []Unit
	byID map[uuid.UUID]Unit
}

func NewUnits(list []Unit) Units {
	byID := make(map[uuid.UUID]Unit, len(list))
	for _, unit := range list {
		byID[unit.id] = unit
	}
	return Units{list: list, byID: byID}
}

func (u Units) All() []Unit { return u.list }

func (u Units) Find(id uuid.UUID) (Unit, bool) {
	unit, ok := u.byID[id]
	return unit, ok
}

// name is the unit's name for messages, or its ID if it isn't a unit.
func (u Units) name(id uuid.UUID) string {
	if unit, ok := u.byID[id]; ok {
		return unit.name
	}
	return id.String()
}
