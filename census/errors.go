package census

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

type noResultsError struct {
	q string
}

func (r noResultsError) Error() string {
	return fmt.Sprintf("no results for %q", r.q)
}

type retryableError struct {
	error
	waitUntil time.Time
}

func (retryableError) Retryable() bool {
	return true
}

func (err retryableError) RetryAfter() time.Time {
	return err.waitUntil
}
func (err retryableError) Unwrap() error { return err.error }

// genericServerError is used when the server returns a well-formed error message for an error case we don't know about.
type genericServerError string

func (e genericServerError) Error() string {
	return string(e)
}

// genericInternalServerError is used when census returns a well-formed error message.
// My guess is that this format only shows up during internal uncaught exceptions,
// which would only happen for unsupported/invalid request syntax.
// known error codes: "SERVER_ERROR"
// I don't know if there are other error codes that can be returned in this format.
type genericInternalServerError struct {
	code    string
	message string
}

func (e genericInternalServerError) Error() string {
	return fmt.Sprintf("%s: %s", e.code, e.message)
}
func (genericInternalServerError) Retryable() bool { return false }

func errBadJSON(err error) error {
	return fmt.Errorf("unmarshal json: %w", err)
}

func errRateLimit() error {
	return retryableError{
		errRateLimitExceeded,
		time.Now().Add(1 * time.Minute),
	}
}

// errRateLimitExceeded is returned when too many requests are sent without a service ID.
// Census does not have rate limits when a service ID is provided.
var errRateLimitExceeded = errors.New("rate limit exceeded")

type permanentError struct {
	error
}

func (permanentError) Retryable() bool { return false }
func (e permanentError) Unwrap() error { return e.error }

// errBadServiceID is returned when a service ID is provided but not registered.
// It may be due to the ID being deleted
var errBadServiceID = permanentError{errors.New("provided service ID is not registered")}

// errBadRequestSyntax is returned when requests are malformed,
// like syntax errors in the query string.
// It's actually a 302 redirect to a file called badrequest.json.
var errBadRequestSyntax = permanentError{errors.New("bad request syntax")}

// errNotFound is known to be returned when a collection name is given that doesn't exist.
var errNotFound = permanentError{errors.New("request path not found")}

// errServerMaintenance is returned when a response is likely to be caused by census being down during maintenance.
var errServerMaintenance = errors.New("server maintenance")

// errShortCircuit is returned when a treshold of errors is reached
var errShortCircuit = errors.New("there is a problem with the system - try again later")

func errMaintenance() error {
	return retryableError{
		errServerMaintenance,
		time.Now().Add(30 * time.Minute),
	}
}

// wrapRetryableErrors wraps context.DeadlineExceeded and network errors with a retryable error.
func wrapRetryableErrors(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return retryableError{
			err,
			time.Now(),
		}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return retryableError{
			err,
			time.Now(),
		}
	}
	return err
}
