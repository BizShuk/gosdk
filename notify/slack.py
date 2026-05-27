"""
SlackNotifier delivers statistics summaries to a Slack channel.
Python version of gosdk/notify/slack.go pattern.
"""

import logging
from typing import Optional, Optional
from slack_sdk import WebClient
from slack_sdk.errors import SlackApiError

logger = logging.getLogger(__name__)


class SlackNotifier:
    """Sends notifications to a Slack channel."""

    def __init__(self, token: str, channel_id: str):
        self.client = WebClient(token=token) if token else None
        self.channel_id = channel_id

    def notify(self, summary: str) -> None:
        """Post a message to Slack channel."""
        if not self.client or not self.channel_id:
            logger.warning("Slack notifier not configured: token or channel ID is missing")
            return

        try:
            self.client.chat_postMessage(
                channel=self.channel_id,
                text=summary
            )
        except SlackApiError as e:
            logger.error(f"Slack post message failed: {e}")
            raise


# Optional: Multi notifier pattern
class MultiNotifier:
    """Composite multiple notifiers."""

    def __init__(self, notifiers: list):
        self.notifiers = notifiers

    def notify(self, summary: str) -> None:
        """Route notification to all registered notifiers."""
        errors = []
        for n in self.notifiers:
            try:
                n.notify(summary)
            except Exception as e:
                errors.append(e)

        if errors:
            raise Exception(errors)
