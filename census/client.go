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
	Key:        "example",
	maxRetries: 1,
}

var DefaultClient = defaultClient

var breaker = &circuitBreaker{
	threshold: 5,
}

type Client struct {
	Key string

	logf       logger
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

	for retries := 0; retries <= int(c.maxRetries); retries++ {
		err = c.get(ctx, env, query, result, retries)

		if retries == int(c.maxRetries)-1 {
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
				// if the error can't be retried within a reasonable time frame just return the error and let the caller decide what to do.
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
	defer func() {
		logger := c.logger()
		if timing.requestStart.IsZero() {
			timing.requestStart = time.Now()
		}
		if timing.requestEnd.IsZero() {
			timing.requestEnd = time.Now()
		}
		logger.log("census.Client.Get",
			"url", url,
			"error", err,
			"http_code", httpResponseCode,
			"response_size", responseSize,
			"wait", timing.requestStart.Sub(timing.fnStart),
			"request_duration", timing.requestEnd.Sub(timing.requestStart),
			"retries", retries,
			// "parse_duration", time.Since(timing.requestEnd),
		)
	}()
	if err = breaker.Err(); err != nil {
		// check the circuit breaker after deferring the logging call
		// and before tracking the error with the circuit breaker again
		return err
	}
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
	ctx, cancel := context.WithTimeout(ctx, censusTimeout)
	defer cancel()
	url = fmt.Sprintf("%s/s:%s/get/%s/%s", apiBase, c.Key, Namespace(env), query)
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
			slog.Debug("unusual response", "final_url", resp.Request.URL.String(), "body", string(body[:512]))
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
			return errBadServiceID
		}
		if errorResponse.Error == "Bad request syntax." {
			return errBadRequestSyntax
		}
		if errorResponse.Error == "No data found." {
			return errNotFound
		}
		if strings.ToLower(errorResponse.Error) == "service unavailable" {
			// a reported error condition, not personally observed yet
			return errServiceUnavailable
		}
		return genericServerError(errorResponse.Error)
	}

	// if the errorCode field is present then this is a java exception
	if errorResponse.ErrorCode != "" {
		return genericInternalServerError{
			code:    errorResponse.ErrorCode,
			message: errorResponse.ErrorMessage,
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
//	client.SetLog(slog.Info)
func (c *Client) SetLog(fn func(msg string, args ...any)) {
	c.logf = fn
}

func (c Client) logger() logger {
	if c.logf == nil {
		return func(msg string, args ...any) {}
	}
	return c.logf
}

// logger is a function such as [slog.Info]
type logger func(msg string, args ...any)

func (f logger) log(msg string, args ...any) {
	f(msg, args...)
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
		breaker.resetAfter = time.Now().Add(30 * time.Minute)
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
const censusTimeout = 30 * time.Second

// ServiceID configures the default client's service ID.
func ServiceID(s string) {
	defaultClient.Key = s
}
