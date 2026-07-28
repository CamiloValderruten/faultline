// Package discord is the Discord-backed operator adapter for
// bidirectional collaborator communication.
package discord

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	"github.com/bwmarrin/discordgo"
)

const maxOutboundFileBytes = 25 << 20 // Discord boost-dependent; hard cap 25 MiB

// Bot is a Discord bot for bidirectional collaborator communication.
type Bot struct {
	session   *discordgo.Session
	channelID string
	logger    *slog.Logger

	media  InboundMedia
	speech Speech

	mu      sync.Mutex
	pending []string

	// buttonData → modal declared on send; opened within the 3s interaction window.
	modalsMu sync.Mutex
	modals   map[string]messaging.ModalSpec
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
		modals:    map[string]messaging.ModalSpec{},
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

	channelID := interactionChannelID(i)
	if channelID != b.channelID {
		b.logger.Warn("ignoring interaction from unknown channel", "channel_id", channelID, "type", i.Type)
		if i.Type == discordgo.InteractionMessageComponent || i.Type == discordgo.InteractionModalSubmit {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredMessageUpdate,
			})
		}
		return
	}

	switch i.Type {
	case discordgo.InteractionMessageComponent:
		b.handleComponentInteraction(s, i)
	case discordgo.InteractionModalSubmit:
		b.handleModalSubmit(s, i)
	}
}

func interactionChannelID(i *discordgo.InteractionCreate) string {
	if i.ChannelID != "" {
		return i.ChannelID
	}
	if i.Message != nil {
		return i.Message.ChannelID
	}
	return ""
}

func (b *Bot) handleComponentInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()

	if data.ComponentType == discordgo.ButtonComponent {
		if spec, ok := b.lookupModal(strings.TrimSpace(data.CustomID)); ok {
			resp, err := modalResponse(spec)
			if err != nil {
				b.logger.Error("build modal response failed", "error", err)
				_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseDeferredMessageUpdate,
				})
				return
			}
			if err := s.InteractionRespond(i.Interaction, resp); err != nil {
				b.logger.Debug("modal respond failed", "error", err)
			}
			return
		}
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

func (b *Bot) handleModalSubmit(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	pending := formatModalPending(data)
	b.logger.Info("received modal submit from collaborator", "text", pending)
	b.enqueue(pending)

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "Got it — thanks.",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		b.logger.Debug("modal submit respond failed", "error", err)
	}
}

func (b *Bot) lookupModal(buttonData string) (messaging.ModalSpec, bool) {
	b.modalsMu.Lock()
	defer b.modalsMu.Unlock()
	spec, ok := b.modals[buttonData]
	return spec, ok
}

func (b *Bot) registerModals(modals map[string]messaging.ModalSpec) {
	if len(modals) == 0 {
		return
	}
	b.modalsMu.Lock()
	defer b.modalsMu.Unlock()
	for k, v := range modals {
		b.modals[k] = v
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
		labels := resolveSelectLabels(data)
		if labels != "" {
			return fmt.Sprintf("Selected menu %q (values=%s; %s)", id, strings.Join(vals, ","), labels)
		}
		return fmt.Sprintf("Selected menu %q (values=%s)", id, strings.Join(vals, ","))
	default:
		if id == "" {
			return "Pressed a button"
		}
		return fmt.Sprintf("Pressed button %q (data=%s)", id, id)
	}
}

func resolveSelectLabels(data discordgo.MessageComponentInteractionData) string {
	var parts []string
	for _, id := range data.Values {
		if u, ok := data.Resolved.Users[id]; ok && u != nil {
			parts = append(parts, fmt.Sprintf("%s=@%s", id, u.Username))
			continue
		}
		if r, ok := data.Resolved.Roles[id]; ok && r != nil {
			parts = append(parts, fmt.Sprintf("%s=@%s", id, r.Name))
			continue
		}
		if c, ok := data.Resolved.Channels[id]; ok && c != nil {
			parts = append(parts, fmt.Sprintf("%s=#%s", id, c.Name))
			continue
		}
	}
	return strings.Join(parts, ", ")
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

// Typing broadcasts the Discord "is typing…" indicator in the configured channel.
func (b *Bot) Typing() {
	if err := b.session.ChannelTyping(b.channelID); err != nil {
		b.logger.Debug("discord typing failed", "error", err)
	}
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
	components, modals, err := buildComponents(buttons, nil)
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
	b.registerModals(modals)
	return nil
}

// SendRich sends an embed (title/content/fields/color) plus optional components.
func (b *Bot) SendRich(msg messaging.RichMessage) error {
	components, modals, err := buildComponents(msg.Buttons, msg.Selects)
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
	b.registerModals(modals)
	return nil
}

// SendFile uploads a file from the local filesystem to the configured channel.
func (b *Bot) SendFile(path, filename, caption string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("path is required")
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("path is not a regular file")
	}
	if st.Size() <= 0 {
		return fmt.Errorf("file is empty")
	}
	if st.Size() > maxOutboundFileBytes {
		return fmt.Errorf("file exceeds %d byte limit", maxOutboundFileBytes)
	}

	name := strings.TrimSpace(filename)
	if name == "" {
		name = filepath.Base(path)
	}
	ct := mime.TypeByExtension(filepath.Ext(name))
	if ct == "" {
		ct = "application/octet-stream"
	}

	send := &discordgo.MessageSend{
		Content: strings.TrimSpace(caption),
		Files: []*discordgo.File{{
			Name:        name,
			ContentType: ct,
			Reader:      io.LimitReader(f, maxOutboundFileBytes+1),
		}},
	}
	_, err = b.session.ChannelMessageSendComplex(b.channelID, send)
	if err != nil {
		return fmt.Errorf("send discord file: %w", err)
	}
	return nil
}
