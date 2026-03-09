package apispacev1

import (
	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"

	"google.golang.org/grpc/codes"
)

// Dependencies defines the dependencies for the space API handler.
type Dependencies struct {
	Service spacedomain.Service
}

// New returns the gRPC-generated service server. Struct is private.
func New(deps Dependencies) spacev1.SpaceServiceServer {
	return &handler{service: deps.Service}
}

type handler struct {
	spacev1.UnimplementedSpaceServiceServer
	service spacedomain.Service
}

var errorMappings = map[error]codes.Code{
	spacedomain.ErrNotFound:      codes.NotFound,
	spacedomain.ErrAlreadyExists: codes.AlreadyExists,
}
