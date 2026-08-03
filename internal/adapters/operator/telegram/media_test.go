package telegram

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func TestPickLargestPhoto(t *testing.T) {
	photos := []tgbotapi.PhotoSize{
		{FileID: "a", Width: 90, Height: 90, FileSize: 100},
		{FileID: "b", Width: 320, Height: 320, FileSize: 400},
		{FileID: "c", Width: 800, Height: 800, FileSize: 2000},
	}
	got := pickLargestPhoto(photos)
	if got == nil || got.FileID != "c" {
		t.Fatalf("pickLargestPhoto = %+v, want file_id=c", got)
	}
	if pickLargestPhoto(nil) != nil {
		t.Fatal("expected nil for empty slice")
	}
}

func TestFormatPhotoNotice(t *testing.T) {
	got := formatPhotoNotice("/input/telegram/x.jpg", "look")
	if !strings.Contains(got, "image_path: /input/telegram/x.jpg") {
		t.Fatalf("missing path: %q", got)
	}
	if !strings.Contains(got, "caption: look") {
		t.Fatalf("missing caption: %q", got)
	}
	if !strings.Contains(got, "mcp_minimax_understand_image") {
		t.Fatalf("missing tool hint: %q", got)
	}
}

func TestSafeFileStem(t *testing.T) {
	got := safeFileStem("AbC/../x Y!")
	for _, r := range got {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			t.Fatalf("unsafe stem %q", got)
		}
	}
	if safeFileStem("!!!") != "img" {
		t.Fatalf("empty stem fallback: %q", safeFileStem("!!!"))
	}
}

func TestImageDocument(t *testing.T) {
	if !imageDocument(&tgbotapi.Document{MimeType: "image/png", FileName: "x.bin"}) {
		t.Fatal("expected image mime to match")
	}
	if !imageDocument(&tgbotapi.Document{FileName: "shot.JPEG"}) {
		t.Fatal("expected jpeg extension to match")
	}
	if imageDocument(&tgbotapi.Document{MimeType: "application/pdf", FileName: "a.pdf"}) {
		t.Fatal("pdf should not match")
	}
}

func TestExtForImage(t *testing.T) {
	if extForImage("image/png", "") != ".png" {
		t.Fatal("png")
	}
	if extForImage("", "x.WEBP") != ".webp" {
		t.Fatal("webp")
	}
	if extForImage("", "") != ".jpg" {
		t.Fatal("default jpg")
	}
}

func TestSaveFile_DownloadsWithoutLeakingURLToCaller(t *testing.T) {
	const body = "fake-image-bytes"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/secret-token/file.jpg" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	b := &Bot{
		logger:     slog.Default(),
		httpClient: srv.Client(),
		fileURL: func(fileID string) (string, error) {
			if fileID != "fid1" {
				t.Fatalf("fileID = %q", fileID)
			}
			return srv.URL + "/secret-token/file.jpg", nil
		},
		media: InboundMedia{
			HostDir:         filepath.Join(dir, "telegram"),
			ContainerPrefix: "/input/telegram",
		},
	}

	containerPath, err := b.saveFile("fid1", ".jpg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(containerPath, "/input/telegram/") || !strings.HasSuffix(containerPath, ".jpg") {
		t.Fatalf("container path = %q", containerPath)
	}
	if strings.Contains(containerPath, "secret-token") {
		t.Fatalf("token leaked into container path: %q", containerPath)
	}

	name := filepath.Base(containerPath)
	got, err := os.ReadFile(filepath.Join(b.media.HostDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != body {
		t.Fatalf("file body = %q", got)
	}
}

func TestInboundText_PhotoRequiresMedia(t *testing.T) {
	b := &Bot{logger: slog.Default()}
	msg := &tgbotapi.Message{
		Photo:   []tgbotapi.PhotoSize{{FileID: "x", Width: 1, Height: 1}},
		Caption: "baby",
	}
	text, ok := b.inboundText(msg)
	if !ok {
		t.Fatal("expected caption fallback when media unset")
	}
	if !strings.Contains(text, "caption: baby") {
		t.Fatalf("got %q", text)
	}
}
