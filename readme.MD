# Retry

A simple Go package that provides retry logic for executing functions with customizable options such as retry count,
delay, timeout, exponential backoff, and jitter.

## Features

- Retry a function multiple times if it fails.
- Customize the number of retry attempts.
- Specify the delay between retries with optional jitter.
- Exponential backoff support for retries.
- Timeout to stop retrying after a certain period.
- Support context cancellation.

## Usage

To use the retry package, you need to call the Do function with the function you want to retry. Here's a simple example:

```go
opts := retry.Option{
    MaxRetries:     4,
    Delay:          1 * time.Second,
    Timeout:        6 * time.Second,
    UseExponential: true,
    UseJitter:      true,
}

err := retry.Do(ctx, func () error {
    // do something ....
    return publishData() // example: publish data that may fail
}, opts)
if err != nil {
    // do something if retry is failed
}

```

## Options

The Option struct allows you to customize the retry behavior:

```go
type Option struct {
    MaxRetries     int           // Maximum number of retry attempts (default: 3)
    Delay          time.Duration // Initial delay between retries (default: 1 second)
    Timeout        time.Duration // Total timeout for retries (default: 5 seconds)
    UseExponential bool          // Enable exponential backoff (default: false)
    UseJitter      bool          // Add random jitter to the delay (default: false)
    OnRetry        func(totalAttempt int, totalDelay time.Duration, err error) // Callback function for custom retry event handling
}
```

- `MaxRetries`: The maximum number of times the function will be retried. Defaults to 3.
- `Delay`: The initial delay between retries. Defaults to 1 * time.Second.
- `Timeout`: The total timeout before stopping retries. Defaults to 5 * time.Second.
- `UseExponential`: If true, the delay will increase exponentially after each retry (e.g., 1s, 2s, 4s, etc.). Defaults
  to false.
- `UseJitter`: If true, random jitter is added to the delay between retries to prevent thundering herd problems.
  Defaults to false.
- `OnRetry`: a function that receives the total attempts, total delay, and error as arguments, allowing for custom retry event handling.

--- 

## Example Use-Cases

Retry package can be used in various scenarios and strategies. You can explore and consider the tradeoff before
implement it in your system. Here are some examples:

### Linear Retry with Max Attempts

In a linear retry strategy, the time between retries increases by a fixed, constant amount.

#### Tradeoffs

- Pros:
    - Simple and predictable.
    - Low Latency: The total time to retry is the sum of all delays.
    - Constant Load: The number of retries per unit time is constant.
- Cons:
    - May Overload the System: when the delays are too short, the system may be overloaded with too many retries.
    - Inefficient in High-Failure Scenarios: In scenarios with high failure rates, linear retry may not be efficient as
      it.

#### Example Flow & Usage

**Use case**: retry 4 times with a delay of 2 seconds between each retry.

**Flow:**

```
Retry 1: After 2 second  
Retry 2: After 4 seconds  
Retry 3: After 6 seconds  
Retry 4: After 8 seconds 
Stop if still failing
```

**Code:**

```go
opts := retry.Option{
    MaxRetries:     4,
    Delay:          2 * time.Second,
}

err := retry.Do(ctx, func () error {
    // Simulate a function that may fail
    return errors.New("temporary error")
}, opts)
if err != nil {
    fmt.Println("Failed:", err)
}
```

### Retry with exponential backoff

In an exponential backoff strategy, the delay between retries increases exponentially, often with a random "jitter" to
prevent retries from occurring simultaneously across many clients.

#### Tradeoffs

- Pros:
    - Efficient in High-Failure Scenarios: Exponential backoff is efficient in scenarios with high failure rates as it.
    - Better Adaptation to Failures: Exponential backoff allows the system to adapt to failures by increasing the delay.
    - Avoids Thundering Herd Problem: Exponential backoff can help avoid the "thundering herd" problem, which occurs
      when many clients retry simultaneously, causing a surge in traffic.
- Cons:
    - Complexity: Exponential backoff can be complicated, especially if you also need to consider jitter, max retry
      limits, and other factors.
    - Slower Recovery: Exponential backoff can lead to slower recovery times, especially if the maximum delay is set too
      high.
    - Potentially High Latency: Depending on the parameters (e.g., the base time for exponential growth), retries might
      take longer to reattempt compared to a linear strategy.

#### Example Flow & Usage

**Use case**: retry 5 times with a delay of 1 second exponentially

**Flow:**
```
Retry 1: After 1 second
Retry 2: After 2 seconds
Retry 3: After 4 seconds
Retry 4: After 8 seconds
Retry 5: After 16 seconds
Stop if still failing
```

**Code:**

```go

opts := retry.Option{
    MaxRetries:     5,
    Delay:          1 * time.Second,
    UseExponential: true,
}

err := retry.Do(ctx, func () error {
    // Simulate a function that may fail
    return errors.New("temporary error")
}, opts)
if err != nil {
    fmt.Println("Failed:", err)
}
```

### How to handle 400 error code?

To handle a 400 error code in a retry mechanism, you should treat it as a non-retryable error. A 400 Bad Request 
indicates that the requested action could not be understood by the server due to invalid syntax or semantics, 
so retrying wouldn't help. This also applies to certain errors depend on error.

```go
opts := retry.Option{
    MaxRetries:     4,
    Delay:          1 * time.Second,
    Timeout:        6 * time.Second,
    UseExponential: true,
    UseJitter:      true,
}

err := retry.Do(ctx, func () error {
	user, err = getData(req)
	if err != nil {
		if err.Error() == "Bad Request" {
			// do not retry on specific status code 
			return nil 
		}
		return err
    }
    return nil
}, opts)
if err != nil {
    // do something if retry is failed
}
```