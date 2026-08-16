package webhook

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxBodyBytes = 64 << 10

// PushFunc enqueues a webhook payload into the agent inbox.
type PushFunc func(text string, urgent bool)

// Server is a tiny authenticated HTTP listener for POST /v1/inbox.
type Server struct {
	bind   string
	token  string
	push   PushFunc
	logger *slog.Logger

	srv      *http.Server
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func NewServer(bind, token string, push PushFunc, logger *slog.Logger) (*Server, error) {
	if bind == "" {
		return nil, fmt.Errorf("webhook.bind is required")
	}
	if token == "" {
		return nil, fmt.Errorf("webhook.token is required")
	}
	if push == nil {
		return nil, fmt.Errorf("webhook push is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{bind: bind, token: token, push: push, logger: logger}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/inbox", s.handleInbox)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	return mux
}

func (s *Server) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.srv = &http.Server{
		Addr:              s.bind,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("inbox webhook listening", "bind", s.bind)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("inbox webhook failed", "error", err)
		}
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-ctx.Done()
		s.Shutdown()
	}()
}

func (s *Server) Wait() {
	if s != nil {
		s.wg.Wait()
	}
}

func (s *Server) Shutdown() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.srv == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(ctx)
	})
}

type inboxBody struct {
	Text   string `json:"text"`
	Urgent bool   `json:"urgent"`
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body) > maxBodyBytes {
		http.Error(w, "message too large", http.StatusRequestEntityTooLarge)
		return
	}
	var req inboxBody
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Text) == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}
	s.logger.Info("inbox webhook accepted", "urgent", req.Urgent)
	s.push(req.Text, req.Urgent)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) authorized(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	got := h[len(prefix):]
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) == 1
}
