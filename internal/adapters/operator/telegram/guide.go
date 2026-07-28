package telegram

import "github.com/CamiloValderruten/faultline/internal/messaging"

// ChannelGuide returns Telegram-specific collaborator instructions injected
// into the system prompt on every context rebuild.
func (t *Bot) ChannelGuide() string {
	return messaging.TelegramChannelGuide
}
