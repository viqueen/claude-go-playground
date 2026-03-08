package main

import (
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/config"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcapp"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func setupGateway(cfg *config.Config, _ *Domains) grpcapp.App {
	application := grpcapp.New(
		grpcapp.WithAddr(cfg.ServerAddr),
		grpcapp.WithServerOpts(grpcutil.NewServerOpts()...),
	)

	// Services are registered here by the integrate agent.
	// Each domain registers its gRPC service on application.Server().

	return application
}
