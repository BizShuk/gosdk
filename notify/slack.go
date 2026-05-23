package notify

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
)

// SlackNotifier delivers statistics summaries to a Slack channel.
type SlackNotifier struct {
	client    *slack.Client
	channelID string
}

// NewSlackNotifier creates a new SlackNotifier.
func NewSlackNotifier(token, channelID string) *SlackNotifier {
	return &SlackNotifier{
		client:    slack.New(token),
		channelID: channelID,
	}
}

// Notify implements Notifier by posting to Slack using context.
func (s *SlackNotifier) Notify(ctx context.Context, summary string) error {
	_, _, err := s.client.PostMessageContext(ctx, s.channelID, slack.MsgOptionText(summary, false))
	if err != nil {
		return fmt.Errorf("slack post message: %w", err)
	}
	return nil
}