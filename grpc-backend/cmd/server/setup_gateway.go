package main

import (
	"github.com/rs/zerolog/log"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	apicontentv1 "github.com/viqueen/claude-go-playground/grpc-backend/internal/api/content/v1"
	apispacev1 "github.com/viqueen/claude-go-playground/grpc-backend/internal/api/space/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/config"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcapp"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func setupGateway(cfg *config.Config, domains *Domains) grpcapp.App {
	serverOpts, err := grpcutil.NewServerOpts()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create server options")
	}

	application := grpcapp.New(
		grpcapp.WithAddr(cfg.ServerAddr),
		grpcapp.WithServerOpts(serverOpts...),
	)

	// Register space service
	spaceHandler := apispacev1.New(apispacev1.Dependencies{
		Service: domains.SpaceService,
	})
	spacev1.RegisterSpaceServiceServer(application.Server(), spaceHandler)

	// Register content service
	contentHandler := apicontentv1.New(apicontentv1.Dependencies{
		Service: domains.ContentService,
	})
	contentv1.RegisterContentServiceServer(application.Server(), contentHandler)

	return application
}
