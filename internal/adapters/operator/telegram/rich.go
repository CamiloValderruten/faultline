package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/CamiloValderruten/faultline/internal/messaging"
)

const (
	defaultTelegramAPI = "https://api.telegram.org"
	// Telegram rich messages allow up to 32768 UTF-8 characters.
	maxRichContentLen = 32000
)

// SendRich sends a structured digest. Title/fields/selects are flattened to
// markdown; Discord-only styles are ignored. Buttons use an inline keyboard
// (rich API has no components), otherwise Bot API sendRichMessage is tried
// with markdown fallback to Send.
func (t *Bot) SendRich(msg messaging.RichMessage) error {
	body := messaging.FlattenText(msg)
	if body == "" && len(msg.Buttons) == 0 {
		return fmt.Errorf("rich content is required")
	}
	if len(msg.Buttons) > 0 {
		if body == "" {
			body = "(choose an option)"
		}
		return t.SendWithButtons(body, msg.Buttons)
	}
	return t.sendRichDigest(body)
}

func (t *Bot) sendRichDigest(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("rich content is required")
	}
	if len(content) > maxRichContentLen || t.bot == nil || t.bot.Token == "" {
		if len(content) > maxRichContentLen {
			t.logger.Info("rich content too long, falling back to Send", "length", len(content))
		}
		return t.Send(content)
	}

	if err := t.postRichMessage(content); err != nil {
		t.logger.Info("sendRichMessage failed, falling back to Send", "error", err)
		return t.Send(content)
	}
	return nil
}

func (t *Bot) postRichMessage(content string) error {
	apiBase := t.apiBase
	if apiBase == "" {
		apiBase = defaultTelegramAPI
	}
	client := t.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	// Bot API 10.1+ InputRichMessage: agents write Markdown digests.
	payload := map[string]any{
		"chat_id": t.chatID,
		"rich_message": map[string]any{
			"markdown": content,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/bot%s/sendRichMessage", strings.TrimRight(apiBase, "/"), t.bot.Token)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))

	var apiResp struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.Unmarshal(respBody, &apiResp)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !apiResp.OK {
		desc := apiResp.Description
		if desc == "" {
			desc = string(respBody)
		}
		return fmt.Errorf("sendRichMessage HTTP %d: %s", resp.StatusCode, desc)
	}
	return nil
}
