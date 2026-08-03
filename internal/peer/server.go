package peer

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

// Server is a tiny loopback HTTP listener that accepts peer POSTs into
// an Inbox. Separate from the admin UI.
type Server struct {
	bind   string
	token  string
	inbox  *Inbox
	logger *slog.Logger

	srv      *http.Server
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewServer constructs a peer inbox HTTP server. Call Start to listen.
func NewServer(bind, token string, inbox *Inbox, logger *slog.Logger) (*Server, error) {
	if bind == "" {
		return nil, fmt.Errorf("peers.listen is required")
	}
	if token == "" {
		return nil, fmt.Errorf("peers.token is required")
	}
	if inbox == nil {
		return nil, fmt.Errorf("inbox is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{bind: bind, token: token, inbox: inbox, logger: logger}, nil
}

// Start listens until ctx is canceled.
func (s *Server) Start(ctx context.Context) {
	if s == nil {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /inbox", s.handleInbox)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	s.srv = &http.Server{
		Addr:              s.bind,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("peer inbox listening", "bind", s.bind)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("peer inbox server failed", "error", err)
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
	var req sendBody
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	msg, err := s.inbox.Enqueue(req.From, req.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(sendResponse{ID: msg.ID})
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
