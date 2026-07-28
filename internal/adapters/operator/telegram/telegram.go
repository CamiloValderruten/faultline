// Package telegram is the Telegram-backed operator adapter for
// bidirectional collaborator communication.
package telegram

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	tgmd "github.com/Mad-Pixels/goldmark-tgmd"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot is a Telegram bot for bidirectional collaborator communication.
type Bot struct {
	bot    *tgbotapi.BotAPI
	chatID int64
	logger *slog.Logger

	media      InboundMedia
	httpClient *http.Client                        // optional; tests inject a fake
	fileURL    func(fileID string) (string, error) // optional; defaults to BotAPI
	apiBase    string                              // optional; defaults to api.telegram.org (tests)

	// answerCallback answers a callback query (tests inject a fake).
	answerCallback func(callbackID string) error

	mu      sync.Mutex
	pending []string
}

// New creates a new Telegram bot connection.
// Clears any existing webhook so long polling works.
func New(token string, chatID int64, logger *slog.Logger) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	logger.Info("telegram bot connected", "username", bot.Self.UserName)

	// Delete any existing webhook - webhooks block getUpdates (long polling).
	// This is the most common reason for the bot silently not receiving messages.
	deleteWebhook := tgbotapi.DeleteWebhookConfig{DropPendingUpdates: false}
	if _, err := bot.Request(deleteWebhook); err != nil {
		logger.Warn("failed to delete webhook (may not matter)", "error", err)
	} else {
		logger.Debug("cleared any existing webhook")
	}

	return &Bot{
		bot:    bot,
		chatID: chatID,
		logger: logger,
	}, nil
}

// Start begins listening for incoming messages.
// It blocks until the context is canceled.
func (t *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30

	updates := t.bot.GetUpdatesChan(u)
	t.logger.Info("telegram listener started, waiting for messages")

	go func() {
		<-ctx.Done()
		t.bot.StopReceivingUpdates()
	}()

	for update := range updates {
		if ctx.Err() != nil {
			return
		}

		t.logger.Debug("telegram update received",
			"update_id", update.UpdateID,
			"has_message", update.Message != nil,
			"has_callback", update.CallbackQuery != nil,
		)

		if update.CallbackQuery != nil {
			t.handleCallbackQuery(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}

		t.logger.Debug("telegram message",
			"chat_id", update.Message.Chat.ID,
			"from", update.Message.From.UserName,
			"text_len", len(update.Message.Text),
			"has_photo", len(update.Message.Photo) > 0,
			"has_document", update.Message.Document != nil,
		)

		// Only accept messages from the configured chat
		if update.Message.Chat.ID != t.chatID {
			t.logger.Warn("ignoring message from unknown chat",
				"chat_id", update.Message.Chat.ID,
				"expected_chat_id", t.chatID,
				"username", update.Message.From.UserName,
			)
			continue
		}

		text, ok := t.inboundText(update.Message)
		if !ok {
			continue
		}

		t.logger.Info("received message from collaborator", "text", text)

		t.mu.Lock()
		t.pending = append(t.pending, text)
		t.mu.Unlock()
	}

	t.logger.Info("telegram listener stopped")
}

// inboundText turns a Telegram message into an agent-facing string.
// Returns ok=false when the update should be ignored.
func (t *Bot) inboundText(msg *tgbotapi.Message) (string, bool) {
	if msg == nil {
		return "", false
	}

	caption := strings.TrimSpace(msg.Caption)
	text := strings.TrimSpace(msg.Text)

	if photo := pickLargestPhoto(msg.Photo); photo != nil {
		return t.inboundPhoto(photo.FileID, ".jpg", caption)
	}
	if imageDocument(msg.Document) {
		ext := extForImage(msg.Document.MimeType, msg.Document.FileName)
		return t.inboundPhoto(msg.Document.FileID, ext, caption)
	}

	if text == "" {
		return "", false
	}
	return text, true
}

func (t *Bot) inboundPhoto(fileID, ext, caption string) (string, bool) {
	if !t.mediaConfigured() {
		t.logger.Warn("collaborator sent a photo but inbound media is not configured (enable sandbox)")
		if caption != "" {
			return "Collaborator sent a photo (not saved — inbound media unavailable).\ncaption: " + caption, true
		}
		return "", false
	}

	path, err := t.saveFile(fileID, ext)
	if err != nil {
		t.logger.Error("failed to save collaborator photo", "error", err)
		if caption != "" {
			return "Collaborator sent a photo but saving it failed.\ncaption: " + caption, true
		}
		return "Collaborator sent a photo but saving it failed.", true
	}

	t.logger.Info("saved collaborator photo", "path", path)
	return formatPhotoNotice(path, caption), true
}

// mdConverter is the goldmark instance configured for Telegram MarkdownV2 output.
var mdConverter = tgmd.TGMD()

// listBullets are the Unicode bullet characters used by goldmark-tgmd.
// Used for post-processing to fix the extra newline after bullets.
var listBullets = []string{"•", "‣", "⁃"}

// toTelegramMarkdown converts standard markdown to Telegram MarkdownV2 format.
// Returns the converted text and true on success, or the original text and false on failure.
func toTelegramMarkdown(text string) (string, bool) {
	var buf bytes.Buffer
	if err := mdConverter.Convert([]byte(text), &buf); err != nil {
		return text, false
	}
	result := buf.String()
	if result == "" {
		return text, false
	}

	// Fix goldmark-tgmd bug: paragraph nodes inside list items emit an
	// extra newline after the bullet, producing "• \ntext" instead of "• text".
	for _, bullet := range listBullets {
		result = strings.ReplaceAll(result, bullet+" \n", bullet+" ")
	}

	return result, true
}

// Send sends a text message to the collaborator.
func (t *Bot) Send(text string) error {
	return t.sendChunks(text, nil)
}

// SendWithButtons sends a text message with an inline keyboard.
// Markup is attached only to the last chunk. buttons must be non-empty.
func (t *Bot) SendWithButtons(text string, buttons [][]messaging.Button) error {
	rows, err := validateButtons(buttons)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("text is required when sending buttons")
	}
	markup := buildInlineKeyboard(rows)
	return t.sendChunks(text, &markup)
}

