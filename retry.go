// Package retry runs a function repeatedly until it succeeds, with configurable
// attempt limits, delays, exponential backoff, jitter, and an overall time budget.
package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

type Option struct {
	MaxRetries     int                                                         // Maximum number of attempts, including the first call (default: 3)
	Delay          time.Duration                                               // Delay before the first retry (default: 1 second)
	MaxDelay       time.Duration                                               // Upper bound for a single delay when using exponential backoff (default: no limit)
	Timeout        time.Duration                                               // Total time budget for all attempts and delays (default: 5 seconds)
	UseExponential bool                                                        // Double the delay after each retry (default: false)
	UseJitter      bool                                                        // Randomize each delay between 50% and 150% (default: false)
	OnRetry        func(totalAttempt int, totalDelay time.Duration, err error) // Called after every failed attempt
}

// withDefaults returns a copy of the options with defaults filled in, so the
// caller's Option is never modified and can be shared between goroutines.
func (o *Option) withDefaults() Option {
	var opts Option
	if o != nil {
		opts = *o
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = 3
	}
	if opts.Delay <= 0 {
		opts.Delay = 1 * time.Second
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}
	return opts
}

// unrecoverableError marks an error that should not be retried.
type unrecoverableError struct{ err error }

func (e unrecoverableError) Error() string { return e.err.Error() }
func (e unrecoverableError) Unwrap() error { return e.err }

// Unrecoverable wraps err so that Do stops immediately and returns it, for
// failures that retrying can't fix (bad input, 4xx responses, not found).
// errors.Is and errors.As still see the original error.
func Unrecoverable(err error) error {
	if err == nil {
		return nil
	}
	return unrecoverableError{err: err}
}

// Do calls f until it returns nil, an unrecoverable error, the attempt limit or
// time budget is reached, or ctx is cancelled. When it gives up, the returned
// error wraps the last error from f, so errors.Is/As work on it.
func Do(ctx context.Context, f func() error, opts *Option) error {
	o := opts.withDefaults()
	if ctx == nil {
		ctx = context.Background()
	}

	var (
		start      = time.Now()
		baseDelay  = o.Delay
		totalDelay time.Duration
	)

	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("retry cancelled before attempt %d: %w", attempt, err)
		}

		err := f()
		if err == nil {
			return nil
		}

		var stop unrecoverableError
		if errors.As(err, &stop) {
			return stop.err
		}

		if o.OnRetry != nil {
			o.OnRetry(attempt, totalDelay, err)
		}

		if attempt >= o.MaxRetries {
			return fmt.Errorf("retry failed after %d attempt(s): %w", attempt, err)
		}

		delay := baseDelay
		if o.UseJitter {
			delay = time.Duration(float64(delay) * (0.5 + rand.Float64()))
		}

		if time.Since(start)+delay > o.Timeout {
			return fmt.Errorf("retry timed out after %d attempt(s) (timeout %s): %w", attempt, o.Timeout, err)
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("retry cancelled after %d attempt(s): %w (last error: %w)", attempt, ctx.Err(), err)
		case <-timer.C:
		}
		totalDelay += delay

		if o.UseExponential {
			baseDelay *= 2
			if o.MaxDelay > 0 && baseDelay > o.MaxDelay {
				baseDelay = o.MaxDelay
			}
		}
	}
}
