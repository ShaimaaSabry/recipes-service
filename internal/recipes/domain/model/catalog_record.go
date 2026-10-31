package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// Source says who made a category or grocery item.
type Source string

const (
	// SourceSystem records were copied from the starter data. They can be hidden, not deleted.
	SourceSystem Source = "system"
	// SourceUser records were made by the account's owner. They can be deleted.
	SourceUser Source = "user"
)

const (
	MaxNameLength  = 100
	MaxEmojiLength = 16
)

// normalizeName trims a name and collapses inner runs of spaces.
func normalizeName(name string) string {
	return strings.Join(strings.Fields(name), " ")
}

func validateNameAndEmoji(kind, name, emoji string) error {
	if name == "" {
		return fmt.Errorf("%w: please give the %s a name", apperrors.ErrInvalidInput, kind)
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return fmt.Errorf("%w: the %s's name must be at most %d characters", apperrors.ErrInvalidInput, kind, MaxNameLength)
	}
	if emoji == "" || utf8.RuneCountInString(emoji) > MaxEmojiLength {
		return fmt.Errorf("%w: please give the %s an emoji", apperrors.ErrInvalidInput, kind)
	}
	return nil
}

// NameTaken is the error for a name another record of the same kind already has. Names are
// compared ignoring case, hidden records included: a hidden one should be shown again instead.
func NameTaken(kind, name string, hidden bool) error {
	if hidden {
		return fmt.Errorf("%w: a hidden %s called “%s” already exists. Show it again instead", apperrors.ErrConflict, kind, name)
	}
	return fmt.Errorf("%w: a %s called “%s” already exists", apperrors.ErrConflict, kind, name)
}

// SameName says whether two names count as the same.
func SameName(a, b string) bool {
	return strings.EqualFold(normalizeName(a), normalizeName(b))
}

func refuseStarterDelete(source Source, name string) error {
	if source == SourceSystem {
		return fmt.Errorf("%w: “%s” came with the app, so it can't be deleted. Hide it instead", apperrors.ErrConflict, name)
	}
	return nil
}

var (
	ErrCategoryNotFound = fmt.Errorf("%w: no such category", apperrors.ErrNotFound)
	ErrItemNotFound     = fmt.Errorf("%w: no such grocery item", apperrors.ErrNotFound)
	// ErrNoSuchCategory is an item naming a category the owner doesn't have.
	ErrNoSuchCategory = fmt.Errorf("%w: please choose a category", apperrors.ErrInvalidInput)
)
