package store

import (
	"errors"
	"time"
)

var (
	// ErrOpenTimeout is returned when the store does not apply its initial
	// log within the specified time.
	ErrOpenTimeout = errors.New("timeout waiting for initial logs application")
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
	leaderWaitDelay     = 100 * time.Millisecond
	appliedWaitDelay    = 100 * time.Millisecond
)
