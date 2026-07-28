package discord

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// InboundMedia configures where collaborator attachments are saved so the
// sandbox (and MCP vision tools) can read them at ContainerPrefix.
type InboundMedia struct {
	HostDir         string
	ContainerPrefix string
	HTTPClient      *http.Client // optional; tests inject a fake
}

// SetInboundMedia enables saving inbound image attachments.
func (b *Bot) SetInboundMedia(media InboundMedia) {
	b.media = media
}

func (b *Bot) mediaConfigured() bool {
	return strings.TrimSpace(b.media.HostDir) != "" && strings.TrimSpace(b.media.ContainerPrefix) != ""
}

func (b *Bot) inboundAttachments(atts []*discordgo.MessageAttachment, caption string) (string, bool) {
	if len(atts) == 0 {
		if caption != "" {
			return caption, true
		}
		return "", false
	}

	var notices []string
	for _, att := range atts {
		if att == nil {
			continue
		}
		if !isImageAttachment(att) {
			if isAudioAttachment(att) {
				// Handled by pickVoiceAttachment / inboundVoice before this path.
				continue
			}
			name := strings.TrimSpace(att.Filename)
			if name == "" {
				name = "file"
			}
			notices = append(notices, fmt.Sprintf("Collaborator sent a non-image attachment (%s).", name))
			continue
		}
		if !b.mediaConfigured() {
			b.logger.Warn("collaborator sent an image but inbound media is not configured (enable sandbox)")
			notices = append(notices, "Collaborator sent an image (not saved — inbound media unavailable).")
			continue
		}
		path, err := b.saveAttachment(att)
		if err != nil {
			b.logger.Error("failed to save collaborator attachment", "error", err, "filename", att.Filename)
			notices = append(notices, "Collaborator sent an image but saving it failed.")
			continue
		}
		b.logger.Info("saved collaborator image", "path", path)
		notices = append(notices, formatPhotoNotice(path, ""))
	}

	if caption != "" {
		notices = append(notices, "caption: "+caption)
	}
	if len(notices) == 0 {
		return "", false
	}
	return strings.Join(notices, "\n"), true
}

func isImageAttachment(att *discordgo.MessageAttachment) bool {
	if att == nil {
		return false
	}
	ct := strings.ToLower(strings.TrimSpace(att.ContentType))
	if strings.HasPrefix(ct, "image/") {
		return true
	}
	name := strings.ToLower(att.Filename)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

func (b *Bot) saveAttachment(att *discordgo.MessageAttachment) (string, error) {
	if att == nil || strings.TrimSpace(att.URL) == "" {
		return "", fmt.Errorf("attachment url missing")
	}
	ext := extForAttachment(att)
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	hostPath := filepath.Join(b.media.HostDir, name)
	if err := os.MkdirAll(b.media.HostDir, 0o755); err != nil {
		return "", err
	}

	client := b.media.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(att.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download attachment HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(hostPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 20<<20)); err != nil {
		_ = os.Remove(hostPath)
		return "", err
	}

	prefix := strings.TrimRight(b.media.ContainerPrefix, "/")
	return prefix + "/" + name, nil
}

func extForAttachment(att *discordgo.MessageAttachment) string {
	name := strings.ToLower(att.Filename)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp"} {
		if strings.HasSuffix(name, ext) {
			if ext == ".jpeg" {
				return ".jpg"
			}
			return ext
		}
	}
	ct := strings.ToLower(att.ContentType)
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "webp"):
		return ".webp"
	default:
		return ".jpg"
	}
}

func formatPhotoNotice(containerPath, caption string) string {
	var b strings.Builder
	b.WriteString("Collaborator sent a photo.\n")
	b.WriteString("image_path: ")
	b.WriteString(containerPath)
	b.WriteString("\n")
	b.WriteString("Use an image-understanding MCP tool (if available) with this path, or describe it from context.")
	if caption = strings.TrimSpace(caption); caption != "" {
		b.WriteString("\ncaption: ")
		b.WriteString(caption)
	}
	return b.String()
}
