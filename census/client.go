package census

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Travis-Britz/ps2"
)

func init() {
	RateLimit(5, 2)
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

var defaultClient = &Client{Key: "example"}

var DefaultClient = defaultClient

type Client struct {
	Key string
}

func (c Client) Get(ctx context.Context, env ps2.Environment, query string, result any) error {
	select {
	case _, ok := <-RateLimiter.Ready():
		if !ok {
			return errors.New("rate limiter stopped")
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	url := fmt.Sprintf("%s/s:%s/get/%s/%s", apiBase, c.Key, Namespace(env), query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	slog.Info("census query", "url", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("census.Client.Get: client.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("census.Client.Get: returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("census.Client.Get: read body:%w", err)
	}
	if err = json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("census.Client.Get: UnmarshalJSON: %w", err)
	}
	return nil
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
