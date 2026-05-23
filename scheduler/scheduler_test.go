package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestScheduler_RunsJobOnInterval(t *testing.T) {
	var mu sync.Mutex
	calls := 0

	s := New()
	s.Add(Job{
		Name:     "tick",
		Interval: 50 * time.Millisecond,
		Fn: func(ctx context.Context) error {
			mu.Lock()
			calls++
			mu.Unlock()
			return nil
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := s.Start(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Start returned %v, want context.DeadlineExceeded", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if calls < 2 {
		t.Errorf("expected at least 2 calls, got %d", calls)
	}
}

func TestScheduler_OnErrorReceivesFailures(t *testing.T) {
	wantErr := errors.New("boom")

	var mu sync.Mutex
	var gotName string
	var gotErr error

	s := New()
	s.Add(Job{
		Name:     "failing",
		Interval: 30 * time.Millisecond,
		Fn:      func(ctx context.Context) error { return wantErr },
		OnError: func(name string, err error) {
			mu.Lock()
			defer mu.Unlock()
			gotName = name
			gotErr = err
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_ = s.Start(ctx)

	mu.Lock()
	defer mu.Unlock()
	if gotName != "failing" {
		t.Errorf("OnError name = %q, want %q", gotName, "failing")
	}
	if !errors.Is(gotErr, wantErr) {
		t.Errorf("OnError err = %v, want %v", gotErr, wantErr)
	}
}
