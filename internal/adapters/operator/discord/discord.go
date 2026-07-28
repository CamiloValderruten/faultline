// Package discord is the Discord-backed operator adapter for
// bidirectional collaborator communication.
package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	"github.com/bwmarrin/discordgo"
)

// Bot is a Discord bot for bidirectional collaborator communication.
type Bot struct {
	session   *discordgo.Session
	channelID string
	logger    *slog.Logger

	media  InboundMedia
	speech Speech

	mu      sync.Mutex
	pending []string

	// Live voice channel (optional).
	voiceMu           sync.Mutex
	voiceChannelID    string
	operatorUserID    string
	voiceGuildID      string
	vc                *discordgo.VoiceConnection
	voicePlaying      bool
	voiceJoining      bool
	voiceHandlerAdded bool
}

// New creates a Discord session (not yet connected). Call Start to open
// the gateway and begin receiving events.
func New(token, channelID string, logger *slog.Logger) (*Bot, error) {
	token = strings.TrimSpace(token)
	channelID = strings.TrimSpace(channelID)
	if token == "" {
		return nil, fmt.Errorf("discord token is required")
	}
	if channelID == "" {
		return nil, fmt.Errorf("discord channel_id is required")
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("create discord session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentGuilds |
		discordgo.IntentGuildMessages |
		discordgo.IntentDirectMessages |
		discordgo.IntentMessageContent

	b := &Bot{
		session:   session,
		channelID: channelID,
		logger:    logger,
	}
	session.AddHandler(b.onMessageCreate)
	session.AddHandler(b.onInteractionCreate)
	return b, nil
}

// Start opens the Discord gateway and blocks until ctx is canceled.
func (b *Bot) Start(ctx context.Context) {
	if err := b.session.Open(); err != nil {
		b.logger.Error("discord gateway open failed", "error", err)
		return
	}
	user := ""
	if b.session.State != nil && b.session.State.User != nil {
		user = b.session.State.User.Username
	}
	b.logger.Info("discord listener started", "user", user, "channel_id", b.channelID)

	<-ctx.Done()
	b.leaveVoice()
	if err := b.session.Close(); err != nil {
		b.logger.Debug("discord session close", "error", err)
	}
	b.logger.Info("discord listener stopped")
}

func (b *Bot) onMessageCreate(_ *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil || m.Message == nil {
		return
	}
	if m.Author != nil && m.Author.Bot {
		return
	}
	if m.ChannelID != b.channelID {
		return
	}

	text, ok := b.inboundText(m.Message)
	if !ok {
		return
	}
	b.logger.Info("received message from collaborator", "text", text)
	b.enqueue(text)
}

func (b *Bot) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Interaction == nil {
		return
	}
	if i.Type != discordgo.InteractionMessageComponent {
		return
	}
	data := i.MessageComponentData()
	channelID := ""
	if i.ChannelID != "" {
		channelID = i.ChannelID
	} else if i.Message != nil {
		channelID = i.Message.ChannelID
	}
	if channelID != b.channelID {
		b.logger.Warn("ignoring interaction from unknown channel", "channel_id", channelID)
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})
		return
	}

	pending := formatComponentPending(data)
	b.logger.Info("received component interaction from collaborator", "text", pending)
	b.enqueue(pending)

	// Acknowledge by disabling all interactive components on the message so
	// the collaborator sees the click landed and can't double-press.
	resp := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}
	if i.Message != nil {
		if disabled := disableMessageComponents(i.Message.Components); len(disabled) > 0 {
			resp = &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content:    i.Message.Content,
					Components: disabled,
					Embeds:     i.Message.Embeds,
				},
			}
		}
	}
	if err := s.InteractionRespond(i.Interaction, resp); err != nil {
		b.logger.Debug("interaction respond failed", "error", err)
	}
}

