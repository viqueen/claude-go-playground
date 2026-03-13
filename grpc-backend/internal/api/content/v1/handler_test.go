package apicontentv1_test

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
	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	apicontentv1 "github.com/viqueen/claude-go-playground/grpc-backend/internal/api/content/v1"
	contentdomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/content"
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

func (p *panicService) Create(_ context.Context, _ db.CreateContentParams) (*db.CollaborationContent, error) {
	panic("unexpected call to Create")
}

func (p *panicService) Get(_ context.Context, _ uuid.UUID) (*db.CollaborationContent, error) {
	panic("unexpected call to Get")
}

func (p *panicService) List(_ context.Context, _ int32, _ string) ([]db.CollaborationContent, string, error) {
	panic("unexpected call to List")
}

func (p *panicService) ListBySpace(_ context.Context, _ uuid.UUID, _ int32, _ string) ([]db.CollaborationContent, string, error) {
	panic("unexpected call to ListBySpace")
}

func (p *panicService) Update(_ context.Context, _ db.UpdateContentParams) (*db.CollaborationContent, error) {
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
func startServer(t *testing.T, handler contentv1.ContentServiceServer) (*testClients[contentv1.ContentServiceClient], context.Context) {
	t.Helper()
	ctx := context.Background()

	lis := bufconn.Listen(bufSize)
	opts, err := grpcutil.NewServerOpts()
	require.NoError(t, err)
	server := grpc.NewServer(opts...)
	contentv1.RegisterContentServiceServer(server, handler)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.GracefulStop)

	newClient := func(level accessLevel) contentv1.ContentServiceClient {
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
		return contentv1.NewContentServiceClient(conn)
	}

	return &testClients[contentv1.ContentServiceClient]{
		anonymous: newClient(anonymous),
		standard:  newClient(standard),
		admin:     newClient(admin),
		elevated:  newClient(elevated),
	}, ctx
}

// setupHandler creates the handler without a database backend.
// Use for tests that never reach the domain layer: invalid argument.
func setupHandler(t *testing.T) (*testClients[contentv1.ContentServiceClient], context.Context) {
	t.Helper()
	handler := apicontentv1.New(apicontentv1.Dependencies{Service: &panicService{}})
	return startServer(t, handler)
}

// testContext holds the clients and a helper to create spaces for content tests.
type testContext struct {
	clients      *testClients[contentv1.ContentServiceClient]
	ctx          context.Context
	spaceDomain  spacedomain.Service
}

// createSpace creates a space in the database and returns its UUID string.
// Content items require a valid space_id foreign key.
func (tc *testContext) createSpace(t *testing.T) string {
	t.Helper()
	space, err := tc.spaceDomain.Create(tc.ctx, db.CreateSpaceParams{
		Name:       "Test Space " + uuid.Must(uuid.NewV4()).String()[:8],
		Key:        "TS" + uuid.Must(uuid.NewV4()).String()[:6],
		Description: "test space for content tests",
		Visibility: int32(spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE),
	})
	require.NoError(t, err)
	return space.ID.String()
}

// setupHandlerWithDB creates the handler with a real database via testcontainers.
// Use for tests that reach the domain layer: not found, already exists, success.
func setupHandlerWithDB(t *testing.T) *testContext {
	t.Helper()
	ctx := context.Background()
	connStr := testkit.SetupPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	queries := db.New(pool)

	spaceSvc := spacedomain.New(spacedomain.Dependencies{
		Pool:    pool,
		Queries: queries,
		Cache:   cache.NewInMemory[uuid.UUID, *db.CollaborationSpace](),
		Outbox:  &noopOutbox[pgx.Tx]{},
	})

	contentSvc := contentdomain.New(contentdomain.Dependencies{
		Pool:    pool,
		Queries: queries,
		Cache:   cache.NewInMemory[uuid.UUID, *db.CollaborationContent](),
		Outbox:  &noopOutbox[pgx.Tx]{},
	})

	handler := apicontentv1.New(apicontentv1.Dependencies{Service: contentSvc})
	clients, ctx := startServer(t, handler)

	return &testContext{
		clients:     clients,
		ctx:         ctx,
		spaceDomain: spaceSvc,
	}
}

// authClientInterceptor injects authorization metadata matching the access level.
func authClientInterceptor(level accessLevel) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+testToken(level))
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func validCreateRequest(spaceID string) *contentv1.CreateContentRequest {
	return &contentv1.CreateContentRequest{
		Space:  &spacev1.SpaceRef{Id: spaceID},
		Title:  "My Content",
		Body:   "Some body text",
		Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		Tags:   []string{},
	}
}
