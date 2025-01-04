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
