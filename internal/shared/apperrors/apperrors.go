// Package apperrors holds the error kinds every module's domain can return. The HTTP layer maps
// each kind to a status code, so domain code never needs to know about HTTP.
package apperrors

import "errors"

var (
	// ErrInvalidInput means the request breaks a business rule, e.g. a recipe without a name (400).
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound means the record does not exist for this owner (404).
	ErrNotFound = errors.New("not found")
	// ErrConflict means the request clashes with data already saved, e.g. a duplicate name (409).
	ErrConflict = errors.New("conflict")
	// ErrUnauthorized means the request doesn't prove who it's from, e.g. a missing or expired
	// token (401).
	ErrUnauthorized = errors.New("unauthorized")
	// ErrUnavailable means a feature isn't set up where the service is running (503).
	ErrUnavailable = errors.New("unavailable")
)
