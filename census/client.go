package census

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Travis-Britz/ps2"
)

func init() {
	RateLimit(2, 2)
}

var RateLimiter rateLimiter

const apiBase = "https://census.daybreakgames.com"

func Namespace(e ps2.Environment) string {
	switch e {
	case ps2.PC:
		return "ps2:v2"
	case ps2.PS4US:
		return "ps2ps4us:v2"
	case ps2.PS4EU:
		return "ps2ps4eu:v2"
	default:
		return ""
	}
}

var defaultClient = &Client{
	ServiceID: "example",

	maxRetries: 2, // a value of 2 means a total of 3 requests may be made to satisfy one call.
}

// DefaultClient is the [Client] that will be used for top-level package functions like [Get] and [GetEnv].
var DefaultClient = defaultClient

var breaker = &circuitBreaker{
	threshold: 5,
}

type Client struct {
	ServiceID string

	logf logger
	// maxRetries specifies how many times the census Client will retry after a failed request,
	// e.g. a value of 1 means if the first request fails then 1 more request will be made.
	maxRetries uint8
	env        ps2.Environment
}

// Get calls DefaultClient.Get, using the default environment.
func Get(ctx context.Context, query string, result any) error {
	return DefaultClient.Get(ctx, DefaultClient.env, query, result)
}

// GetEnv calls DefaultClient.Get.
func GetEnv(ctx context.Context, env ps2.Environment, query string, result any) error {
	return DefaultClient.Get(ctx, env, query, result)
}

// requestError wraps every error returned by census requests.
// fs:
//
//	func openFileNolog(name string, flag int, perm FileMode) (*File, error) {
//		if name == "" {
//			return nil, &PathError{Op: "open", Path: name, Err: syscall.ENOENT}
//		}
//	}
type requestError struct {
	// url is the requested url
	url string

	// e is the wrapped error
	e error

	// op is the operation. todo: figure out a format for this
	// use the name of the method?
	op string

	// responseBody
	responseBody io.Reader
}

func (e *requestError) Error() string {
	// fs.PathError.Error: func (e *PathError) Error() string { return e.Op + " " + e.Path + ": " + e.Err.Error() }

	return fmt.Sprintf("census: %s: %s; url=%q", e.op, e.e, e.url)
}

// // Timeout reports whether this error represents a timeout.
// func (e *PathError) Timeout() bool {
// 	t, ok := e.Err.(interface{ Timeout() bool })
// 	return ok && t.Timeout()
// }

func (e *requestError) Unwrap() error {
	return e.e
}

// func (e *requestError) Retryable() bool {
// 	return false
// }
// func (e *requestError) RetryAfter() time.Time {
// 	return time.Time{}
// }

