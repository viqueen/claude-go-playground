package grpcutil

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewErrorFrom(err error, mappings map[error]codes.Code) error {
	for sentinel, code := range mappings {
		if errors.Is(err, sentinel) {
			return status.Error(code, err.Error())
		}
	}
	return status.Error(codes.Internal, err.Error())
}
