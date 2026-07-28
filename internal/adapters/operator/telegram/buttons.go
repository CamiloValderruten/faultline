package telegram

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Button is one inline keyboard button. Data is sent back as callback_data.
type Button struct {
	Text string
	Data string
}

const (
	maxButtonsTotal     = 8
	maxCallbackDataBytes = 64
)

// validateButtons enforces Telegram limits. Returns a copy with trimmed fields.
func validateButtons(rows [][]Button) ([][]Button, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("buttons required")
	}
	total := 0
	out := make([][]Button, 0, len(rows))
	for i, row := range rows {
		if len(row) == 0 {
			return nil, fmt.Errorf("buttons row %d is empty", i)
		}
		outRow := make([]Button, 0, len(row))
		for j, b := range row {
			text := strings.TrimSpace(b.Text)
			data := strings.TrimSpace(b.Data)
			if text == "" {
				return nil, fmt.Errorf("buttons[%d][%d].text is required", i, j)
			}
			if data == "" {
				return nil, fmt.Errorf("buttons[%d][%d].data is required", i, j)
			}
			if len(data) > maxCallbackDataBytes {
				return nil, fmt.Errorf("buttons[%d][%d].data exceeds %d bytes", i, j, maxCallbackDataBytes)
			}
			total++
			if total > maxButtonsTotal {
				return nil, fmt.Errorf("at most %d buttons allowed", maxButtonsTotal)
			}
			outRow = append(outRow, Button{Text: text, Data: data})
		}
		out = append(out, outRow)
	}
	return out, nil
}

func buildInlineKeyboard(rows [][]Button) tgbotapi.InlineKeyboardMarkup {
	kb := make([][]tgbotapi.InlineKeyboardButton, 0, len(rows))
	for _, row := range rows {
		kbRow := make([]tgbotapi.InlineKeyboardButton, 0, len(row))
		for _, b := range row {
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
