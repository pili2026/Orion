// Package apperr defines sentinel errors shared across the application layers.
// Handlers use errors.Is(err, apperr.ErrNotFound) to map service errors to HTTP status codes.
package apperr

import "errors"

// ErrNotFound is returned by service methods when the requested resource does not exist.
var ErrNotFound = errors.New("not found")
