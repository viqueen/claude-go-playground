package grpcutil

import (
	"context"
	"fmt"
	"time"

	"buf.build/go/protovalidate"
	protovalidate_middleware "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/protovalidate"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NewServerOpts() []grpc.ServerOption {
	validator, err := protovalidate.New()
	if err != nil {
		panic(fmt.Sprintf("failed to create protovalidate validator: %v", err))
	}
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			RecoveryInterceptor(),
			LoggingInterceptor(),
			protovalidate_middleware.UnaryServerInterceptor(validator),
		),
	}
}

func RecoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = status.Errorf(codes.Internal, "panic: %v", r)
			}
		}()
		return handler(ctx, req)
	}
}

func LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		evt := log.Info()
		if err != nil {
			evt = log.Error().Err(err)
		}
		evt.
			Str("method", info.FullMethod).
			Dur("duration", time.Since(start)).
			Msg("rpc")
		return resp, err
	}
}
