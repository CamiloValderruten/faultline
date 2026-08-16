package main

import (
	"fmt"
	"log/slog"

	"github.com/CamiloValderruten/faultline/internal/adapters/webhook"
	"github.com/CamiloValderruten/faultline/internal/config"
)

func buildWebhook(cfg config.WebhookConfig, push webhook.PushFunc, logger *slog.Logger) (*webhook.Server, error) {
	if !cfg.Active() {
		logger.Info("inbox webhook disabled")
		return nil, nil
	}
	srv, err := webhook.NewServer(cfg.Bind, cfg.Token, push, logger)
	if err != nil {
		return nil, fmt.Errorf("webhook server: %w", err)
	}
	return srv, nil
}
