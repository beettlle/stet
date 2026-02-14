// Package erruser provides errors whose Error() returns only a user-facing
// message; the cause is available via Unwrap() for Details or logs.
package erruser

import "errors"

// Err holds a user-facing message and an optional cause for debugging.
// Error() returns only Msg so the primary line never contains command names
// or exit codes; use Unwrap() for technical detail.
type Err struct {
	Msg string
	Err error
}

// Error returns the user-facing message only.
func (e *Err) Error() string {
	if e == nil {
		return ""
	}
	return e.Msg
}

// Unwrap returns the underlying error for Details or logging.
// Handles nil receiver (method call on nil *Err is valid in Go).
func (e *Err) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// New returns an error with the given user-facing message. If err is non-nil,
// it is wrapped and available via Unwrap() so callers can print "Details: %v".
// If err is nil, returns a simple error with just msg (no Unwrap).
func New(msg string, err error) error {
	if err == nil {
		return errors.New(msg)
	}
	return &Err{Msg: msg, Err: err}
}
