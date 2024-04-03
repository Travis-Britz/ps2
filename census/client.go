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

// concurrentLimiter limits the number of census requests that can be in-flight concurrently.
var concurrentLimiter chan struct{} = make(chan struct{}, 2)

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
	Key: "example",
}

var DefaultClient = defaultClient

var breaker = newCircuitBreaker()

type Client struct {
	Key string

	// logf is a logger function such as slog.Debug
	logf func(msg string, args ...any)
}

func (c Client) Get(ctx context.Context, env ps2.Environment, query string, result any) (err error) {
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
		breaker.Track(err)
	}()
	defer func() {
		err = wrapRetryableErrors(err)
	}()
	defer func() {
		if c.logf == nil {
			return
		}
		if timing.requestStart.IsZero() {
			timing.requestStart = time.Now()
		}
		if timing.requestEnd.IsZero() {
			timing.requestEnd = time.Now()
		}
		c.logf("census.Client.Get",
			"url", url,
			"error", err,
			"http_code", httpResponseCode,
			"response_size", responseSize,
			"wait", timing.requestStart.Sub(timing.fnStart),
			"request_duration", timing.requestEnd.Sub(timing.requestStart),
			// "parse_duration", time.Since(timing.requestEnd),
		)
	}()
	if err = breaker.Err(); err != nil {
		return err
	}
	select {
	case _, ok := <-RateLimiter.Ready():
		if !ok {
			return errors.New("rate limiter stopped")
		}
		select {
		case concurrentLimiter <- struct{}{}:
			defer func() { <-concurrentLimiter }()
		case <-ctx.Done():
			return fmt.Errorf("waiting for other requests to finish: %w", ctx.Err())
		}
	case <-ctx.Done():
		return fmt.Errorf("waiting for rate limiter: %w", ctx.Err())
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
		return fmt.Errorf("returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	responseSize = len(body)
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
			// This only works if the provided http.Client follows redirects,
			// which it does by default.
			return errMaintenance()
		}
		return errBadJSON(err)
	}

	if errorResponse.Error != "" {
		if strings.HasPrefix(errorResponse.Error, "Missing Service ID") {
			return errRateLimit()
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
		return genericServerError(errorResponse.Error)
	}
	if errorResponse.ErrorCode != "" {
		return genericInternalServerError{errorResponse.ErrorCode, errorResponse.ErrorMessage}
	}

	if err = json.Unmarshal(body, result); err != nil {
		return errBadJSON(err)
	}
	return nil
}

// SetLog sets the log function the client will use when making requests.
//
//	client := &census.Client{Key:"example"}
//	client.SetLog(slog.Info)
func (c *Client) SetLog(logf func(msg string, args ...any)) {
	if logf == nil {
		c.logf = func(msg string, args ...any) {}
		return
	}
	c.logf = logf
}

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

func newCircuitBreaker() *circuitBreaker {
	breaker := &circuitBreaker{
		threshold: 5,
	}
	return breaker
}

type circuitBreaker struct {
	mu         sync.Mutex
	err        error
	errorCount int
	threshold  int
	resetAfter time.Time
}

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

func (breaker *circuitBreaker) Track(err error) {
	breaker.mu.Lock()
	defer breaker.mu.Unlock()

	if err == nil {
		breaker.errorCount = 0
		breaker.resetAfter = time.Now()
		return
	}

	if errors.Is(err, errServerMaintenance) {
		breaker.err = errMaintenance()
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
