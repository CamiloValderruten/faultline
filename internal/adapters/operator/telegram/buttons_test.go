package telegram

import (
	"log/slog"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestValidateButtons_OK(t *testing.T) {
	rows, err := validateButtons([][]Button{
		{{Text: "Approve", Data: "approve"}, {Text: "Deny", Data: "deny"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestValidateButtons_TooMany(t *testing.T) {
	var row []Button
	for i := 0; i < 9; i++ {
		row = append(row, Button{Text: "B", Data: "d"})
	}
	_, err := validateButtons([][]Button{row})
	if err == nil {
		t.Fatal("expected error for >8 buttons")
	}
}

func TestValidateButtons_CallbackDataTooLong(t *testing.T) {
	_, err := validateButtons([][]Button{
		{{Text: "X", Data: strings.Repeat("a", 65)}},
	})
	if err == nil {
		t.Fatal("expected error for long callback_data")
	}
}

func TestValidateButtons_EmptyText(t *testing.T) {
	_, err := validateButtons([][]Button{{{Text: " ", Data: "ok"}}})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestBuildInlineKeyboard(t *testing.T) {
	kb := buildInlineKeyboard([][]Button{
		{{Text: "A", Data: "a"}},
		{{Text: "B", Data: "b"}, {Text: "C", Data: "c"}},
	})
	if len(kb.InlineKeyboard) != 2 {
		t.Fatalf("rows=%d", len(kb.InlineKeyboard))
	}
	if len(kb.InlineKeyboard[1]) != 2 {
		t.Fatalf("row1 len=%d", len(kb.InlineKeyboard[1]))
	}
	if kb.InlineKeyboard[0][0].CallbackData == nil || *kb.InlineKeyboard[0][0].CallbackData != "a" {
		t.Fatalf("callback data: %+v", kb.InlineKeyboard[0][0].CallbackData)
	}
}

func TestFormatCallbackPending(t *testing.T) {
	got := formatCallbackPending("Approve", "mcp_approve")
	want := `Pressed button "Approve" (data=mcp_approve)`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestHandleCallbackQuery_Enqueues(t *testing.T) {
	var answered string
	bot := &Bot{
		chatID: 42,
		logger: slog.Default(),
		answerCallback: func(id string) error {
			answered = id
			return nil
		},
	}
	data := "approve"
	cq := &tgbotapi.CallbackQuery{
		ID:   "cb1",
		Data: data,
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 42},
			ReplyMarkup: &tgbotapi.InlineKeyboardMarkup{
				InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{
					{tgbotapi.NewInlineKeyboardButtonData("Approve", data)},
				},
			},
		},
	}
	bot.handleCallbackQuery(cq)
	pending := bot.Pending()
	if len(pending) != 1 {
		t.Fatalf("pending=%v", pending)
	}
	if !strings.Contains(pending[0], "Approve") || !strings.Contains(pending[0], "approve") {
		t.Fatalf("pending text=%q", pending[0])
	}
	if answered != "cb1" {
		t.Fatalf("answered=%q", answered)
	}
}

func TestHandleCallbackQuery_WrongChat(t *testing.T) {
	bot := &Bot{
		chatID:         42,
		logger:         slog.Default(),
		answerCallback: func(string) error { return nil },
	}
	bot.handleCallbackQuery(&tgbotapi.CallbackQuery{
		ID:   "cb2",
		Data: "x",
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: 99},
		},
	})
	if bot.HasPending() {
		t.Fatal("should ignore wrong chat")
	}
}

func TestChunkText(t *testing.T) {
	chunks := chunkText(strings.Repeat("a", 4500), 4000)
	if len(chunks) != 2 {
		t.Fatalf("chunks=%d", len(chunks))
	}
}
