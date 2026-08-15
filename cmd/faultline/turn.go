package main

import (
	"fmt"
	"log/slog"

	turnhttp "github.com/CamiloValderruten/faultline/internal/adapters/turn"
	"github.com/CamiloValderruten/faultline/internal/agent"
	"github.com/CamiloValderruten/faultline/internal/config"
)

func buildTurnServer(cfg config.TurnConfig, a *agent.Agent, logger *slog.Logger) (*turnhttp.Server, error) {
	if !cfg.Active() {
		logger.Info("local turn HTTP disabled")
		return nil, nil
	}
	srv, err := turnhttp.NewServer(cfg.Bind, cfg.Token, cfg.Timeout.Duration(), a.SubmitTurn, logger)
	if err != nil {
		return nil, fmt.Errorf("turn server: %w", err)
	}
	return srv, nil
}
