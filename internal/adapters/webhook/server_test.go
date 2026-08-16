package webhook

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInboxRequiresBearer(t *testing.T) {
	s := newTestServer(t, func(string, bool) {})
	req := httptest.NewRequest(http.MethodPost, "/v1/inbox", strings.NewReader(`{"text":"hi"}`))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", rec.Code)
	}
}

func TestInboxRejectsEmptyText(t *testing.T) {
	s := newTestServer(t, func(string, bool) {})
	rec := postInbox(t, s, "secret", `{"text":"  "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rec.Code)
	}
}

func TestInboxPushesUrgent(t *testing.T) {
	var gotText string
	var gotUrgent bool
	s := newTestServer(t, func(text string, urgent bool) {
		gotText = text
		gotUrgent = urgent
	})
	rec := postInbox(t, s, "secret", `{"text":"page me","urgent":true}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotText != "page me" || !gotUrgent {
		t.Fatalf("text=%q urgent=%v", gotText, gotUrgent)
	}
}

func TestHealthz(t *testing.T) {
	s := newTestServer(t, func(string, bool) {})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
}

func newTestServer(t *testing.T, push PushFunc) *Server {
	t.Helper()
	s, err := NewServer("127.0.0.1:0", "secret", push, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func postInbox(t *testing.T, s *Server, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/v1/inbox", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}
