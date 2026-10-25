// Package model holds the recipes module's entities and the rules that keep them valid.
package model

import (
	"fmt"
	"strings"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// MaxIDLength matches the id columns. IDs come from the app, which creates them while offline.
const MaxIDLength = 64

var (
	ErrInvalidRecipeName = fmt.Errorf("%w: please give the recipe a name", apperrors.ErrInvalidInput)
	ErrInvalidID         = fmt.Errorf("%w: id must be 1 to %d characters", apperrors.ErrInvalidInput, MaxIDLength)
)

// Recipe is a dish the owner cooks.
type Recipe struct {
	id    string
	owner owner.ID
	name  string
}

// NewRecipe validates a new recipe. The name is trimmed and inner runs of spaces collapsed.
func NewRecipe(id string, owner owner.ID, name string) (*Recipe, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > MaxIDLength {
		return nil, ErrInvalidID
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return nil, ErrInvalidRecipeName
	}
	return &Recipe{id: id, owner: owner, name: name}, nil
}

// Of rebuilds a recipe that was already validated, e.g. one read back from the database.
func Of(id string, owner owner.ID, name string) *Recipe {
	return &Recipe{id: id, owner: owner, name: name}
}

func (r Recipe) ID() string      { return r.id }
func (r Recipe) Owner() owner.ID { return r.owner }
func (r Recipe) Name() string    { return r.name }
