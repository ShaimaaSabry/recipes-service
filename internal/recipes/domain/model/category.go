package model

import (
	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// Category groups grocery items on the list ("Produce", "Dairy & Eggs").
type Category struct {
	id        uuid.UUID
	owner     owner.ID
	name      string
	emoji     string
	source    Source
	systemKey string
	hidden    bool
}

// NewCategory validates a category the owner makes.
func NewCategory(id uuid.UUID, owner owner.ID, name, emoji string) (*Category, error) {
	c := &Category{id: id, owner: owner, source: SourceUser}
	if err := c.Edit(name, emoji); err != nil {
		return nil, err
	}
	return c, nil
}

// NewStarterCategory validates a category copied from the starter data entry systemKey.
func NewStarterCategory(id uuid.UUID, owner owner.ID, systemKey, name, emoji string) (*Category, error) {
	c := &Category{id: id, owner: owner, source: SourceSystem, systemKey: systemKey}
	if err := c.Edit(name, emoji); err != nil {
		return nil, err
	}
	return c, nil
}

// CategoryOf rebuilds a category that was already validated, e.g. one read back from the database.
func CategoryOf(id uuid.UUID, owner owner.ID, name, emoji string, source Source, systemKey string, hidden bool) *Category {
	return &Category{id: id, owner: owner, name: name, emoji: emoji, source: source, systemKey: systemKey, hidden: hidden}
}

func (c Category) ID() uuid.UUID     { return c.id }
func (c Category) Owner() owner.ID   { return c.owner }
func (c Category) Name() string      { return c.name }
func (c Category) Emoji() string     { return c.emoji }
func (c Category) Source() Source    { return c.source }
func (c Category) SystemKey() string { return c.systemKey }
func (c Category) Hidden() bool      { return c.hidden }

// Edit renames the category. Starter categories can be edited too, and stay starter ones.
func (c *Category) Edit(name, emoji string) error {
	name = normalizeName(name)
	if err := validateNameAndEmoji("category", name, emoji); err != nil {
		return err
	}
	c.name, c.emoji = name, emoji
	return nil
}

// SetHidden hides the category from pickers, or shows it again. Its items stay where they are.
func (c *Category) SetHidden(hidden bool) { c.hidden = hidden }

// CanDelete refuses for a starter category, which can only be hidden.
func (c Category) CanDelete() error { return refuseStarterDelete(c.source, c.name) }
