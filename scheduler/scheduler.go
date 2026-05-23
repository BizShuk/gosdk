// Package scheduler is a minimal, dependency-free periodic job runner.
//
// The package is intentionally narrow: it schedules user-supplied work
// on fixed intervals and nothing more. All domain concerns (logging,
// metrics, side effects, shutdown semantics) belong to the caller and
// are expressed through the Job value.
package scheduler

import (
	"context"
	"sync"
	"time"
)


// Scheduler runs registered Jobs concurrently, each driven by its own
// ticker. The zero value is not usable; obtain one via New.
type Scheduler struct {
	mu   sync.Mutex
	jobs []Job
}

// New returns an empty Scheduler ready to accept Jobs.
func New() *Scheduler {
	return &Scheduler{}
}

// Add registers a Job and returns the receiver to support chaining.
// Add is safe for concurrent use, but Jobs added after Start has been
// entered are ignored.
func (s *Scheduler) Add(j Job) *Scheduler {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, j)
	return s
}

// Start launches every registered Job on its own goroutine and blocks
// until ctx is cancelled. The returned error is ctx.Err(), so callers
// that treat cancellation as the normal exit path can safely ignore it.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	jobs := make([]Job, len(s.jobs))
	copy(jobs, s.jobs)
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j Job) {
			defer wg.Done()
			ticker := time.NewTicker(j.Interval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if err := j.Fn(ctx); err != nil && j.OnError != nil {
						j.OnError(j.Name, err)
					}
				case <-ctx.Done():
					return
				}
			}
		}(j)
	}

	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}
