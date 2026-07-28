package telegram

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// InboundMedia configures where collaborator photos are written so sandbox
// MCP tools (e.g. understand_image) can read them via a bind mount.
// HostDir is on the Faultline host; ContainerPrefix is the path the agent
// and MCP servers should use (e.g. "/input/telegram").
type InboundMedia struct {
	HostDir         string
	ContainerPrefix string
}

// SetInboundMedia enables saving inbound photos/images. Safe to call with
// zero value to disable. Not concurrent with Start.
func (t *Bot) SetInboundMedia(m InboundMedia) {
	t.media = m
}

func (t *Bot) mediaConfigured() bool {
	return t.media.HostDir != "" && t.media.ContainerPrefix != ""
}

// pickLargestPhoto returns the highest-resolution PhotoSize, or nil.
func pickLargestPhoto(photos []tgbotapi.PhotoSize) *tgbotapi.PhotoSize {
	if len(photos) == 0 {
		return nil
	}
	best := &photos[0]
	for i := 1; i < len(photos); i++ {
		p := &photos[i]
		if p.FileSize > best.FileSize || (p.Width*p.Height > best.Width*best.Height) {
			best = p
		}
	}
	return best
}

func imageDocument(doc *tgbotapi.Document) bool {
	if doc == nil {
		return false
	}
	if strings.HasPrefix(strings.ToLower(doc.MimeType), "image/") {
		return true
	}
	// Some clients omit mime; accept common image extensions.
	name := strings.ToLower(doc.FileName)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".heif"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func extForImage(mime, fileName string) string {
	mime = strings.ToLower(mime)
	switch {
	case strings.Contains(mime, "png"):
		return ".png"
	case strings.Contains(mime, "gif"):
		return ".gif"
	case strings.Contains(mime, "webp"):
		return ".webp"
	case strings.Contains(mime, "heic"), strings.Contains(mime, "heif"):
		return ".heic"
	}
	name := strings.ToLower(fileName)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".heic", ".heif"} {
		if strings.HasSuffix(name, ext) {
			if ext == ".jpeg" {
				return ".jpg"
			}
			return ext
		}
	}
	return ".jpg"
}

// safeFileStem keeps sandbox filename rules in mind: [a-z0-9._-]+.
func safeFileStem(fileID string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(fileID) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" {
		s = "img"
	}
	if len(s) > 24 {
		s = s[:24]
	}
	return s
}

func formatPhotoNotice(containerPath, caption string) string {
	var b strings.Builder
	b.WriteString("Collaborator sent a photo.\n")
	b.WriteString("image_path: ")
	b.WriteString(containerPath)
	b.WriteString("\n")
	if caption != "" {
		b.WriteString("caption: ")
		b.WriteString(caption)
		b.WriteString("\n")
	}
	b.WriteString("Call mcp_minimax_understand_image with image_url set to image_path (a local path visible inside the sandbox).")
	return b.String()
}

// saveFile downloads a Telegram file by file_id into the inbound media dir
// and returns the container path the agent should use. The Telegram CDN URL
// (which embeds the bot token) is never returned to the agent.
func (t *Bot) saveFile(fileID, ext string) (string, error) {
	if !t.mediaConfigured() {
		return "", fmt.Errorf("inbound media not configured")
	}
	if err := os.MkdirAll(t.media.HostDir, 0o755); err != nil {
		return "", fmt.Errorf("create media dir: %w", err)
	}

	getURL := t.fileURL
	if getURL == nil {
		getURL = t.bot.GetFileDirectURL
	}
	directURL, err := getURL(fileID)
	if err != nil {
		return "", fmt.Errorf("get file url: %w", err)
	}

	client := t.httpClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Get(directURL)
	if err != nil {
		return "", fmt.Errorf("download file: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: HTTP %d", resp.StatusCode)
	}

	name := fmt.Sprintf("%s-%s%s",
		time.Now().UTC().Format("20060102T150405"),
		safeFileStem(fileID),
		ext,
	)
	hostPath := filepath.Join(t.media.HostDir, name)
	f, err := os.OpenFile(hostPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create media file: %w", err)
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Remove(hostPath)
		return "", fmt.Errorf("write media file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(hostPath)
		return "", fmt.Errorf("close media file: %w", closeErr)
	}

	prefix := strings.TrimRight(t.media.ContainerPrefix, "/")
	return prefix + "/" + name, nil
}
