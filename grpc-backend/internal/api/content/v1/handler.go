package apicontentv1

import (
	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	contentdomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/content"

	"google.golang.org/grpc/codes"
)

// Dependencies defines the dependencies for the content API handler.
type Dependencies struct {
	Service contentdomain.Service
}

// New returns the gRPC-generated service server. Struct is private.
func New(deps Dependencies) contentv1.ContentServiceServer {
	return &handler{service: deps.Service}
}

type handler struct {
	contentv1.UnimplementedContentServiceServer
	service contentdomain.Service
}

var errorMappings = map[error]codes.Code{
	contentdomain.ErrNotFound: codes.NotFound,
}