// Get performs a request against the Census API.
// Response json will be unmarshaled into result.
// Retryable errors will automatically be tried up to maxAttempts.
// If all tries fail, err will contain the most recent error.
//
// Returned errors MAY implement one or both of the following interfaces:
//
//	interface { Retryable() bool }
//	interface { RetryAfter() time.Time }
//
// The presence of a Retryable method that returns true does not guarantee that a request should be retried.
// However, when Retryable returns false the request should not be retried.
// Errors are already retried internally based on Retryable when maxAttempts is configured,
// so callers should use RetryAfter to decide whether to fail or delay a retry.
// RetryAfter may return long wait times in cases like census server maintenance.
//
// It is safe to perform concurrent census requests;
// rate and concurrency limits are automatically enforced at the package level.
func (c Client) Get(ctx context.Context, env ps2.Environment, query string, result any) (err error) {
	var canRetry interface{ Retryable() bool }
	var delayRetry interface{ RetryAfter() time.Time }

	for retries := uint8(0); retries <= c.maxRetries; retries++ {
		err = c.get(ctx, env, query, result, int(retries))
		if err == nil {
			break
		}

		if retries == c.maxRetries {
			// skip checking the error result on the last attempt
			return err
		}

		if errors.As(err, &canRetry) {
			if !canRetry.Retryable() {
				return err
			}
		}
		if errors.As(err, &delayRetry) {
			if wait := time.Until(delayRetry.RetryAfter()); wait > 5*time.Second {
				// if the error can't be retried within a reasonable human-scale time frame just return the error and let the caller decide what to do.
				return err
			} else {
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return err
				}
			}
		}
	}
	return err
}
func (c Client) get(ctx context.Context, env ps2.Environment, query string, result any, retries int) (err error) {
	var url string
	timing := struct {
		fnStart      time.Time
		requestStart time.Time
		requestEnd   time.Time
	}{
		fnStart: time.Now(),
	}
	var httpResponseCode int
	var responseSize int

	// defer the logging function before any conditions that might return,
	// so that every call here is logged
	defer func() {
		logger := c.logger()
		if timing.requestStart.IsZero() {
			timing.requestStart = time.Now()
		}
		if timing.requestEnd.IsZero() {
			timing.requestEnd = time.Now()
		}
		logger.log(ctx, "census request",
			"url", url,
			slog.Group("response",
				"size", responseSize,
				"statuscode", httpResponseCode,
				"duration", timing.requestEnd.Sub(timing.requestStart),
			),
			// "http_code", httpResponseCode,
			// "response_size", responseSize,
			"wait", timing.requestStart.Sub(timing.fnStart),
			// "request_duration", timing.requestEnd.Sub(timing.requestStart),
			"attempt", retries+1,
			"error", err,
			// "parse_duration", time.Since(timing.requestEnd),
		)
	}()

	// once logging is ready and before any other conditions,
	// check if the circuit breaker has already been tripped.
	// this check should be after logging is set up so that failures are still logged,
	// but before the deferred function that might modify errors or track them towards the circuit breaker limits.
	if err = breaker.Err(); err != nil {
		return err
	}

	// now that we know the circuit breaker has been checked,
	// deferring this function allows us to check err after the function has returned.
	// this means every possible error path is covered so that we can easily let the circuit breaker keep track of errors.
	defer func() {
		err = wrapRetryableErrors(err)
		breaker.Track(err)
	}()

	select {
	case concurrentLimiter <- struct{}{}:
		// first wait for other requests to finish
		defer func() { <-concurrentLimiter }()
		select {
		case _, ok := <-RateLimiter.Ready():
			// then wait for the rate limiter
			if !ok {
				return errors.New("rate limiter stopped")
			}
		case <-ctx.Done():
			return fmt.Errorf("waiting for rate limiter: %w", ctx.Err())

		}
	case <-ctx.Done():
		return fmt.Errorf("waiting for other requests to finish: %w", ctx.Err())
	}

	var waitduration time.Duration
	switch retries {
	case 0:
		// On the first try we want to fail fast so that we are not blocking the request queue.
		// We may have hit a slow/bad load balancer,
		// we may be stuck behind a long query,
		// or Census might be in a Java garbage collection pause.
		waitduration = 3 * time.Second
	case 1:
		// Since we have already tried once and failed,
		// we'll give the second attempt a little more time.
		// If our query was the slow one it may be cached by now,
		// or it may take a little longer to complete.
		waitduration = 15 * time.Second
	default:
		// If we have tried more than twice then we will wait the maximum allowed time.
		// Ideally this duration would be much shorter,
		// but it is possible for some very complicated queries to take a long time to execute.
		// Census caches identical queries,
		// so if our query was the slow one then hopefully it is actually cached by now and will return quickly anyway.
		waitduration = censusTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, waitduration)
	defer cancel()
	url = fmt.Sprintf("%s/s:%s/get/%s/%s", apiBase, c.ServiceID, Namespace(env), query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	timing.requestStart = time.Now()
	resp, err := http.DefaultClient.Do(req)
	timing.requestEnd = time.Now()
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	httpResponseCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		// even internal server errors return "200 OK" with an errorCode json field.
		return fmt.Errorf("returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	responseSize = len(body)

	// Planetside's api follows the philosophy that the HTTP protocol is a transport layer.
	// Any HTTP code other than 200 indicates something went wrong in transport and should generally be treated the same as if a TCP connection had an error.
	// The application-level error codes are returned in the json response body of an HTTP 200 response.
	//
	// Anything with the "error" key is equivalent to an HTTP 4XX (bad client request).
	//
	// If the keys "errorCode" and "errorMessage" are both present then it is similar to an HTTP 5XX response.
	// These are uncaught Java exceptions that were rendered by the framework.
	// In practice I have only observed this case for queries that broke parsing,
	// so retrying the same request will generally still fail with the same error.
	// This is different than "bad request syntax",
	// which is only returned if the census parser completed without throwing an exception.

	// From MangoBean on the PlanetSide developers discord:
	// "No data found" means the collection does not exist.
	// "Service unavailable" means the collection is valid,
	// but the data source backing is in a bad state
	// (Usually this means a DB is down, or some data source cannot be reached).
	// A redirect means the Apache load balancer couldn't find a Tomcat server to forward the request to.
	// https://discord.com/channels/1019343142471880775/1019509468754608168/1278010950913359953
	errorResponse := struct {
		Error        string `json:"error"`
		ErrorCode    string `json:"errorCode"`
		ErrorMessage string `json:"errorMessage"`
	}{}
	if err = json.Unmarshal(body, &errorResponse); err != nil {
		if bytes.Contains(bytes.TrimSpace(body)[:512], []byte("<html")) {
			c.logger().log(ctx, "census returned an unusual response", "final_url", resp.Request.URL.String(), "body_truncated", string(body[:512]))
		}
		if resp.Request.URL.Host == "www.daybreakgames.com" && resp.Request.URL.Path == "/home" {
			// Census has been observed to redirect to the daybreak homepage during maintenance,
			// which contains normal HTML.
			// This check only works if the provided http.Client follows redirects,
			// which it does by default.
			return errServerMaintenance
		}
		return errBadJSON(err)
	}

	// if the error field is present then this is a normal error response
	if errorResponse.Error != "" {
		if strings.HasPrefix(errorResponse.Error, "Missing Service ID") {
			return errRateLimitExceeded
		}
		if strings.HasPrefix(errorResponse.Error, "Provided Service ID is not registered") {
			return ErrBadServiceID
		}
		if errorResponse.Error == "Bad request syntax." {
			return errBadRequestSyntax
		}
		if errorResponse.Error == "No data found." {
			return errNotFound
		}
		if errorResponse.Error == "service_unavailable" {
			return errServiceUnavailable
		}

		return genericServerError(errorResponse.Error)
	}

	// if the errorCode field is present then this is a java exception
	if errorResponse.ErrorCode != "" {
		return genericInternalServerError{
			errorCode:    errorResponse.ErrorCode,
			errorMessage: errorResponse.ErrorMessage,
		}
	}

	if err = json.Unmarshal(body, result); err != nil {
		// json decoding errors like html in the body would have been caught already when unmarshaling the errorResponse struct.
		// If an error occurs at this stage,
		// then it's likely to be caused by result implementing json.Unmarshaler and returning an error.
		// Another request would probably result in the same response body
		// (except for hitting dynamic collections or different load balancer caches),
		// so it's better to skip retries.
		return permanentError{errBadJSON(err)}
	}
	return nil
}

// SetLog sets the log function the client will use when making requests.
//
//	client := &census.Client{Key:"example"}
//	client.SetLog(slog.DebugContext)
func (c *Client) SetLog(fn func(ctx context.Context, msg string, args ...any)) {
	c.logf = fn
}

func (c Client) logger() logger {
	if c.logf == nil {
		return func(context.Context, string, ...any) {}
	}
	return c.logf
}

// logger is a function such as [slog.InfoContext]
type logger func(ctx context.Context, msg string, args ...any)

func (f logger) log(ctx context.Context, msg string, args ...any) {
	f(ctx, msg, args...)
}

// concurrentLimiter limits the number of census requests that can be in-flight concurrently.
// This is intentionally not configurable.
// Two concurrent requests allows up to one to be stuck for a few moments waiting to be handled by a load balancer
// without blocking the next request (which might hit a different load balancer).
// Requests are ultimately still limited by the ratelimiter once the burst request limit is exhausted.
var concurrentLimiter chan struct{} = make(chan struct{}, 2)

// RateLimit sets the global rate limiter used by every Client.
// burst sets the number of requests that can be sent initially without throttling,
// and nPerSec defines how many requests can be made per second after that.
//
// To use a custom rate limiter,
// set the package RateLimiter variable.
//
// Note that regardless of how high the rate limit is set,
// the concurrency limit remains at two in-flight requests.
func RateLimit(burst, nPerSec int) {
	stopLastLimiter()
	if burst < 1 {
		burst = 1
	}
	limiter := make(rateLimit, burst-1) //nPerSec-1 because we assume at the start of a burst there will already be a waiting send from ticker
	ticker := time.NewTicker(time.Second / time.Duration(nPerSec))
	stopLastLimiter = ticker.Stop
	RateLimiter = limiter
	for range burst - 1 {
		limiter <- struct{}{}
	}
	go func() {
		for range ticker.C {
			limiter <- struct{}{}
		}
	}()
}

var stopLastLimiter func() = func() {}

type rateLimiter interface {
	Ready() <-chan struct{}
}

type rateLimit chan struct{}

func (limit rateLimit) Ready() <-chan struct{} {
	return limit
}

type circuitBreaker struct {
	mu         sync.Mutex
	err        error // a non-nil error indicates an "open" (tripped) circuit breaker
	threshold  int   // number of consecutive errors required to trip the circuit breaker
	errorCount int
	resetAfter time.Time
}

// Err returns a non-nil error when the circuit breaker has been tripped.
func (breaker *circuitBreaker) Err() error {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	if time.Now().After(breaker.resetAfter) {
		// we want the error to reset after a specified time,
		// but leave errorCount so that another error after the reset will immediately trip the breaker.
		breaker.err = nil
	}

	return breaker.err
}

// Track inspects errors and trips the circuit breaker when specific conditions are met.
// Consecutive errors increase the error count.
// nil errors reset the error count.
// Some errors, such as errServerMaintenance, may trip the breaker immediately.
func (breaker *circuitBreaker) Track(err error) {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	if err == nil {
		breaker.errorCount = 0
		breaker.resetAfter = time.Now()
		return
	}

	if errors.Is(err, errServerMaintenance) {
		breaker.err = err
		breaker.resetAfter = time.Now().Add(15 * time.Minute)
		return
	}
	if errors.Is(err, errRateLimitExceeded) {
		breaker.err = err
		breaker.resetAfter = time.Now().Add(2 * time.Minute)
		return
	}
	breaker.errorCount++
	if breaker.errorCount > breaker.threshold {
		const tripDuration = 15 * time.Minute
		breaker.err = retryableError{
			errShortCircuit,
			time.Now().Add(tripDuration),
		}
		breaker.resetAfter = time.Now().Add(tripDuration)
	}
}

// censusTimeout is the maximum duration to wait for a request to complete once it has started.
// Ideally this duration would be much shorter to fail/retry early,
// however some complicated Census queries may actually return successfully after a long time.
const censusTimeout = 31 * time.Second

// ServiceID configures the default client's service ID.
func ServiceID(s string) {
	defaultClient.ServiceID = s
}
