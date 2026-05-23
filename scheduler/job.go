// Package scheduler is a minimal, dependency-free periodic job runner.
package scheduler

import (
	"context"
	"time"
)

// Job is a scheduled unit of work owned by the application. The
// scheduler treats Job as immutable data; once handed to Add it is
// copied and never mutated.
type Job struct {
	// Name identifies the job. It is passed back to OnError so a
	// single callback can disambiguate multiple sources.
	Name string

	// Interval is the period between consecutive Fn invocations.
	// The first invocation fires after one Interval, not immediately.
	Interval time.Duration

	// Fn is the work to perform on each tick. It must honour ctx
	// cancellation; the scheduler will not interrupt it directly.
	Fn func(ctx context.Context) error

	// OnError, if non-nil, is invoked whenever Fn returns a non-nil
	// error. The scheduler itself takes no opinion on logging or
	// recovery — that policy belongs to the application.
	OnError func(name string, err error)
}