func formatComponentPending(data discordgo.MessageComponentInteractionData) string {
	id := strings.TrimSpace(data.CustomID)
	switch data.ComponentType {
	case discordgo.SelectMenuComponent, discordgo.ChannelSelectMenuComponent,
		discordgo.UserSelectMenuComponent, discordgo.RoleSelectMenuComponent,
		discordgo.MentionableSelectMenuComponent:
		vals := data.Values
		if len(vals) == 0 {
			return fmt.Sprintf("Selected menu %q (no values)", id)
		}
		return fmt.Sprintf("Selected menu %q (values=%s)", id, strings.Join(vals, ","))
	default:
		if id == "" {
			return "Pressed a button"
		}
		return fmt.Sprintf("Pressed button %q (data=%s)", id, id)
	}
}

func (b *Bot) inboundText(msg *discordgo.Message) (string, bool) {
	if msg == nil {
		return "", false
	}
	text := strings.TrimSpace(msg.Content)

	if voiceAtt := pickVoiceAttachment(msg); voiceAtt != nil {
		return b.inboundVoice(voiceAtt)
	}

	if len(msg.Attachments) > 0 {
		return b.inboundAttachments(msg.Attachments, text)
	}
	if text == "" {
		return "", false
	}
	return text, true
}

func (b *Bot) enqueue(text string) {
	b.mu.Lock()
	b.pending = append(b.pending, text)
	b.mu.Unlock()
}

// Pending drains and returns all queued incoming messages.
func (b *Bot) Pending() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.pending) == 0 {
		return nil
	}
	msgs := b.pending
	b.pending = nil
	return msgs
}

// HasPending reports whether any incoming messages are queued.
func (b *Bot) HasPending() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.pending) > 0
}

// Send sends a plain text message (Discord markdown supported).
func (b *Bot) Send(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("text is required")
	}
	_, err := b.session.ChannelMessageSend(b.channelID, text)
	if err != nil {
		return fmt.Errorf("send discord message: %w", err)
	}
	return nil
}

// SendWithButtons sends text with button/select action rows.
func (b *Bot) SendWithButtons(text string, buttons [][]messaging.Button) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("text is required when sending buttons")
	}
	components, err := buildComponents(buttons, nil)
	if err != nil {
		return err
	}
	_, err = b.session.ChannelMessageSendComplex(b.channelID, &discordgo.MessageSend{
		Content:    text,
		Components: components,
	})
	if err != nil {
		return fmt.Errorf("send discord message with buttons: %w", err)
	}
	return nil
}

// SendRich sends an embed (title/content/fields/color) plus optional components.
func (b *Bot) SendRich(msg messaging.RichMessage) error {
	components, err := buildComponents(msg.Buttons, msg.Selects)
	if err != nil {
		return err
	}

	embed := &discordgo.MessageEmbed{}
	if title := strings.TrimSpace(msg.Title); title != "" {
		embed.Title = title
	}
	if content := strings.TrimSpace(msg.Content); content != "" {
		embed.Description = content
	}
	if msg.Color != 0 {
		embed.Color = msg.Color
	}
	for _, f := range msg.Fields {
		name := strings.TrimSpace(f.Name)
		value := strings.TrimSpace(f.Value)
		if name == "" && value == "" {
			continue
		}
		if name == "" {
			name = "\u200b"
		}
		if value == "" {
			value = "\u200b"
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:   name,
			Value:  value,
			Inline: f.Inline,
		})
	}

	hasEmbed := embed.Title != "" || embed.Description != "" || len(embed.Fields) > 0
	if !hasEmbed && len(components) == 0 {
		return fmt.Errorf("rich content is required")
	}

	send := &discordgo.MessageSend{Components: components}
	if hasEmbed {
		send.Embeds = []*discordgo.MessageEmbed{embed}
	} else {
		send.Content = "(choose an option)"
	}

	_, err = b.session.ChannelMessageSendComplex(b.channelID, send)
	if err != nil {
		return fmt.Errorf("send discord rich message: %w", err)
	}
	return nil
}
