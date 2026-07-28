package discord

import "github.com/CamiloValderruten/faultline/internal/messaging"

// ChannelGuide returns Discord-specific collaborator instructions injected
// into the system prompt on every context rebuild.
func (b *Bot) ChannelGuide() string {
	return messaging.DiscordChannelGuide
}
