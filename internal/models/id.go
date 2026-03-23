package models

import (
	"crypto/rand"
	"time"

	"github.com/oklog/ulid/v2"
)

// NewID generates a new ULID string.
func NewID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Now returns the current time in UTC ISO 8601 format.
func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
