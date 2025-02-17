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
// This format only shows up during internal uncaught exceptions,
// which would only happen for unsupported/invalid request syntax.
// known error codes: "SERVER_ERROR"
// I don't know if there are other error codes that can be returned in this format.
// accessing nested distinct keys that don't exist
//
// 200 OK
// {
// "errorCode": "SERVER_ERROR",
// "errorMessage": "INVALID_SEARCH_TERM: Invalid distinct value"
// }
// {
// "errorCode": "SERVER_ERROR"
// }
type genericInternalServerError struct {
	errorCode    string
	errorMessage string
}

func (e genericInternalServerError) Error() string {
	if e.errorMessage != "" {
		return fmt.Sprintf("%s: %s", e.errorCode, e.errorMessage)
	}
	return "code: " + e.errorCode
}
func (genericInternalServerError) Retryable() bool { return false }

func errBadJSON(err error) error {
	return fmt.Errorf("unmarshal json: %w", err)
}

// errRateLimitExceeded is returned when too many requests are sent without a service ID.
// Census does not have rate limits when a service ID is provided.
// 200 OK
// {
// "error": "Missing Service ID.  A valid Service ID is required for continued api use.  The Service ID s:example is for casual use only.  (http://census.daybreakgames.com/#devSignup)"
// }
var errRateLimitExceeded = errors.New("rate limit exceeded")

type permanentError struct {
	error
}

func (permanentError) Retryable() bool { return false }
func (e permanentError) Unwrap() error { return e.error }

// ErrBadServiceID is returned when a service ID is provided but not registered.
// If it's not a typo,
// then it may be due to the ID being deleted.
// {
// "error": "Provided Service ID is not registered.  A valid Service ID is required for continued api use. (http://census.daybreakgames.com/#devSignup)"
// }
var ErrBadServiceID = permanentError{errors.New("provided service ID is not registered")}

// errBadRequestSyntax is returned when requests are malformed,
// like syntax errors in the query string.
// It's actually a 302 redirect to a file called badrequest.json.
// 302 -> badrequest.json
// {
// "error": "Bad request syntax."
// }
var errBadRequestSyntax = permanentError{errors.New("bad request syntax")}

// errNotFound is known to be returned when a collection name is given that doesn't exist.
// collection typo
// 200 OK
// {
// "error": "No data found."
// }
var errNotFound = permanentError{errors.New("request path not found")}

// errServerMaintenance is returned when a response is likely to be caused by census being down during maintenance.
// This is only checked with heuristics,
// since as far as I know there is no official error code for this state.
var errServerMaintenance = errors.New("server maintenance")

// errShortCircuit is returned when a treshold of errors is reached
var errShortCircuit = errors.New("short circuit: too many failures detected")

// errServiceUnavailable is returned when a valid collection was queried but the data source is in a bad state
// (like a DB down).
// Also observed by the /map endpoint during game updates when the game worlds are down,
// because the game worlds are the backing data sournce for that endpoint.
// {
// "error": "service_unavailable"
// }
var errServiceUnavailable = errors.New("service unavailable")

// wrapRetryableErrors wraps known error types with Retryable.
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
	if errors.Is(err, errServerMaintenance) {
		return retryableError{
			errServerMaintenance,
			time.Now().Add(30 * time.Minute),
		}
	}
	if errors.Is(err, errRateLimitExceeded) {
		return retryableError{
			errRateLimitExceeded,
			time.Now().Add(1 * time.Minute),
		}
	}
	if errors.Is(err, errServiceUnavailable) {
		// since this error can be returned by a bad database connection on the census side,
		// retrying should be fine because the load balancer might give us a different backend the next time.
		// Also, this error might only apply to specific endpoints depending on which data source is backing them.
		// If the entire service is having issues then the circuit breaker will trip anyway.
		return retryableError{
			err,
			time.Now(),
		}
	}
	return err
}
