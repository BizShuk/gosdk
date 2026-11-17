package pstate

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func ok() Func { return Do(func(context.Context, *T) error { return nil }) }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func status(t *testing.T, p *Pipeline, id string) node {
	t.Helper()
	for _, n := range p.snapshot().Nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("node %s not found", id)
	return node{}
}

// eventually 等待條件成立, 用於背景 scheduler 的測試.
func eventually(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in 2s")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAddValidates(t *testing.T) {
	p := New()
	if err := p.Add("", ok()); !errors.Is(err, ErrEmptyID) {
		t.Fatalf("empty id: %v", err)
	}
	if err := p.Add("a", Func{}); !errors.Is(err, ErrNilFunc) {
		t.Fatalf("nil func: %v", err)
	}
	must(t, p.Add("a", ok()))
	if err := p.Add("a", ok()); !errors.Is(err, ErrDuplicateNode) {
		t.Fatalf("duplicate: %v", err)
	}
	if err := p.Add("b", ok(), After("later")); !errors.Is(err, ErrUnknownNode) {
		t.Fatalf("deps must be registered first: %v", err)
	}
}

func TestExecuteRunsIndependentNodesInParallelAfterDeps(t *testing.T) {
	p := New()
	var mu sync.Mutex
	var order []string
	record := func(id string) { mu.Lock(); order = append(order, id); mu.Unlock() }

	var inFlight, peak atomic.Int32
	parallel := Do(func(ctx context.Context, t *T) error {
		n := inFlight.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		inFlight.Add(-1)
		record(t.ID())
		return nil
	})
	must(t, p.Add("root", Do(func(_ context.Context, t *T) error { record(t.ID()); return nil })))
	must(t, p.Add("a", parallel, After("root")))
	must(t, p.Add("b", parallel, After("root")))
	must(t, p.Add("c", parallel, After("root")))
	must(t, p.Add("join", Do(func(_ context.Context, t *T) error { record(t.ID()); return nil }), After("a", "b", "c")))

	must(t, p.Execute(context.Background()))
	if order[0] != "root" || order[4] != "join" {
		t.Fatalf("order %v", order)
	}
	if peak.Load() != 3 {
		t.Fatalf("a, b, c should run concurrently; peak %d", peak.Load())
	}
}

func TestFailureBlocksDownstreamAndRetryReusesParams(t *testing.T) {
	type args struct{ Target string }
	var seen []string
	var calls atomic.Int32
	flaky := Wrap(func(_ context.Context, _ *T, a args) error {
		seen = append(seen, a.Target)
		if calls.Add(1) == 1 {
			return errors.New("exit 2\nstack trace")
		}
		return nil
	}, args{Target: "linux"})

	p := New()
	must(t, p.Add("build", flaky))
	must(t, p.Add("deploy", ok(), After("build")))

	err := p.Execute(context.Background())
	if err == nil || !strings.Contains(err.Error(), "build: exit 2") {
		t.Fatalf("Execute error: %v", err)
	}
	if n := status(t, p, "build"); n.Status != Failed || n.Message != "exit 2" {
		t.Fatalf("build: %+v", n)
	}
	if status(t, p, "deploy").Status != NotStarted {
		t.Fatal("deploy must not run after build failed")
	}

	if err := p.Retry("deploy"); !errors.Is(err, ErrNotFailed) {
		t.Fatalf("retry of a node that did not fail: %v", err)
	}
	must(t, p.Retry("build"))
	must(t, p.Execute(context.Background()))
	if n := status(t, p, "build"); n.Status != Finished || n.Attempts != 2 {
		t.Fatalf("build after retry: %+v", n)
	}
	if status(t, p, "deploy").Status != Finished {
		t.Fatal("deploy should run once build succeeds")
	}
	if strings.Join(seen, ",") != "linux,linux" {
		t.Fatalf("retry must reuse the wrapped params: %v", seen)
	}
}

func TestWaitBlocksUntilTrigger(t *testing.T) {
	p := New()
	must(t, p.Add("approval", Do(func(ctx context.Context, t *T) error { return t.Wait(ctx, "need ok") })))
	done := make(chan error, 1)
	go func() { done <- p.Execute(context.Background()) }()

	eventually(t, func() bool { return status(t, p, "approval").Status == Pending })
	if n := status(t, p, "approval"); n.Message != "need ok" {
		t.Fatalf("pending reason %q", n.Message)
	}
	if err := p.Retry("approval"); !errors.Is(err, ErrNotFailed) {
		t.Fatalf("retry on pending: %v", err)
	}
	must(t, p.Trigger("approval"))
	must(t, <-done)
	if status(t, p, "approval").Status != Finished {
		t.Fatal("approval should finish after trigger")
	}
}

func TestPanicBecomesFailure(t *testing.T) {
	p := New()
	must(t, p.Add("boom", Do(func(context.Context, *T) error { panic("nil map") })))
	if err := p.Execute(context.Background()); err == nil || !strings.Contains(err.Error(), "panic: nil map") {
		t.Fatalf("panic should surface as error: %v", err)
	}
	logs := p.snapshot().Logs
	if last := logs[len(logs)-1]; last.Level != LevelError || !strings.Contains(last.Text, "goroutine") {
		t.Fatalf("log should carry the stack: %+v", last)
	}
}

func TestTaskLogsGoToLogPane(t *testing.T) {
	p := New()
	must(t, p.Add("a", Do(func(_ context.Context, t *T) error {
		t.Logf("compiled %d files", 3)
		t.Error("warning: deprecated")
		return nil
	})))
	must(t, p.Execute(context.Background()))
	var texts []string
	for _, e := range p.snapshot().Logs {
		texts = append(texts, e.Text)
	}
	got := strings.Join(texts, "|")
	if !strings.HasPrefix(got, "running|compiled 3 files|warning: deprecated|finished in") {
		t.Fatalf("logs %q", got)
	}
}

func TestLogRingDropsOldest(t *testing.T) {
	r := newLogRing(3)
	for _, s := range []string{"1", "2", "3", "4", "5"} {
		r.push(LogEntry{Text: s})
	}
	got := r.list()
	if len(got) != 3 || got[0].Text != "3" || got[2].Text != "5" {
		t.Fatalf("ring: %+v", got)
	}
}
