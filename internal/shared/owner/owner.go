// Package owner identifies whose data a request reads and writes. Until the auth module exists,
// the owner is the random device ID the app sends; logging in will later attach it to an account.
package owner

import (
	"fmt"
	"strings"

	"github.com/mamienellie/recipes-service/internal/shared/apperrors"
)

// Header is the request header that carries the owner's ID.
const Header = "X-Device-Id"

// MaxLength keeps IDs to a sane size for an indexed column.
const MaxLength = 64

// ErrMissing is returned when a request does not say who it is for.
var ErrMissing = fmt.Errorf("%w: the %s header is required", apperrors.ErrInvalidInput, Header)

// ID is the owner of a set of data.
type ID string

// Parse checks a raw header value.
func Parse(raw string) (ID, error) {
	id := strings.TrimSpace(raw)
	if id == "" || len(id) > MaxLength {
		return "", ErrMissing
	}
	return ID(id), nil
}
