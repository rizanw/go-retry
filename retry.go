package retry

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"
)

type Option struct {
	MaxRetries     int                                                         // Maximum number of retry attempts (default: 3)
	Delay          time.Duration                                               // Initial delay between retries (default: 1 second)
	Timeout        time.Duration                                               // Total timeout for retries (default: 5 seconds)
	UseExponential bool                                                        // Enable exponential backoff (default: false)
	UseJitter      bool                                                        // Add random jitter to the delay (default: false)
	OnRetry        func(totalAttempt int, totalDelay time.Duration, err error) // Callback function for custom retry event handling
}

// fillDefault will set required options with default value if it is not set.
func (o *Option) fillDefault() {
	if o.MaxRetries <= 0 {
		o.MaxRetries = 3
	}
	if o.Delay <= 0 {
		o.Delay = 1 * time.Second
	}
	if o.Timeout <= 0 {
		o.Timeout = 5 * time.Second
	}
}

// Do attempts to execute the provided function 'f' multiple times with retry logic.
// It will retry the function execution based on the specified options.
func Do(ctx context.Context, f func() error, opts *Option) error {
	if opts == nil {
		opts = &Option{}
	}
	opts.fillDefault()

	var (
		attempts   = 0
		totalDelay time.Duration
		delay      = opts.Delay
	)

	for {
		attempts++
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled at %d attempt(s): %w", attempts, ctx.Err())
		default:
		}

		err := f()
		if err == nil {
			if attempts > 1 {
				log.Printf("[Retry] Attempt succeeded after %d attempt(s)\n", attempts)
			}
			return nil
		}

		if opts.OnRetry != nil {
			opts.OnRetry(attempts, totalDelay, err)
		}

		if attempts >= opts.MaxRetries {
			return fmt.Errorf("retry failed after %d attempt(s) with total delay: %fs", attempts, totalDelay.Seconds())
		}
		if totalDelay >= opts.Timeout {
			return fmt.Errorf("retry failed after reach timeout(%fs) with %d attempt(s) ", opts.Timeout.Seconds(), attempts)
		}

		if opts.UseJitter {
			jitter := rand.Float64()*1.0 + 0.5
			delay = time.Duration(float64(delay) * jitter)
		}
		totalDelay += delay
		time.Sleep(delay)
		if opts.UseExponential {
			delay = delay * 2
		}
	}
}
