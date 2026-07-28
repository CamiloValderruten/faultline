// Package deepgram is a thin HTTP client for Deepgram STT and TTS.
package deepgram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL  = "https://api.deepgram.com"
	defaultSTTModel = "nova-3"
	defaultTTSModel = "aura-2-thalia-en"
	maxTTSChars     = 2000 // Aura REST limit
)

// Client talks to Deepgram's pre-recorded listen and speak REST APIs.
type Client struct {
	apiKey   string
	sttModel string
	ttsModel string
	baseURL  string
	http     *http.Client
}

// New constructs a Deepgram client. apiKey is required.
func New(apiKey, sttModel, ttsModel string) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, fmt.Errorf("deepgram api_key is required")
	}
	if strings.TrimSpace(sttModel) == "" {
		sttModel = defaultSTTModel
	}
	if strings.TrimSpace(ttsModel) == "" {
		ttsModel = defaultTTSModel
	}
	return &Client{
		apiKey:   apiKey,
		sttModel: sttModel,
		ttsModel: ttsModel,
		baseURL:  defaultBaseURL,
		http:     &http.Client{Timeout: 120 * time.Second},
	}, nil
}

// Transcribe sends raw audio bytes to /v1/listen and returns the transcript.
func (c *Client) Transcribe(ctx context.Context, audio []byte, contentType string) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("audio is empty")
	}
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	u, err := url.Parse(strings.TrimRight(c.baseURL, "/") + "/v1/listen")
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("model", c.sttModel)
	q.Set("smart_format", "true")
	q.Set("punctuate", "true")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(audio))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("deepgram listen: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("deepgram listen HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var parsed struct {
		Results struct {
			Channels []struct {
				Alternatives []struct {
					Transcript string `json:"transcript"`
				} `json:"alternatives"`
			} `json:"channels"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("deepgram listen decode: %w", err)
	}
	if len(parsed.Results.Channels) == 0 || len(parsed.Results.Channels[0].Alternatives) == 0 {
		return "", fmt.Errorf("deepgram listen: empty transcript")
	}
	text := strings.TrimSpace(parsed.Results.Channels[0].Alternatives[0].Transcript)
	if text == "" {
		return "", fmt.Errorf("deepgram listen: empty transcript")
	}
	return text, nil
}

// Speak converts text to MP3 audio via /v1/speak.
func (c *Client) Speak(ctx context.Context, text string) ([]byte, string, error) {
	body, err := c.speakEncoding(ctx, text, "mp3", "")
	if err != nil {
		return nil, "", err
	}
	return body, "audio/mpeg", nil
}

// SpeakOggOpus converts text to an Ogg/Opus container suitable for Discord voice playback.
func (c *Client) SpeakOggOpus(ctx context.Context, text string) ([]byte, error) {
	return c.speakEncoding(ctx, text, "opus", "ogg")
}

func (c *Client) speakEncoding(ctx context.Context, text, encoding, container string) ([]byte, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	if len([]rune(text)) > maxTTSChars {
		runes := []rune(text)
		text = string(runes[:maxTTSChars])
	}

	u, err := url.Parse(strings.TrimRight(c.baseURL, "/") + "/v1/speak")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("model", c.ttsModel)
	q.Set("encoding", encoding)
	if container != "" {
		q.Set("container", container)
	}
	u.RawQuery = q.Encode()

	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("deepgram speak: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("deepgram speak HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("deepgram speak: empty audio")
	}
	return body, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
