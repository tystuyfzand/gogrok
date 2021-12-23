package store

import (
	"errors"
	"time"
)

var (
	ErrNoHost = errors.New("host not found")
)

// Store represents an interface to retrieve and store hosts
type Store interface {
	Has(key string) bool
	Get(key string) (*Host, error)
	Add(host Host) error
	Remove(key string) error
}

// Host represents a claimed host
type Host struct {
	Host    string    `json:"host"`
	Owner   string    `json:"owner"`
	IP      string    `json:"ip"`
	Created time.Time `json:"created"`
	LastUse time.Time `json:"lastUse"`
}
