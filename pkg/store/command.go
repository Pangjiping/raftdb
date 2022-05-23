package store

type command struct {
	Op    string `json:"op,omitempty"`
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type ConsistencyLevel int

// Represents the available consistency levels.
const (
	Default ConsistencyLevel = iota
	Stale
	Consistent
)
