package http

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fastPolicy keeps the loop honest about attempt counts without spending
// real backoff time in tests.
func fastPolicy(attempts int) RetryPolicy {
	return ConstantRetryPolicy(attempts, 0)
}

func TestRetryReturnsFirstSuccess(t *testing.T) {
	calls := 0
	got, err := Retry(context.Background(), fastPolicy(5), func(context.Context) (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil || got != "ok" {
		t.Fatalf("got (%q, %v), want (\"ok\", nil)", got, err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestRetryStopsOnPermanentError(t *testing.T) {
	permanent := errors.New("http 404")
	calls := 0
	_, err := Retry(context.Background(), fastPolicy(5), func(context.Context) (int, error) {
		calls++
		return 0, permanent
	})
	if !errors.Is(err, permanent) {
		t.Fatalf("err = %v, want %v", err, permanent)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (permanent errors must not retry)", calls)
	}
}

func TestRetryRetriesTaggedErrorUpToBudget(t *testing.T) {
	transient := errors.New("http 503")
	calls := 0
	_, err := Retry(context.Background(), fastPolicy(5), func(context.Context) (int, error) {
		calls++
		return 0, Retryable(transient)
	})
	if !errors.Is(err, transient) {
		t.Fatalf("err = %v, want it to wrap %v", err, transient)
	}
	if calls != 5 {
		t.Fatalf("calls = %d, want 5", calls)
	}
}

func TestRetrySucceedsAfterTransientFailures(t *testing.T) {
	calls := 0
	got, err := Retry(context.Background(), fastPolicy(5), func(context.Context) (int, error) {
		calls++
		if calls < 3 {
			return 0, Retryable(errors.New("flaky"))
		}
		return 42, nil
	})
	if err != nil || got != 42 {
		t.Fatalf("got (%d, %v), want (42, nil)", got, err)
	}
	if calls != 3 {
		t.Fatalf("calls = %d, want 3", calls)
	}
}

func TestRetryReturnsLastValueOnFailure(t *testing.T) {
	got, err := Retry(context.Background(), fastPolicy(3), func(context.Context) (string, error) {
		return "last-body", Retryable(errors.New("boom"))
	})
	if err == nil {
		t.Fatal("err = nil, want an error")
	}
	if got != "last-body" {
		t.Fatalf("got %q, want the final attempt's value to survive", got)
	}
}

func TestRetryAbandonsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	calls := 0
	_, err := Retry(ctx, ConstantRetryPolicy(5, time.Second), func(context.Context) (int, error) {
		calls++
		return 0, Retryable(errors.New("boom"))
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (cancellation outranks the attempt error)", calls)
	}
}

func TestIsRetryableSurvivesFurtherWrapping(t *testing.T) {
	inner := errors.New("root cause")
	wrapped := errors.Join(Retryable(inner), errors.New("context"))
	if !IsRetryable(wrapped) {
		t.Fatal("IsRetryable = false, want the tag to survive re-wrapping")
	}
	if !errors.Is(wrapped, inner) {
		t.Fatal("errors.Is lost the original cause")
	}
}

func TestRetryTreatsZeroAttemptsAsOne(t *testing.T) {
	calls := 0
	_, _ = Retry(context.Background(), RetryPolicy{}, func(context.Context) (int, error) {
		calls++
		return 0, Retryable(errors.New("boom"))
	})
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestDefaultRetryPolicyMatchesProjectConvention(t *testing.T) {
	p := DefaultRetryPolicy()
	if p.MaxAttempts != DEFAULT_MAX_ATTEMPTS || p.BaseDelay != DEFAULT_BASE_DELAY || p.MaxDelay != DEFAULT_MAX_DELAY {
		t.Fatalf("DefaultRetryPolicy() = %+v, want the shared defaults", p)
	}
}

func TestIsRetryableStatus(t *testing.T) {
	retryable := []int{429, 500, 502, 503, 504}
	for _, status := range retryable {
		if !IsRetryableStatus(status) {
			t.Errorf("IsRetryableStatus(%d) = false, want true", status)
		}
	}

	permanent := []int{200, 201, 301, 400, 401, 403, 404, 422}
	for _, status := range permanent {
		if IsRetryableStatus(status) {
			t.Errorf("IsRetryableStatus(%d) = true, want false", status)
		}
	}
}
