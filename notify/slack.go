package notify

import (
	"context"
	"fmt"

	"github.com/bizshuk/gosdk/log"
	"github.com/slack-go/slack"
)

// SlackNotifier delivers statistics summaries to a Slack channel.
type SlackNotifier struct {
	client    *slack.Client
	channelID string
}

// NewSlackNotifier creates a new SlackNotifier.
func NewSlackNotifier(token, channelID string) *SlackNotifier {
	var client *slack.Client
	if token != "" {
		client = slack.New(token)
	}
	return &SlackNotifier{
		client:    client,
		channelID: channelID,
	}
}

// Notify implements Notifier by posting to Slack using context.
func (s *SlackNotifier) Notify(ctx context.Context, summary string) error {
	if s.client == nil || s.channelID == "" {
		log.Warn("Slack notifier is not configured: token or channel ID is missing")
		return nil
	}
	_, _, err := s.client.PostMessageContext(ctx, s.channelID, slack.MsgOptionText(summary, false))
	if err != nil {
		return fmt.Errorf("slack post message: %w", err)
	}
	return nil
}
