// Package http holds the client-side HTTP helpers that every gosdk consumer
// otherwise rewrites: the retry loop, its backoff policy, and the
// transient/permanent classification of a response.
//
// The package name shadows net/http, so callers that need both import this
// one under an alias (conventionally gohttp).
package http

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Retry budget shared by every HTTP-facing caller. Stated once here so
// "max retry times is 5" does not get re-decided per package.
const (
	// DEFAULT_MAX_ATTEMPTS counts the initial try plus its retries.
	DEFAULT_MAX_ATTEMPTS = 5
	// DEFAULT_BASE_DELAY is the first backoff step; each further attempt
	// doubles it (200ms, 400ms, 800ms, 1.6s).
	DEFAULT_BASE_DELAY = 200 * time.Millisecond
	// DEFAULT_MAX_DELAY caps the exponential growth so a long budget can
	// never turn into a multi-minute stall.
	DEFAULT_MAX_DELAY = 5 * time.Second
)

// RetryPolicy bounds a Retry loop. A zero BaseDelay retries immediately;
// a zero MaxDelay leaves the exponential growth uncapped. Setting MaxDelay
// equal to BaseDelay turns the backoff into a constant delay.
type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
}

// DefaultRetryPolicy returns the shared 5-attempt exponential-backoff policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: DEFAULT_MAX_ATTEMPTS,
		BaseDelay:   DEFAULT_BASE_DELAY,
		MaxDelay:    DEFAULT_MAX_DELAY,
	}
}

// ConstantRetryPolicy returns a policy that waits the same delay between
// every attempt, for callers that do not want backoff growth.
func ConstantRetryPolicy(attempts int, delay time.Duration) RetryPolicy {
	return RetryPolicy{MaxAttempts: attempts, BaseDelay: delay, MaxDelay: delay}
}

// retryableError tags an error as worth another attempt. A typed wrapper
// (rather than a sentinel) is used so the classification survives being
// wrapped again with %w at the call site.
type retryableError struct {
	err error
}

func (e *retryableError) Error() string { return e.err.Error() }
func (e *retryableError) Unwrap() error { return e.err }

// Retryable marks err as transient so Retry will try again. Returns nil
// unchanged so callers can wrap unconditionally.
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return &retryableError{err: err}
}

// IsRetryable reports whether err, or anything it wraps, was marked by
// Retryable.
func IsRetryable(err error) bool {
	var target *retryableError
	return errors.As(err, &target)
}

// IsRetryableStatus reports whether an HTTP status code is worth another
// attempt: 429 (rate limited) and any 5xx. Every other 4xx is the caller's
// own fault and will not fix itself.
func IsRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

// Retry calls do until it succeeds, returns an error not marked Retryable,
// exhausts policy.MaxAttempts, or ctx is cancelled.
//
// The value from the final attempt is returned even when the error is
// non-nil, so callers that need the last response (a status code, a partial
// body) can still inspect it. Between attempts Retry sleeps
// BaseDelay << (attempt-1), clamped to MaxDelay; the sleep is abandoned as
// soon as ctx is done.
func Retry[T any](ctx context.Context, policy RetryPolicy, do func(context.Context) (T, error)) (T, error) {
	attempts := policy.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var last T
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		value, err := do(ctx)
		last = value
		if err == nil {
			return value, nil
		}
		lastErr = err

		// A cancelled context outranks the error the attempt reported —
		// the failure is our own doing, not the endpoint's.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return last, ctxErr
		}
		if !IsRetryable(err) {
			return last, err
		}
		if attempt == attempts {
			break
		}
		if waitErr := waitBackoff(ctx, policy, attempt); waitErr != nil {
			return last, waitErr
		}
	}
	return last, lastErr
}

// waitBackoff sleeps the backoff for the attempt that just failed, or
// returns ctx.Err() if the context is cancelled first.
func waitBackoff(ctx context.Context, policy RetryPolicy, attempt int) error {
	delay := policy.BaseDelay << uint(attempt-1)
	if policy.MaxDelay > 0 && delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
