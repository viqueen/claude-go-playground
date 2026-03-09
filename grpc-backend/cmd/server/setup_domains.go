package main

import (
	uuid "github.com/gofrs/uuid/v5"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
	riveroutbox "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/cache"
)

// Domains holds all domain services.
type Domains struct {
	SpaceService spacedomain.Service
}

func setupDomains(connections *Connections) *Domains {
	queries := db.New(connections.Pool)
	outbox := riveroutbox.NewRiverOutbox(connections.RiverClient)

	spaceService := spacedomain.New(spacedomain.Dependencies{
		Pool:    connections.Pool,
		Queries: queries,
		Cache:   cache.NewInMemory[uuid.UUID, *db.CollaborationSpace](),
		Outbox:  outbox,
	})

	return &Domains{
		SpaceService: spaceService,
	}
}
