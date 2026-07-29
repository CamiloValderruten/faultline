package main

import (
	"fmt"
	"log/slog"

	"github.com/CamiloValderruten/faultline/internal/config"
	"github.com/CamiloValderruten/faultline/internal/peer"
)

func buildPeerMailbox(cfg config.PeersConfig, logger *slog.Logger) (*peer.Mailbox, *peer.Server, error) {
	if !cfg.Active() {
		logger.Info("peer messaging disabled, peer_* tools not advertised")
		return nil, nil, nil
	}

	inbox := peer.NewInbox(cfg.InboxFile, cfg.MaxInbox)
	agents := make([]peer.Agent, 0, len(cfg.Agents))
	for _, a := range cfg.Agents {
		agents = append(agents, peer.Agent{
			Name:  a.Name,
			URL:   a.URL,
			Token: a.Token,
		})
	}
	mailbox := peer.NewMailbox(cfg.Name, inbox, agents)

	srv, err := peer.NewServer(cfg.Listen, cfg.Token, inbox, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("peer server: %w", err)
	}
	return mailbox, srv, nil
}
