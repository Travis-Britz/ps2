package wsc

import (
	"context"
	"log/slog"
	"time"
)

// WithRetry will run a Client,
// with retries on error,
// until ctx is cancelled.
//
// Connection retries will follow an exponential backoff,
// with up to 1hr between retries.
// Successful connections will reset the retry delay.
func WithRetry(c *Client, ctx context.Context) error {
	var delay time.Duration
	h := c.connectHandler
	c.connectHandler = func() {
		if h != nil {
			h()
		}
		delay = 0
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
			err := c.Run(ctx)
			select {
			case <-ctx.Done():
				return nil
			default:
				if err != nil {
					delay = delay*2 + time.Second
					if delay > time.Hour {
						delay = time.Hour
					}
					slog.Info("planetside push service disconnected", "error", err, "retry_delay", delay.String())
				}
			}

		}
	}
}
