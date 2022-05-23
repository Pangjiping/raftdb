package store

import (
	"errors"
)

var (
	// ErrOpenTimeout is returned when the store does not apply its initial
	// log within the specified time.
	ErrOpenTimeout = errors.New("timeout waiting for initial logs application")
)
