package notify

import "context"

// Notifier defines how a stats summary is delivered.
type Notifier interface {
	Notify(ctx context.Context, summary string) error
}
