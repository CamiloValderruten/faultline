// Package turn is a tiny loopback HTTP listener for blocking local-voice
// turns. Separate from the public publish server and the admin UI.
package turn

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/CamiloValderruten/faultline/internal/agent"
)

const maxBodyBytes = 16 << 10

// SubmitFunc runs one local-voice turn and returns spoken reply text.
type SubmitFunc func(ctx context.Context, text string) (string, error)

// Server accepts POST /v1/turn.
type Server struct {
	bind    string
	token   string
	timeout time.Duration
	submit  SubmitFunc
	logger  *slog.Logger

	srv      *http.Server
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewServer constructs a local-turn HTTP server. Call Start to listen.
func NewServer(bind, token string, timeout time.Duration, submit SubmitFunc, logger *slog.Logger) (*Server, error) {
	if bind == "" {
		return nil, fmt.Errorf("turn.bind is required")
	}
	if token == "" {
		return nil, fmt.Errorf("turn.token is required")
	}
	if submit == nil {
		return nil, fmt.Errorf("turn submit is required")
	}
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{bind: bind, token: token, timeout: timeout, submit: submit, logger: logger}, nil
}

// Start listens until ctx is canceled.
func (s *Server) Start(ctx context.Context) {
	if s == nil {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/turn", s.handleTurn)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	s.srv = &http.Server{
		Addr:              s.bind,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      s.timeout + 30*time.Second,
		IdleTimeout:       60 * time.Second,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("local turn listening", "bind", s.bind)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("local turn server failed", "error", err)
		}
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-ctx.Done()
		s.Shutdown()
	}()
}

// Wait blocks until the server goroutines exit.
func (s *Server) Wait() {
	if s != nil {
		s.wg.Wait()
	}
}

// Shutdown stops the listener.
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

type turnRequest struct {
	Text string `json:"text"`
}

type turnResponse struct {
	Text string `json:"text"`
}

func (s *Server) handleTurn(w http.ResponseWriter, r *http.Request) {
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
	var req turnRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		http.Error(w, "text is required", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	reply, err := s.submit(ctx, text)
	if err != nil {
		if errors.Is(err, agent.ErrTurnBusy) {
			http.Error(w, "turn in progress", http.StatusConflict)
			return
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			http.Error(w, "turn timeout", http.StatusGatewayTimeout)
			return
		}
		s.logger.Error("local turn failed", "error", err)
		http.Error(w, "turn failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(turnResponse{Text: reply})
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
