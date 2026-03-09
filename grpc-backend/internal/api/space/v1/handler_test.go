package apispacev1_test

import (
	"context"
	"net"
	"testing"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	apispacev1 "github.com/viqueen/claude-go-playground/grpc-backend/internal/api/space/v1"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/cache"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/testkit"
)

const bufSize = 1024 * 1024

// accessLevel represents the four test access levels.
type accessLevel int

const (
	anonymous accessLevel = iota
	standard
	admin
	elevated
)

// testToken returns a deterministic bearer token for the given access level.
func testToken(level accessLevel) string {
	switch level {
	case standard:
		return "test-standard-token"
	case admin:
		return "test-admin-token"
	case elevated:
		return "test-elevated-token"
	default:
		return ""
	}
}

type testClients[T any] struct {
	anonymous T
	standard  T
	admin     T
	elevated  T
}

// panicService is a service implementation that panics on every method.
// Used in setupHandler where interceptors reject before reaching the handler.
type panicService struct{}

func (p *panicService) Create(_ context.Context, _ db.CreateSpaceParams) (*db.CollaborationSpace, error) {
	panic("unexpected call to Create")
}

func (p *panicService) Get(_ context.Context, _ uuid.UUID) (*db.CollaborationSpace, error) {
	panic("unexpected call to Get")
}

func (p *panicService) List(_ context.Context, _ int32, _ string) ([]db.CollaborationSpace, string, error) {
	panic("unexpected call to List")
}

func (p *panicService) Update(_ context.Context, _ db.UpdateSpaceParams) (*db.CollaborationSpace, error) {
	panic("unexpected call to Update")
}

func (p *panicService) Delete(_ context.Context, _ uuid.UUID) error {
	panic("unexpected call to Delete")
}

type noopOutbox[T any] struct{}

func (n *noopOutbox[T]) Emit(_ context.Context, _ T, _ ...outbox.Event) error {
	return nil
}

// startServer creates the bufconn server and returns clients for all access levels.
func startServer(t *testing.T, handler spacev1.SpaceServiceServer) (*testClients[spacev1.SpaceServiceClient], context.Context) {
	t.Helper()
	ctx := context.Background()

	lis := bufconn.Listen(bufSize)
	opts, err := grpcutil.NewServerOpts()
	require.NoError(t, err)
	server := grpc.NewServer(opts...)
	spacev1.RegisterSpaceServiceServer(server, handler)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.GracefulStop)

	newClient := func(level accessLevel) spacev1.SpaceServiceClient {
		dialOpts := []grpc.DialOption{
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}
		if level != anonymous {
			dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(authClientInterceptor(level)))
		}
		conn, err := grpc.NewClient("passthrough:///bufconn", dialOpts...)
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close() })
		return spacev1.NewSpaceServiceClient(conn)
	}

	return &testClients[spacev1.SpaceServiceClient]{
		anonymous: newClient(anonymous),
		standard:  newClient(standard),
		admin:     newClient(admin),
		elevated:  newClient(elevated),
	}, ctx
}

// setupHandler creates the handler without a database backend.
// Use for tests that never reach the domain layer: invalid argument.
func setupHandler(t *testing.T) (*testClients[spacev1.SpaceServiceClient], context.Context) {
	t.Helper()
	handler := apispacev1.New(apispacev1.Dependencies{Service: &panicService{}})
	return startServer(t, handler)
}

// setupHandlerWithDB creates the handler with a real database via testcontainers.
// Use for tests that reach the domain layer: not found, already exists, success.
func setupHandlerWithDB(t *testing.T) (*testClients[spacev1.SpaceServiceClient], context.Context) {
	t.Helper()
	ctx := context.Background()
	connStr := testkit.SetupPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	svc := spacedomain.New(spacedomain.Dependencies{
		Pool:    pool,
		Queries: db.New(pool),
		Cache:   cache.NewInMemory[uuid.UUID, *db.CollaborationSpace](),
		Outbox:  &noopOutbox[pgx.Tx]{},
	})

	handler := apispacev1.New(apispacev1.Dependencies{Service: svc})
	return startServer(t, handler)
}

// authClientInterceptor injects authorization metadata matching the access level.
func authClientInterceptor(level accessLevel) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+testToken(level))
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func validCreateRequest() *spacev1.CreateSpaceRequest {
	return &spacev1.CreateSpaceRequest{
		Name:        "My Space",
		Key:         "MYSPACE",
		Description: "A test space",
		Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
	}
}
