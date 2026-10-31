package model

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

type conversionCases struct {
	Units []struct {
		Code     string  `json:"code"`
		Base     string  `json:"base"`
		BaseSize float64 `json:"baseSize"`
	} `json:"units"`
	Cases []struct {
		Name        string   `json:"name"`
		Conversions [][3]any `json:"conversions"`
		From        string   `json:"from"`
		To          string   `json:"to"`
		Factor      *float64 `json:"factor"`
	} `json:"cases"`
}

// testUnits builds the units the cases name, by code, with IDs of their own.
func testUnits(t *testing.T, cases conversionCases) (Units, map[string]uuid.UUID) {
	t.Helper()
	ids := map[string]uuid.UUID{}
	for _, u := range cases.Units {
		ids[u.Code] = uuid.New()
	}
	list := make([]Unit, 0, len(cases.Units))
	for _, u := range cases.Units {
		list = append(list, UnitOf(ids[u.Code], u.Code, u.Code, ids[u.Base], u.BaseSize, u.Base == ""))
	}
	return NewUnits(list), ids
}

func loadConversionCases(t *testing.T) (conversionCases, Units, map[string]uuid.UUID) {
	t.Helper()
	data, err := os.ReadFile("testdata/conversion_cases.json")
	require.NoError(t, err)
	var cases conversionCases
	require.NoError(t, json.Unmarshal(data, &cases))
	units, ids := testUnits(t, cases)
	return cases, units, ids
}

func TestFactor_SharedCases(t *testing.T) {
	cases, units, ids := loadConversionCases(t)
	require.NotEmpty(t, cases.Cases)

	for _, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			conversions := make([]Conversion, 0, len(c.Conversions))
			for _, raw := range c.Conversions {
				from, to := ids[raw[0].(string)], ids[raw[1].(string)]
				require.NotEqual(t, uuid.Nil, from, "unknown unit %v", raw[0])
				require.NotEqual(t, uuid.Nil, to, "unknown unit %v", raw[1])
				conversions = append(conversions, Conversion{From: from, To: to, Factor: raw[2].(float64)})
			}

			factor, ok := Factor(units, conversions, ids[c.From], ids[c.To])

			if c.Factor == nil {
				require.False(t, ok, "expected no conversion, got %v", factor)
				return
			}
			require.True(t, ok, "expected %v, got no conversion", *c.Factor)
			require.InEpsilon(t, *c.Factor, factor, 1e-9)
		})
	}
}

func TestValidateConversions(t *testing.T) {
	_, units, id := loadConversionCases(t)

	t.Run("accepts rules that are all needed", func(t *testing.T) {
		require.NoError(t, validateConversions(units, []Conversion{
			{From: id["head"], To: id["clove"], Factor: 10},
			{From: id["clove"], To: id["g"], Factor: 5},
		}))
	})

	refusals := map[string][]Conversion{
		"a standard conversion":            {{From: id["kg"], To: id["g"], Factor: 1000}},
		"one unit to itself":               {{From: id["box"], To: id["box"], Factor: 1}},
		"a zero amount":                    {{From: id["pack"], To: id["g"], Factor: 0}},
		"a negative amount":                {{From: id["pack"], To: id["g"], Factor: -1}},
		"an unknown unit":                  {{From: uuid.New(), To: id["g"], Factor: 1}},
		"the same pair twice":              {{From: id["pack"], To: id["g"], Factor: 200}, {From: id["g"], To: id["pack"], Factor: 0.005}},
		"one implied by the rest":          {{From: id["head"], To: id["clove"], Factor: 10}, {From: id["clove"], To: id["g"], Factor: 5}, {From: id["head"], To: id["kg"], Factor: 0.04}},
		"one implied with a standard step": {{From: id["pack"], To: id["g"], Factor: 200}, {From: id["pack"], To: id["kg"], Factor: 0.2}},
		"an infinite amount":               {{From: id["pack"], To: id["g"], Factor: math.Inf(1)}},
	}
	for name, conversions := range refusals {
		t.Run("refuses "+name, func(t *testing.T) {
			err := validateConversions(units, conversions)
			require.ErrorIs(t, err, ErrInvalidConversion)
			require.ErrorIs(t, err, apperrors.ErrInvalidInput)
		})
	}
}
