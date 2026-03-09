package main

import (
	"github.com/rs/zerolog/log"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/config"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcapp"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func setupGateway(cfg *config.Config, _ *Domains) grpcapp.App {
	serverOpts, err := grpcutil.NewServerOpts()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create server options")
	}

	application := grpcapp.New(
		grpcapp.WithAddr(cfg.ServerAddr),
		grpcapp.WithServerOpts(serverOpts...),
	)

	// Services are registered here by the integrate agent.
	// Each domain registers its gRPC service on application.Server().

	return application
}