func (t *Bot) sendChunks(text string, markup *tgbotapi.InlineKeyboardMarkup) error {
	const maxLen = 4000
	chunks := chunkText(text, maxLen)
	if len(chunks) == 0 {
		return fmt.Errorf("text is required")
	}

	for i, chunk := range chunks {
		var replyMarkup interface{}
		if markup != nil && i == len(chunks)-1 {
			replyMarkup = markup
		}
		if err := t.sendOneChunk(chunk, replyMarkup); err != nil {
			return err
		}
	}
	return nil
}

func (t *Bot) sendOneChunk(chunk string, replyMarkup interface{}) error {
	converted, ok := toTelegramMarkdown(chunk)
	if ok {
		msg := tgbotapi.NewMessage(t.chatID, converted)
		msg.ParseMode = tgbotapi.ModeMarkdownV2
		if replyMarkup != nil {
			msg.ReplyMarkup = replyMarkup
		}
		if _, err := t.bot.Send(msg); err != nil {
			t.logger.Debug("markdownV2 send failed, retrying as plain text", "error", err)
			msg = tgbotapi.NewMessage(t.chatID, chunk)
			if replyMarkup != nil {
				msg.ReplyMarkup = replyMarkup
			}
			if _, err := t.bot.Send(msg); err != nil {
				return fmt.Errorf("send telegram message: %w", err)
			}
		}
		return nil
	}

	msg := tgbotapi.NewMessage(t.chatID, chunk)
	if replyMarkup != nil {
		msg.ReplyMarkup = replyMarkup
	}
	if _, err := t.bot.Send(msg); err != nil {
		return fmt.Errorf("send telegram message: %w", err)
	}
	return nil
}

// handleCallbackQuery answers the query and enqueues a collaborator message.
func (t *Bot) handleCallbackQuery(cq *tgbotapi.CallbackQuery) {
	if cq == nil {
		return
	}

	if cq.Message == nil || cq.Message.Chat == nil {
		t.logger.Warn("ignoring callback without message chat")
		t.answerCallbackQuery(cq.ID)
		return
	}
	if cq.Message.Chat.ID != t.chatID {
		t.logger.Warn("ignoring callback from unknown chat",
			"chat_id", cq.Message.Chat.ID,
			"expected_chat_id", t.chatID,
		)
		t.answerCallbackQuery(cq.ID)
		return
	}

	buttonText := ""
	if cq.Message != nil {
		// Prefer the label matching this callback_data if present on the keyboard.
		buttonText = buttonTextForData(cq.Message, cq.Data)
	}
	if buttonText == "" {
		buttonText = cq.Data
	}

	pending := formatCallbackPending(buttonText, cq.Data)
	t.logger.Info("received button press from collaborator", "text", pending)

	t.mu.Lock()
	t.pending = append(t.pending, pending)
	t.mu.Unlock()

	t.answerCallbackQuery(cq.ID)
}

func (t *Bot) answerCallbackQuery(callbackID string) {
	if callbackID == "" {
		return
	}
	if t.answerCallback != nil {
		if err := t.answerCallback(callbackID); err != nil {
			t.logger.Debug("answerCallbackQuery failed", "error", err)
		}
		return
	}
	if t.bot == nil {
		return
	}
	cfg := tgbotapi.NewCallback(callbackID, "")
	if _, err := t.bot.Request(cfg); err != nil {
		t.logger.Debug("answerCallbackQuery failed", "error", err)
	}
}

func buttonTextForData(msg *tgbotapi.Message, data string) string {
	if msg == nil || msg.ReplyMarkup == nil {
		return ""
	}
	for _, row := range msg.ReplyMarkup.InlineKeyboard {
		for _, b := range row {
			if b.CallbackData != nil && *b.CallbackData == data {
				return b.Text
			}
		}
	}
	return ""
}

// Pending drains and returns all queued incoming messages.
func (t *Bot) Pending() []string {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.pending) == 0 {
		return nil
	}

	msgs := t.pending
	t.pending = nil
	return msgs
}

// HasPending reports whether any incoming messages are queued, without
// draining them. Used by the sleep tool to wake on operator input while
// leaving the queue intact for the agent loop's normal between-turn drain.
func (t *Bot) HasPending() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.pending) > 0
}

// Typing broadcasts Telegram's "typing…" chat action.
func (t *Bot) Typing() {
	action := tgbotapi.NewChatAction(t.chatID, tgbotapi.ChatTyping)
	if _, err := t.bot.Request(action); err != nil {
		t.logger.Debug("telegram typing failed", "error", err)
	}
}
