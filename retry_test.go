package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestDo(t *testing.T) {
	var testAttempts int
	ctxWithCancel, cancel := context.WithCancel(context.Background())

	type args struct {
		ctx  context.Context
		f    func() error
		opts *Option
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success with default options",
			args: args{
				ctx: context.Background(),
				f: func() error {
					return nil
				},
				opts: nil,
			},
			wantErr: false,
		},
		{
			name: "fails canceled by context cancellation",
			args: args{
				ctx: ctxWithCancel,
				f: func() error {
					testAttempts++
					if testAttempts == 1 {
						// simulate cancel on 2nd attempt
						// its just sample: in real usage, cancellation can happen any time
						cancel()
					}
					return errors.New("test-error")
				},
				opts: &Option{
					MaxRetries: 5,
					Delay:      1 * time.Second,
					OnRetry: func(totalAttempt int, totalDelay time.Duration, err error) {
						fmt.Printf("attempt: %d, total delay: %fs, err: %v\n", totalAttempt, totalDelay.Seconds(), err)
					},
				},
			},
			wantErr: true,
		},
		{
			name: "linear attempt - success on first attempt",
			args: args{
				ctx: context.Background(),
				f: func() error {
					return nil
				},
				opts: &Option{
					MaxRetries: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "linear attempt - success on last attempt",
			args: args{
				ctx: context.Background(),
				f: func() error {
					testAttempts++
					if testAttempts < 2 {
						return errors.New("test-error")
					}
					return nil
				},
				opts: &Option{
					MaxRetries: 2,
				},
			},
			wantErr: false,
		},
		{
			name: "linear attempt - fails on all attempts",
			args: args{
				ctx: context.Background(),
				f: func() error {
					return errors.New("test-error")
				},
				opts: &Option{
					MaxRetries: 4,
					Delay:      1 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "linear timeout - fails on timeout",
			args: args{
				ctx: context.Background(),
				f: func() error {
					return errors.New("test error")
				},
				opts: &Option{
					Timeout: 1 * time.Second,
					Delay:   1 * time.Second,
				},
			},
			wantErr: true,
		},
		{
			name: "exponential backoff - fails on last attempt",
			args: args{
				ctx: context.Background(),
				f: func() error {
					testAttempts++
					if testAttempts < 2 {
						return errors.New("test-error")
					}
					return nil
				},
				opts: &Option{
					MaxRetries:     2,
					Delay:          1 * time.Second,
					UseExponential: true,
				},
			},
			wantErr: false,
		},
		{
			name: "exponential backoff - fails on timeout",
			args: args{
				ctx: context.Background(),
				f: func() error {
					return errors.New("test error")
				},
				opts: &Option{
					MaxRetries:     10,
					Timeout:        3 * time.Second,
					Delay:          1 * time.Second,
					UseExponential: true,
				},
			},
			wantErr: true,
		},
		{
			name: "exponential backoff with jitter - success on last attempt",
			args: args{
				ctx: context.Background(),
				f: func() error {
					testAttempts++
					if testAttempts < 2 {
						return errors.New("test-error")
					}
					return nil
				},
				opts: &Option{
					MaxRetries:     2,
					Delay:          1 * time.Second,
					UseExponential: true,
					UseJitter:      true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testAttempts = 0 // reset testAttempts for each test case
			if err := Do(tt.args.ctx, tt.args.f, tt.args.opts); (err != nil) != tt.wantErr {
				t.Errorf("Do() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

var errTemporary = errors.New("temporary error")

func TestDo_WrapsLastError(t *testing.T) {
	attempts := 0
	err := Do(context.Background(), func() error {
		attempts++
		return errTemporary
	}, &Option{MaxRetries: 3, Delay: time.Millisecond})

	if !errors.Is(err, errTemporary) {
		t.Fatalf("expected error to wrap the last error, got %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_Unrecoverable(t *testing.T) {
	errBadRequest := errors.New("bad request")
	attempts := 0
	err := Do(context.Background(), func() error {
		attempts++
		return Unrecoverable(errBadRequest)
	}, &Option{MaxRetries: 5, Delay: time.Millisecond})

	if err != errBadRequest {
		t.Fatalf("expected the original error, got %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected no retries, got %d attempts", attempts)
	}
	if Unrecoverable(nil) != nil {
		t.Fatal("Unrecoverable(nil) should be nil")
	}
}

func TestDo_CancelDuringDelay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := Do(ctx, func() error { return errTemporary }, &Option{MaxRetries: 5, Delay: 10 * time.Second, Timeout: time.Minute})

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected Do to stop waiting when ctx is done, took %s", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, errTemporary) {
		t.Fatalf("expected both ctx error and last error, got %v", err)
	}
}

func TestDo_ExponentialWithMaxDelay(t *testing.T) {
	var delays []time.Duration
	var last time.Duration
	_ = Do(context.Background(), func() error { return errTemporary }, &Option{
		MaxRetries:     5,
		Delay:          10 * time.Millisecond,
		MaxDelay:       25 * time.Millisecond,
		Timeout:        time.Second,
		UseExponential: true,
		OnRetry: func(_ int, totalDelay time.Duration, _ error) {
			delays = append(delays, totalDelay-last)
			last = totalDelay
		},
	})

	want := []time.Duration{0, 10, 20, 25, 25}
	for i, d := range delays {
		if d != want[i]*time.Millisecond {
			t.Fatalf("delay %d: want %s, got %s (all: %v)", i, want[i]*time.Millisecond, d, delays)
		}
	}
}

func TestDo_DoesNotModifyOptions(t *testing.T) {
	opts := &Option{}
	_ = Do(context.Background(), func() error { return nil }, opts)
	if opts.MaxRetries != 0 || opts.Delay != 0 || opts.Timeout != 0 {
		t.Fatalf("Do should not modify the caller's options, got %+v", *opts)
	}
}
