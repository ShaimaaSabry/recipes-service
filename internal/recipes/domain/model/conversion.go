package model

import (
	"fmt"
	"math"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// Conversion is one of an item's own rules: one From is Factor of To ("Garlic: 1 head = 10
// clove"). Standard conversions (1 kg = 1000 g) follow from the units and are never stored.
type Conversion struct {
	From   uuid.UUID
	To     uuid.UUID
	Factor float64
}

var ErrInvalidConversion = fmt.Errorf("%w: invalid conversion", apperrors.ErrInvalidInput)

// Factor says how many to are in one from, for an item with these conversions. It follows
// standard conversions and the item's own through as many steps as it takes: garlic goes head →
// clove → g → kg. ok is false when there's no way from one to the other.
//
// The app runs the same logic offline; testdata/conversion_cases.json holds the cases both must
// agree on.
func Factor(units Units, conversions []Conversion, from, to uuid.UUID) (factor float64, ok bool) {
	if from == to {
		return 1, true
	}
	graph := newConversionGraph(units, conversions)
	found := map[uuid.UUID]float64{from: 1}
	queue := []uuid.UUID{from}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range graph.edges(current) {
			if _, seen := found[edge.to]; seen {
				continue
			}
			total := found[current] * edge.factor
			if edge.to == to {
				return total, true
			}
			found[edge.to] = total
			queue = append(queue, edge.to)
		}
	}
	return 0, false
}

type conversionEdge struct {
	to     uuid.UUID
	factor float64 // one of the edge's start is factor of to
}

type conversionGraph struct {
	units Units
	// sameBase lists the units measured in each base unit, the base itself included.
	sameBase map[uuid.UUID][]Unit
	own      map[uuid.UUID][]conversionEdge
}

func newConversionGraph(units Units, conversions []Conversion) conversionGraph {
	sameBase := map[uuid.UUID][]Unit{}
	for _, unit := range units.All() {
		if base, _, ok := unit.Base(); ok {
			sameBase[base] = append(sameBase[base], unit)
		}
	}
	own := map[uuid.UUID][]conversionEdge{}
	for _, c := range conversions {
		own[c.From] = append(own[c.From], conversionEdge{to: c.To, factor: c.Factor})
		own[c.To] = append(own[c.To], conversionEdge{to: c.From, factor: 1 / c.Factor})
	}
	return conversionGraph{units: units, sameBase: sameBase, own: own}
}

// edges are where one step from unit leads: every other unit with the same base (1 cup = 16
// tbsp, directly), then the item's own conversions.
func (g conversionGraph) edges(unitID uuid.UUID) []conversionEdge {
	var edges []conversionEdge
	if unit, ok := g.units.Find(unitID); ok {
		if base, size, ok := unit.Base(); ok {
			for _, other := range g.sameBase[base] {
				if other.id != unitID {
					edges = append(edges, conversionEdge{to: other.id, factor: size / other.baseSize})
				}
			}
		}
	}
	return append(edges, g.own[unitID]...)
}

// validateConversions checks an item's conversions are usable together. Each one must be needed:
// one that already follows from the others or from the units (1 kg = 1000 g) is refused, so two
// rules can never disagree about the same pair of units.
func validateConversions(units Units, conversions []Conversion) error {
	pairs := map[[2]uuid.UUID]bool{}
	for i, c := range conversions {
		from, to := units.name(c.From), units.name(c.To)
		if _, ok := units.Find(c.From); !ok {
			return fmt.Errorf("%w: %s is not a unit", ErrInvalidConversion, from)
		}
		if _, ok := units.Find(c.To); !ok {
			return fmt.Errorf("%w: %s is not a unit", ErrInvalidConversion, to)
		}
		if c.From == c.To {
			return fmt.Errorf("%w: it must be between two different units", ErrInvalidConversion)
		}
		if !(c.Factor > 0) || math.IsInf(c.Factor, 0) {
			return fmt.Errorf("%w: 1 %s = ? %s needs an amount more than 0", ErrInvalidConversion, from, to)
		}
		pair := [2]uuid.UUID{c.From, c.To}
		if c.To.String() < c.From.String() {
			pair = [2]uuid.UUID{c.To, c.From}
		}
		if pairs[pair] {
			return fmt.Errorf("%w: %s and %s have more than one conversion", ErrInvalidConversion, from, to)
		}
		pairs[pair] = true

		others := append(append([]Conversion{}, conversions[:i]...), conversions[i+1:]...)
		if _, ok := Factor(units, others, c.From, c.To); ok {
			return fmt.Errorf("%w: 1 %s = ? %s already follows from the units or the item's other conversions",
				ErrInvalidConversion, from, to)
		}
	}
	return nil
}
