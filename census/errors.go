package census

import (
	"fmt"
	"time"
)

type noResults struct {
	q string
}

func (r noResults) Error() string {
	return fmt.Sprintf("no results for %q", r.q)
}

type possibleMaintenance struct {
	e error
}

func (e possibleMaintenance) Error() string {
	return e.e.Error()
}

func (e possibleMaintenance) Unwrap() error {
	return e.e
}

type retryable struct {
	e         error
	waitUntil time.Time
}

func (e retryable) Temporary() bool {
	return true
}

func (e retryable) Unwrap() error { return e.e }
