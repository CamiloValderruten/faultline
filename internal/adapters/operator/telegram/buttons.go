package telegram

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	maxButtonsTotal      = 8
	maxCallbackDataBytes = 64
)

// validateButtons enforces Telegram limits. Returns a copy with trimmed fields.
func validateButtons(rows [][]messaging.Button) ([][]messaging.Button, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("buttons required")
	}
	total := 0
	out := make([][]messaging.Button, 0, len(rows))
	for i, row := range rows {
		if len(row) == 0 {
			return nil, fmt.Errorf("buttons row %d is empty", i)
		}
		outRow := make([]messaging.Button, 0, len(row))
		for j, b := range row {
			text := strings.TrimSpace(b.Text)
			data := strings.TrimSpace(b.Data)
			url := strings.TrimSpace(b.URL)
			if text == "" {
				return nil, fmt.Errorf("buttons[%d][%d].text is required", i, j)
			}
			if url == "" && data == "" {
				return nil, fmt.Errorf("buttons[%d][%d].data or url is required", i, j)
			}
			if url == "" && len(data) > maxCallbackDataBytes {
				return nil, fmt.Errorf("buttons[%d][%d].data exceeds %d bytes", i, j, maxCallbackDataBytes)
			}
			total++
			if total > maxButtonsTotal {
				return nil, fmt.Errorf("at most %d buttons allowed", maxButtonsTotal)
			}
			outRow = append(outRow, messaging.Button{
				Text:  text,
				Data:  data,
				Style: strings.TrimSpace(b.Style),
				URL:   url,
			})
		}
		out = append(out, outRow)
	}
	return out, nil
}

func buildInlineKeyboard(rows [][]messaging.Button) tgbotapi.InlineKeyboardMarkup {
	kb := make([][]tgbotapi.InlineKeyboardButton, 0, len(rows))
	for _, row := range rows {
		kbRow := make([]tgbotapi.InlineKeyboardButton, 0, len(row))
		for _, b := range row {
			if b.URL != "" {
				kbRow = append(kbRow, tgbotapi.NewInlineKeyboardButtonURL(b.Text, b.URL))
				continue
			}
			kbRow = append(kbRow, tgbotapi.NewInlineKeyboardButtonData(b.Text, b.Data))
		}
		kb = append(kb, kbRow)
	}
	return tgbotapi.NewInlineKeyboardMarkup(kb...)
}

func formatCallbackPending(buttonText, data string) string {
	buttonText = strings.TrimSpace(buttonText)
	data = strings.TrimSpace(data)
	if buttonText == "" {
		buttonText = data
	}
	return fmt.Sprintf("Pressed button %q (data=%s)", buttonText, data)
}

// chunkText splits text into Telegram-safe chunks (same rules as Send).
func chunkText(text string, maxLen int) []string {
	if text == "" {
		return nil
	}
	var chunks []string
	for len(text) > 0 {
		chunk := text
		if len(chunk) > maxLen {
			cut := maxLen
			for cut > 0 && !utf8.RuneStart(text[cut]) {
				cut--
			}
			for i := cut; i > cut-500 && i > 0; i-- {
				if text[i] == '\n' {
					cut = i + 1
					break
				}
			}
			chunk = text[:cut]
			text = text[cut:]
		} else {
			text = ""
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}
