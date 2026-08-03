package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/CamiloValderruten/faultline/internal/adapters/publish"
	"github.com/CamiloValderruten/faultline/internal/config"
)

// publishServer wraps the HTML publishing harness listener. Nil-safe
// on Start/Wait/Shutdown/Close so main can treat a disabled feature
// the same way as oauth/admin.
type publishServer struct {
	cfg    config.PublishConfig
	pub    *publish.Server
	logger *slog.Logger

	srv      *http.Server
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// buildPublish constructs the publish HTTP server. When disabled,
// returns (nil, nil). Root defaults to <sandbox.dir>/output/html and
// is created if missing so the agent can write there before the first
// request.
func buildPublish(cfg config.PublishConfig, sandboxDir string, logger *slog.Logger) (*publishServer, error) {
	if !cfg.Active() {
		logger.Info("publish server disabled")
		return nil, nil
	}

	root := cfg.Root
	if root == "" {
		if sandboxDir == "" {
			sandboxDir = "./sandbox"
		}
		root = filepath.Join(sandboxDir, "output", "html")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("publish: create root %q: %w", root, err)
	}

	pub, err := publish.New(root, cfg.MDTemplate)
	if err != nil {
		return nil, err
	}

	logger.Info("publish server configured",
		"bind", cfg.Bind,
		"root", pub.Root(),
		"public_base_url", cfg.PublicBaseURL)

	return &publishServer{cfg: cfg, pub: pub, logger: logger}, nil
}

func (s *publishServer) Start(ctx context.Context) {
	if s == nil {
		return
	}
	s.srv = &http.Server{
		Addr:              s.cfg.Bind,
		Handler:           s.pub.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Info("publish server listening",
			"bind", s.cfg.Bind,
			"root", s.pub.Root(),
			"public_base_url", s.cfg.PublicBaseURL)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("publish server failed", "error", err)
		}
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-ctx.Done()
		s.shutdown()
	}()
}

func (s *publishServer) Wait() {
	if s != nil {
		s.wg.Wait()
	}
}

func (s *publishServer) Shutdown() {
	if s != nil {
		s.shutdown()
	}
}

func (s *publishServer) Close() {
	if s == nil {
		return
	}
	s.shutdown()
	if s.pub != nil {
		_ = s.pub.Close()
	}
}

func (s *publishServer) shutdown() {
	s.stopOnce.Do(func() {
		if s.srv == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.srv.Shutdown(ctx); err != nil {
			s.logger.Warn("publish server shutdown", "error", err)
		}
	})
}
