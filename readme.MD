# go-retry

[![Go Reference](https://pkg.go.dev/badge/github.com/rizanw/go-retry.svg)](https://pkg.go.dev/github.com/rizanw/go-retry)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A small, dependency-free Go package for retrying operations that can fail temporarily, such as network calls, message publishing, or database writes. It supports fixed delays, exponential backoff with an optional cap, jitter, an overall time budget, context cancellation, and errors that should not be retried.

```go
err := retry.Do(ctx, func() error {
    return publish(msg)
}, &retry.Option{MaxRetries: 5, UseExponential: true, UseJitter: true})
```

## Installation

```bash
go get github.com/rizanw/go-retry
```

Requires Go 1.20 or newer.

## Features

- **Attempt limit**: stop after a maximum number of attempts
- **Fixed or exponential delay**: wait the same time between attempts, or double it each time, with an optional `MaxDelay` cap
- **Jitter**: randomize each delay so many clients don't retry in lockstep
- **Time budget**: stop when the next retry would go past `Timeout`
- **Context-aware**: cancelling `ctx` stops retrying immediately, even in the middle of a delay
- **Unrecoverable errors**: wrap an error with `retry.Unrecoverable` to stop right away
- **Useful errors**: the final error wraps the last failure, so `errors.Is` and `errors.As` work
- **Hooks**: `OnRetry` is called after every failed attempt, for logging or metrics

## Usage

```go
import (
    "context"
    "time"

    "github.com/rizanw/go-retry"
)

err := retry.Do(ctx, func() error {
    return publishData() // any operation that may fail
}, &retry.Option{
    MaxRetries:     4,
    Delay:          500 * time.Millisecond,
    Timeout:        10 * time.Second,
    UseExponential: true,
    UseJitter:      true,
    OnRetry: func(attempt int, totalDelay time.Duration, err error) {
        log.Printf("attempt %d failed (waited %s so far): %v", attempt, totalDelay, err)
    },
})
if err != nil {
    // every attempt failed; err wraps the last error from publishData
}
```

Pass `nil` as the options to use the defaults.

## Options

| Field | Default | Description |
|---|---|---|
| `MaxRetries` | 3 | maximum number of attempts, **including the first call** |
| `Delay` | 1s | wait before the first retry |
| `MaxDelay` | none | upper bound for a single delay when `UseExponential` is on |
| `Timeout` | 5s | total time budget; Do stops if the next retry would start after it |
| `UseExponential` | false | double the delay after each retry |
| `UseJitter` | false | randomize each delay to between 50% and 150% of its value |
| `OnRetry` | nil | `func(attempt int, totalDelay time.Duration, err error)`, called after every failed attempt |

`Timeout` and `MaxRetries` both apply, and whichever is reached first ends the retries. Make sure `Timeout` is long enough for the delays you configure: with the defaults (3 attempts, 1s delay) the last attempt starts at 2s, well inside 5s, but 5 attempts with exponential backoff from 1s need 15s.

`Timeout` doesn't interrupt a call that's already running. To bound each individual call, give `f` its own context with a deadline, and use `ctx` to bound the whole operation.

## Errors

| Situation | Returned error |
|---|---|
| `f` succeeds | `nil` |
| all attempts fail | wraps the last error from `f` |
| time budget reached | wraps the last error from `f` |
| `ctx` cancelled or expired | wraps both `ctx.Err()` and the last error from `f` |
| `f` returns `retry.Unrecoverable(err)` | `err` itself, after a single attempt |

```go
if errors.Is(err, context.Canceled) {
    // the caller gave up
}
var apiErr *APIError
if errors.As(err, &apiErr) {
    // inspect the last failure
}
```

### Don't retry what can't succeed

Some failures won't change on a retry, such as invalid input, a `400 Bad Request`, or a `404 Not Found`. Wrap them with `retry.Unrecoverable` to stop immediately and return the original error:

```go
err := retry.Do(ctx, func() error {
    resp, err := client.Do(req)
    if err != nil {
        return err // network error: retry
    }
    defer resp.Body.Close()

    switch {
    case resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests:
        return fmt.Errorf("server error: %s", resp.Status) // retry
    case resp.StatusCode >= 400:
        return retry.Unrecoverable(fmt.Errorf("client error: %s", resp.Status)) // don't retry
    }
    return nil
}, &retry.Option{MaxRetries: 4, UseExponential: true})
```

## Choosing a strategy

### Fixed delay

Wait the same amount of time between attempts. Simple and predictable, and fine for a single client or short outages. With many clients it can keep hammering a struggling service at a steady rate.

```go
// 4 attempts, 2s apart: at 0s, 2s, 4s, 6s
opts := &retry.Option{
    MaxRetries: 4,
    Delay:      2 * time.Second,
    Timeout:    10 * time.Second,
}
```

### Exponential backoff

Double the delay after each retry, so a failing service gets more and more room to recover. Add `MaxDelay` to keep waits reasonable and `UseJitter` to spread out clients that failed at the same moment, which avoids the "thundering herd" effect. This is the usual choice for calls to shared services.

```go
// 5 attempts at roughly 0s, 1s, 3s, 7s, 15s (delays 1s, 2s, 4s, 8s)
opts := &retry.Option{
    MaxRetries:     5,
    Delay:          1 * time.Second,
    MaxDelay:       10 * time.Second,
    Timeout:        20 * time.Second,
    UseExponential: true,
    UseJitter:      true,
}
```

Only retry operations that are safe to repeat (idempotent). Retrying a payment or an order creation can perform it twice unless the upstream deduplicates requests.

## Testing

```bash
go test ./...
```

## License

[MIT](LICENSE)
