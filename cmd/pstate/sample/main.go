// sample 示範以 pstate 包住一條平行 pipeline:
//
//	go run ./cmd/pstate/sample            # TUI: 失敗的節點點兩下 (或選取後按 r) 以原參數重跑
//	go run ./cmd/pstate/sample -headless  # 不開 TUI, 跑完印出結果
//
// 執行中可觀察: 平行的 build-* 同時跑, e2e-test 第一次固定失敗, approval 停在 pending 等 enter.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"sync/atomic"
	"time"

	"github.com/bizshuk/gosdk/v2/cmd/pstate"
)

// BuildArgs 是 build 步驟的參數; Wrap 時保存, 重試時原封不動再傳一次.
type BuildArgs struct {
	Target string
	Arch   string
}

type TestArgs struct {
	Suite string
	Shard int
}

func main() {
	headless := flag.Bool("headless", false, "run without the TUI")
	wide := flag.Int("wide", 6, "number of parallel build targets (more = wider graph)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	p := pstate.New()
	must(p.Add("checkout", pstate.Do(work(3))))
	must(p.Add("lint", pstate.Do(work(3)), pstate.After("checkout"), pstate.Priority(1)))

	builds := []string{}
	for i := range *wide {
		id := fmt.Sprintf("build-%d", i)
		builds = append(builds, id)
		must(p.Add(id, pstate.Wrap(build, BuildArgs{Target: fmt.Sprintf("svc-%d", i), Arch: "arm64"}), pstate.After("checkout"), pstate.Priority(1)))
	}
	var e2eRuns atomic.Int32
	must(p.Add("unit-test", pstate.Wrap(test(nil), TestArgs{Suite: "unit"}), pstate.After("checkout"), pstate.Priority(1)))
	must(p.Add("e2e-test", pstate.Wrap(test(&e2eRuns), TestArgs{Suite: "e2e", Shard: 2}), pstate.After(builds...), pstate.Priority(2)))
	must(p.Add("image", pstate.Do(work(3)), pstate.After(append(builds, "unit-test")...), pstate.Priority(2)))
	must(p.Add("approval", pstate.Do(approval), pstate.After("e2e-test", "image", "lint"), pstate.Priority(3)))
	must(p.Add("deploy", pstate.Do(work(4)), pstate.After("approval"), pstate.Priority(4)))
	must(p.Add("notify", pstate.Do(work(1)), pstate.After("deploy"), pstate.Priority(5)))
	must(p.Add("docs", pstate.Do(work(2)), pstate.After("checkout"), pstate.Priority(5))) // 與主線平行, 直接連到最後

	var err error
	if *headless {
		err = p.Execute(ctx)
	} else {
		err = pstate.Run(ctx, p, pstate.WithTitle("release"))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "pipeline:", err)
		os.Exit(1)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func work(steps int) func(context.Context, *pstate.T) error {
	return func(ctx context.Context, t *pstate.T) error {
		for i := range steps {
			if err := sleep(ctx, time.Duration(300+rand.IntN(700))*time.Millisecond); err != nil {
				return err
			}
			t.Logf("step %d/%d done", i+1, steps)
		}
		return nil
	}
}

func build(ctx context.Context, t *pstate.T, a BuildArgs) error {
	t.Logf("building %s for %s", a.Target, a.Arch)
	return work(3)(ctx, t)
}

// test 回傳測試函式; runs 不為 nil 時第一次執行固定失敗, 用來示範重試.
func test(runs *atomic.Int32) func(context.Context, *pstate.T, TestArgs) error {
	return func(ctx context.Context, t *pstate.T, a TestArgs) error {
		t.Logf("running %s suite, shard %d", a.Suite, a.Shard)
		if err := sleep(ctx, 1500*time.Millisecond); err != nil {
			return err
		}
		if runs != nil && runs.Add(1) == 1 {
			t.Error("--- FAIL: TestCheckout (1.42s)\n    checkout_test.go:41: expected 200, got 502")
			return errors.New("1 test failed")
		}
		return nil
	}
}

func approval(ctx context.Context, t *pstate.T) error {
	if err := t.Wait(ctx, "waiting for release manager"); err != nil {
		return err
	}
	t.Log("approved")
	return nil
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
