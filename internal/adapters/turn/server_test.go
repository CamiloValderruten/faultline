package turn

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/CamiloValderruten/faultline/internal/agent"
)

func TestHandleTurnUnauthorized(t *testing.T) {
	s := mustServer(t, func(context.Context, string) (string, error) {
		t.Fatal("submit should not run")
		return "", nil
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/turn", strings.NewReader(`{"text":"hi"}`))
	s.handleTurn(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTurnSuccess(t *testing.T) {
	s := mustServer(t, func(_ context.Context, text string) (string, error) {
		if text != "hello" {
			t.Fatalf("text=%q", text)
		}
		return "hi there", nil
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/turn", strings.NewReader(`{"text":"hello"}`))
	req.Header.Set("Authorization", "Bearer secret")
	s.handleTurn(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var got struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Text != "hi there" {
		t.Fatalf("got %q", got.Text)
	}
}

func TestHandleTurnBusy(t *testing.T) {
	s := mustServer(t, func(context.Context, string) (string, error) {
		return "", agent.ErrTurnBusy
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/turn", strings.NewReader(`{"text":"hello"}`))
	req.Header.Set("Authorization", "Bearer secret")
	s.handleTurn(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTurnTooLarge(t *testing.T) {
	s := mustServer(t, func(context.Context, string) (string, error) {
		t.Fatal("submit should not run")
		return "", nil
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/turn", strings.NewReader(`{"text":"`+strings.Repeat("a", maxBodyBytes)+`"}`))
	req.Header.Set("Authorization", "Bearer secret")
	s.handleTurn(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestHandleTurnEmptyText(t *testing.T) {
	s := mustServer(t, func(context.Context, string) (string, error) {
		t.Fatal("submit should not run")
		return "", nil
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/turn", strings.NewReader(`{"text":"  "}`))
	req.Header.Set("Authorization", "Bearer secret")
	s.handleTurn(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestStartHealthz(t *testing.T) {
	var mu sync.Mutex
	called := false
	s, err := NewServer("127.0.0.1:0", "secret", time.Second, func(context.Context, string) (string, error) {
		mu.Lock()
		called = true
		mu.Unlock()
		return "x", nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.Start(ctx)
	defer s.Shutdown()

	// ListenAndServe is async; httptest the handler directly for healthz.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	s.srv.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz status=%d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "ok\n" {
		t.Fatalf("body=%q", body)
	}
	mu.Lock()
	defer mu.Unlock()
	if called {
		t.Fatal("submit ran on healthz")
	}
}

func mustServer(t *testing.T, submit SubmitFunc) *Server {
	t.Helper()
	s, err := NewServer("127.0.0.1:0", "secret", time.Second, submit, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
