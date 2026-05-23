package notify

import (
	"context"
	"errors"
)

// Multi composites multiple notifiers.
type Multi struct{ notifiers []Notifier }

// NewMulti creates a new Multi.
func NewMulti(notifiers ...Notifier) *Multi {
	return &Multi{notifiers: notifiers}
}

// Notify routes notifications to all registered notifiers.
func (m *Multi) Notify(ctx context.Context, summary string) error {
	var errs []error
	for _, n := range m.notifiers {
		if err := n.Notify(ctx, summary); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}