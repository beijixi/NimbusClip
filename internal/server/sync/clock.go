package sync

import "time"

// Clock abstracts time retrieval for synchronization decisions.
type Clock interface {
	Now() time.Time
}

// SystemClock is a production Clock implementation using the real clock.
type SystemClock struct{}

// Now returns the current UTC time.
func (SystemClock) Now() time.Time {
	return time.Now().UTC()
}
