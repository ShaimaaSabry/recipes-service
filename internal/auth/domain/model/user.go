// Package model holds the auth module's entities and the rules that keep them valid.
package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
	"github.com/mamienellie/recipes-service/internal/shared/owner"
)

// MaxNameLength keeps a display name to something a screen can show.
const MaxNameLength = 100

// ErrInvalidUserID is returned for an ID that isn't one of ours.
var ErrInvalidUserID = fmt.Errorf("%w: user id must be a UUID", apperrors.ErrInvalidInput)

// User is an account. Everything the user saves belongs to it, by its ID.
type User struct {
	id        uuid.UUID
	email     string
	name      string
	createdAt time.Time
}

// NewUser builds a new account. The email may be empty (Apple can withhold it); the name is
// trimmed, inner runs of spaces collapsed and cut to MaxNameLength.
func NewUser(id uuid.UUID, email, name string, createdAt time.Time) *User {
	return &User{id: id, email: strings.TrimSpace(email), name: cleanName(name), createdAt: createdAt}
}

// Of rebuilds a user that was already validated, e.g. one read back from the database.
func Of(id uuid.UUID, email, name string, createdAt time.Time) *User {
	return &User{id: id, email: email, name: name, createdAt: createdAt}
}

func (u User) ID() uuid.UUID        { return u.id }
func (u User) Owner() owner.ID      { return owner.ID(u.id.String()) }
func (u User) Email() string        { return u.email }
func (u User) Name() string         { return u.name }
func (u User) CreatedAt() time.Time { return u.createdAt }

// OwnerOf is the user ID an owner stands for.
func OwnerOf(id owner.ID) (uuid.UUID, error) {
	userID, err := uuid.Parse(string(id))
	if err != nil {
		return uuid.Nil, ErrInvalidUserID
	}
	return userID, nil
}

func cleanName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if runes := []rune(name); len(runes) > MaxNameLength {
		name = strings.TrimSpace(string(runes[:MaxNameLength]))
	}
	return name
}
