package grpcapp

import (
	"context"
	"net"
	"net/http"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

// App is the public interface for the gRPC application.
type App interface {
	Server() *grpc.Server
	Run(ctx context.Context) error
}

type Option func(*app)

func WithAddr(addr string) Option         { return func(a *app) { a.addr = addr } }
func WithHealthAddr(addr string) Option   { return func(a *app) { a.healthAddr = addr } }
func WithServerOpts(opts ...grpc.ServerOption) Option {
	return func(a *app) { a.serverOpts = append(a.serverOpts, opts...) }
}

func New(opts ...Option) App {
	a := &app{addr: ":8080", healthAddr: ":8081"}
	for _, o := range opts {
		o(a)
	}
	a.server = grpc.NewServer(a.serverOpts...)

	// gRPC health check
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(a.server, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	// gRPC reflection
	reflection.Register(a.server)

	return a
}

type app struct {
	addr       string
	healthAddr string
	serverOpts []grpc.ServerOption
	server     *grpc.Server
}

func (a *app) Server() *grpc.Server {
	return a.server
}

func (a *app) Run(ctx context.Context) error {
	lis, err := net.Listen("tcp", a.addr)
	if err != nil {
		return err
	}

	// HTTP health sidecar for docker-compose/k8s probes
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"up"}`))
	})
	healthServer := &http.Server{Addr: a.healthAddr, Handler: healthMux}
	go func() { _ = healthServer.ListenAndServe() }()

	log.Info().Str("addr", a.addr).Str("health_addr", a.healthAddr).Msg("server started")

	errCh := make(chan error, 1)
	go func() { errCh <- a.server.Serve(lis) }()

	select {
	case <-ctx.Done():
		a.server.GracefulStop()
		return healthServer.Close()
	case err := <-errCh:
		_ = healthServer.Close()
		return err
	}
}
