package discord

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	"github.com/bwmarrin/discordgo"
	"github.com/pion/webrtc/v3/pkg/media/oggreader"
)

// Speech is optional STT/TTS used for collaborator voice notes.
type Speech interface {
	Transcribe(ctx context.Context, audio []byte, contentType string) (string, error)
	Speak(ctx context.Context, text string) (audio []byte, contentType string, err error)
}

// SetSpeech enables voice-note transcription and send_voice_message.
func (b *Bot) SetSpeech(speech Speech) {
	b.speech = speech
}

// VoiceEnabled reports whether outbound voice replies are available.
func (b *Bot) VoiceEnabled() bool {
	return b.speech != nil
}

func pickVoiceAttachment(msg *discordgo.Message) *discordgo.MessageAttachment {
	if msg == nil || len(msg.Attachments) == 0 {
		return nil
	}
	isVoice := msg.Flags&discordgo.MessageFlagsIsVoiceMessage != 0
	for _, att := range msg.Attachments {
		if att == nil {
			continue
		}
		if isVoice || isAudioAttachment(att) {
			return att
		}
	}
	return nil
}

func isAudioAttachment(att *discordgo.MessageAttachment) bool {
	if att == nil {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(att.ContentType))
	if strings.HasPrefix(ct, "audio/") {
		return true
	}
	name := strings.ToLower(att.Filename)
	for _, ext := range []string{".ogg", ".oga", ".mp3", ".wav", ".m4a", ".webm", ".opus"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func (b *Bot) inboundVoice(att *discordgo.MessageAttachment) (string, bool) {
	if b.speech == nil {
		b.logger.Warn("collaborator sent a voice note but speech (Deepgram) is not configured")
		return "Collaborator sent a voice note (transcription unavailable — configure [deepgram]).", true
	}

	audio, contentType, err := b.downloadAttachmentBytes(att)
	if err != nil {
		b.logger.Error("failed to download voice note", "error", err)
		return "Collaborator sent a voice note but downloading it failed.", true
	}
	if contentType == "" {
		contentType = strings.TrimSpace(att.ContentType)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	transcript, err := b.speech.Transcribe(ctx, audio, contentType)
	if err != nil {
		b.logger.Error("voice note transcription failed", "error", err)
		return "Collaborator sent a voice note but transcription failed.", true
	}

	b.logger.Info("transcribed collaborator voice note", "chars", len(transcript))
	return messaging.VoiceNotePreamble + transcript, true
}

func (b *Bot) downloadAttachmentBytes(att *discordgo.MessageAttachment) ([]byte, string, error) {
	if att == nil || strings.TrimSpace(att.URL) == "" {
		return nil, "", fmt.Errorf("attachment url missing")
	}
	client := b.media.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(att.URL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download attachment HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 25<<20))
	if err != nil {
		return nil, "", err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = att.ContentType
	}
	return data, ct, nil
}

// oggOpusDurationSecs estimates playback length from Ogg pages (~20ms each).
func oggOpusDurationSecs(ogg []byte) float64 {
	reader, _, err := oggreader.NewWith(bytes.NewReader(ogg))
	if err != nil {
		return fallbackOggDuration(len(ogg))
	}
	pages := 0
	for {
		page, _, err := reader.ParseNextPage()
		if err != nil {
			break
		}
		if len(page) > 0 {
			pages++
		}
	}
	if pages < 1 {
		return fallbackOggDuration(len(ogg))
	}
	secs := float64(pages) * 0.02
	if secs < 0.5 {
		secs = 0.5
	}
	return secs
}

func fallbackOggDuration(nbytes int) float64 {
	// ~24 kbps speech bitrate estimate.
	secs := float64(nbytes) * 8 / 24000
	if secs < 0.5 {
		secs = 0.5
	}
	return secs
}

// waveformFromBytes builds a Discord voice-message waveform preview (≤256 samples).
func waveformFromBytes(data []byte) string {
	const n = 256
	out := make([]byte, n)
	if len(data) == 0 {
		return base64.StdEncoding.EncodeToString(out)
	}
	chunk := len(data) / n
	if chunk < 1 {
		chunk = 1
	}
	for i := 0; i < n; i++ {
		start := i * chunk
		if start >= len(data) {
			break
		}
		end := start + chunk
		if end > len(data) {
			end = len(data)
		}
		var max byte
		for _, b := range data[start:end] {
			if b > max {
				max = b
			}
		}
		out[i] = max
	}
	return base64.StdEncoding.EncodeToString(out)
}

func stripForSpeech(s string) string {
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "~~", "")
	s = strings.ReplaceAll(s, "`", "")
	var b strings.Builder
	for _, field := range strings.Fields(s) {
		if strings.HasPrefix(field, "http://") || strings.HasPrefix(field, "https://") {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(field)
	}
	out := strings.TrimSpace(b.String())
	const maxRunes = 1800
	if utf8.RuneCountInString(out) > maxRunes {
		runes := []rune(out)
		out = string(runes[:maxRunes])
	}
	return out
}
