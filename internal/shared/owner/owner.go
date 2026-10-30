// Package owner identifies whose data a request reads and writes: the signed-in account. Every
// module stores it with the data it owns, and httpserver.RequireOwner gives it to the handlers.
package owner

// ID is the owner of a set of data: the account's ID.
type ID string
