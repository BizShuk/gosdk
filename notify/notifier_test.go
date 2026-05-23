package notify

import (
	"context"
	"errors"
	"testing"
)

type mockNotifier struct {
	called bool
	err    error
}

func (m *mockNotifier) Notify(ctx context.Context, summary string) error {
	m.called = true
	return m.err
}

func TestMulti_Notify(t *testing.T) {
	n1 := &mockNotifier{}
	n2 := &mockNotifier{}

	multi := NewMulti(n1, n2)
	err := multi.Notify(context.Background(), "test message")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !n1.called || !n2.called {
		t.Error("expected both notifiers to be called")
	}
}

func TestMulti_Notify_WithError(t *testing.T) {
	n1 := &mockNotifier{err: errors.New("fail")}
	n2 := &mockNotifier{}

	multi := NewMulti(n1, n2)
	err := multi.Notify(context.Background(), "test message")
	if err == nil {
		t.Error("expected error, got nil")
	}

	if !n1.called || !n2.called {
		t.Error("expected both notifiers to be called even if one fails")
	}
}
