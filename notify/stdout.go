package notify

import (
	"context"
	"fmt"
)

// StdoutNotifier writes the summary to stdout.
type StdoutNotifier struct{}

// Notify implements Notifier by writing to stdout.
func (s *StdoutNotifier) Notify(_ context.Context, summary string) error {
	fmt.Println(summary)
	return nil
}
