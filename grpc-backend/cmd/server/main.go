package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	connections := setupConnections(ctx, cfg)
	defer connections.Close(ctx)

	domains := setupDomains(connections)
	application := setupGateway(cfg, domains)

	if err := application.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}
