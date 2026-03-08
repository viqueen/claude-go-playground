# Test Agent

Add unit and integration tests for a domain. This PR is auditable as: **"Is this adequately tested?"**

Depends on: `do-integrate` agent PR (full domain must be wired).

## Inputs

The user will specify:
- The **domain name** (e.g., `content`)
- The **project**: `connect-rpc-backend` or `grpc-backend`
- Any specific test scenarios or edge cases to cover

## Project Root

All file paths below are relative to the chosen project folder.
All `make` commands must be run from the project root.

The framework is determined by the project:
- `connect-rpc-backend` → tests use `httptest.NewServer`, Connect client, `connect.NewRequest`, `connect.CodeOf(err)`, `resp.Msg`
- `grpc-backend` → tests use `bufconn`, gRPC client, direct proto messages, `status.Code(err)`, direct response fields

## What to generate

### 1. Testcontainers Setup — `pkg/testkit/containers.go`

If not already present, create a shared testcontainers helper:

```go
package testkit

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// SetupPostgres starts a postgres container and returns the connection string.
// The container is automatically terminated when the test completes.
func SetupPostgres(ctx context.Context, t *testing.T) string {
	t.Helper()
	// Start postgres container with testcontainers
	// Run goose migrations against it
	// Return the connection string
}
```

- Use `testcontainers-go/modules/postgres` for postgres
- Use `testcontainers-go/modules/opensearch` for opensearch if needed
- Run migrations (goose) against the container before returning
- Use `t.Cleanup()` for container teardown

### 2. Domain Tests — `internal/domain/<domain>/`

#### `service_test.go` — Setup only

This file contains **only** the test setup — shared helpers, fixtures, and the service
constructor used by all operation test files. No test functions here.

```go
package content_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	dbcontent "<module>/gen/db/content"
	contentdomain "<module>/internal/domain/content"
	"<module>/pkg/testkit"
	"<module>/pkg/cache"
	"<module>/pkg/outbox"
)

type noopOutbox[T any] struct{}

func (n *noopOutbox[T]) Emit(_ context.Context, _ T, _ ...outbox.Event) error {
	return nil
}

func setupService(t *testing.T) (contentdomain.Service, context.Context) {
	t.Helper()
	ctx := context.Background()
	connStr := testkit.SetupPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	svc := contentdomain.New(contentdomain.Dependencies{
		Pool:    pool,
		Queries: dbcontent.New(pool),
		Cache:   cache.NewInMemory[uuid.UUID, *dbcontent.Content](),
		Outbox:  &noopOutbox[pgx.Tx]{},
	})
	return svc, ctx
}
```

#### `op_<operation>_test.go` — One test file per operation

Each file mirrors the corresponding `op_<operation>.go` and contains a single parent
test function with nested `t.Run()` subtests. Error cases come first, success last.

Domain tests focus on **business logic errors** (not found, already exists, precondition failed).
Input validation (invalid argument) is handled by `buf/validate` interceptors at the API layer —
the domain never sees invalid inputs.

```go
// op_create_test.go
package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dbcontent "<module>/gen/db/content"
)

func TestCreate(t *testing.T) {
	svc, ctx := setupService(t)

	t.Run("already exists — duplicate title", func(t *testing.T) {
		_, err := svc.Create(ctx, dbcontent.CreateContentParams{
			Title:  "unique title",
			Body:   "body",
			Status: 1,
		})
		require.NoError(t, err)

		_, err = svc.Create(ctx, dbcontent.CreateContentParams{
			Title:  "unique title",
			Body:   "other body",
			Status: 1,
		})
		assert.ErrorIs(t, err, contentdomain.ErrAlreadyExists)
	})

	t.Run("success", func(t *testing.T) {
		result, err := svc.Create(ctx, dbcontent.CreateContentParams{
			Title:  "my title",
			Body:   "my body",
			Status: 1,
		})
		require.NoError(t, err)
		assert.Equal(t, "my title", result.Title)
		assert.Equal(t, "my body", result.Body)
		assert.NotEmpty(t, result.ID)
	})
}
```

```go
// op_get_test.go
package content_test

func TestGet(t *testing.T) {
	svc, ctx := setupService(t)

	t.Run("not found", func(t *testing.T) {
		_, err := svc.Get(ctx, uuid.Must(uuid.NewV4()))
		assert.ErrorIs(t, err, contentdomain.ErrNotFound)
	})

	t.Run("success", func(t *testing.T) {
		created, err := svc.Create(ctx, validCreateParams())
		require.NoError(t, err)

		result, err := svc.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, result.ID)
	})
}
```

### 3. API Tests — `internal/api/<domain>/v1/`

#### `handler_test.go` — Setup only

This file contains **only** the test setup — server start, client creation,
service wiring. No test functions here.

There are **two setup functions** to avoid unnecessary testcontainers overhead:

- `setupHandler(t)` — lightweight, no database. For tests that never reach the domain layer
  (unauthenticated, invalid argument, permission denied). Uses a panic service since
  interceptors reject the request before the handler method is called.
- `setupHandlerWithDB(t)` — full setup with testcontainers postgres. For tests that reach
  the domain layer (not found, already exists, success).

Each subtest calls the setup function it needs. This means setup happens inside `t.Run()`
blocks rather than at the parent test level.

The setup returns four clients representing different access levels:

| Client | Access Level | Purpose |
|--------|-------------|---------|
| `anonymous` | No auth | Tests unauthenticated paths |
| `standard` | Authenticated user | Tests normal user operations |
| `admin` | Authenticated admin | Tests admin-only operations |
| `elevated` | System-level | Tests system/service-to-service operations |

Each client injects its access level via request headers (Connect-RPC) or metadata (gRPC).
The auth interceptor reads this to determine the caller's identity and permissions.

```go
// accessLevel represents the four test access levels.
type accessLevel int

const (
	anonymous accessLevel = iota
	standard
	admin
	elevated
)
```

##### `testClients` struct

All four clients are returned in a struct so route tests can pick the right one:

```go
type testClients[T any] struct {
	anonymous T
	standard  T
	admin     T
	elevated  T
}
```

##### If Connect-RPC

```go
package apicontentv1_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	contentv1connect "<module>/gen/sdk/content/v1/contentv1connect"
	apicontentv1 "<module>/internal/api/content/v1"
)

// panicService is a service implementation that panics on every method.
// Used in setupHandler where interceptors reject before reaching the handler.
type panicService struct{}

// startServer creates the httptest server and returns clients for all access levels.
func startServer(t *testing.T, handler contentv1connect.ContentServiceHandler) (*testClients[contentv1connect.ContentServiceClient], context.Context) {
	t.Helper()
	ctx := context.Background()

	path, h := contentv1connect.NewContentServiceHandler(handler, connect.WithInterceptors(interceptors...))

	mux := http.NewServeMux()
	mux.Handle(path, h)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	newClient := func(level accessLevel) contentv1connect.ContentServiceClient {
		return contentv1connect.NewContentServiceClient(
			http.DefaultClient,
			server.URL,
			connect.WithInterceptors(authInterceptor(level)),
		)
	}

	return &testClients[contentv1connect.ContentServiceClient]{
		anonymous: newClient(anonymous),
		standard:  newClient(standard),
		admin:     newClient(admin),
		elevated:  newClient(elevated),
	}, ctx
}

// setupHandler creates the handler without a database backend.
// Use for tests that never reach the domain layer: unauthenticated, invalid argument, permission denied.
func setupHandler(t *testing.T) (*testClients[contentv1connect.ContentServiceClient], context.Context) {
	t.Helper()
	handler := apicontentv1.New(apicontentv1.Dependencies{Service: &panicService{}})
	return startServer(t, handler)
}

// setupHandlerWithDB creates the handler with a real database via testcontainers.
// Use for tests that reach the domain layer: not found, already exists, success.
func setupHandlerWithDB(t *testing.T) (*testClients[contentv1connect.ContentServiceClient], context.Context) {
	t.Helper()
	ctx := context.Background()
	connStr := testkit.SetupPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	svc := contentdomain.New(contentdomain.Dependencies{
		Pool:    pool,
		Queries: dbcontent.New(pool),
		Cache:   cache.NewInMemory[uuid.UUID, *dbcontent.Content](),
		Outbox:  &noopOutbox[pgx.Tx]{},
	})

	handler := apicontentv1.New(apicontentv1.Dependencies{Service: svc})
	return startServer(t, handler)
}

// authInterceptor injects an Authorization header matching the access level.
// anonymous sends no header. standard/admin/elevated send a bearer token
// that the auth interceptor on the server side decodes into the appropriate identity.
func authInterceptor(level accessLevel) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if level != anonymous {
				req.Header().Set("Authorization", "Bearer "+testToken(level))
			}
			return next(ctx, req)
		}
	}
}
```

##### If gRPC

```go
package apicontentv1_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	contentv1grpc "<module>/gen/sdk/content/v1/contentv1grpc"
	apicontentv1 "<module>/internal/api/content/v1"
)

const bufSize = 1024 * 1024

// panicService is a service implementation that panics on every method.
// Used in setupHandler where interceptors reject before reaching the handler.
type panicService struct{}

// startServer creates the bufconn server and returns clients for all access levels.
func startServer(t *testing.T, handler contentv1grpc.ContentServiceServer) (*testClients[contentv1grpc.ContentServiceClient], context.Context) {
	t.Helper()
	ctx := context.Background()

	lis := bufconn.Listen(bufSize)
	server := grpc.NewServer(serverOpts...)
	contentv1grpc.RegisterContentServiceServer(server, handler)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.GracefulStop)

	newClient := func(level accessLevel) contentv1grpc.ContentServiceClient {
		opts := []grpc.DialOption{
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}
		if level != anonymous {
			opts = append(opts, grpc.WithUnaryInterceptor(authClientInterceptor(level)))
		}
		conn, err := grpc.NewClient("passthrough:///bufconn", opts...)
		require.NoError(t, err)
		t.Cleanup(func() { conn.Close() })
		return contentv1grpc.NewContentServiceClient(conn)
	}

	return &testClients[contentv1grpc.ContentServiceClient]{
		anonymous: newClient(anonymous),
		standard:  newClient(standard),
		admin:     newClient(admin),
		elevated:  newClient(elevated),
	}, ctx
}

// setupHandler creates the handler without a database backend.
// Use for tests that never reach the domain layer: unauthenticated, invalid argument, permission denied.
func setupHandler(t *testing.T) (*testClients[contentv1grpc.ContentServiceClient], context.Context) {
	t.Helper()
	handler := apicontentv1.New(apicontentv1.Dependencies{Service: &panicService{}})
	return startServer(t, handler)
}

// setupHandlerWithDB creates the handler with a real database via testcontainers.
// Use for tests that reach the domain layer: not found, already exists, success.
func setupHandlerWithDB(t *testing.T) (*testClients[contentv1grpc.ContentServiceClient], context.Context) {
	t.Helper()
	ctx := context.Background()
	connStr := testkit.SetupPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	svc := contentdomain.New(contentdomain.Dependencies{
		Pool:    pool,
		Queries: dbcontent.New(pool),
		Cache:   cache.NewInMemory[uuid.UUID, *dbcontent.Content](),
		Outbox:  &noopOutbox[pgx.Tx]{},
	})

	handler := apicontentv1.New(apicontentv1.Dependencies{Service: svc})
	return startServer(t, handler)
}

// authClientInterceptor injects authorization metadata matching the access level.
func authClientInterceptor(level accessLevel) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+testToken(level))
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
```

#### `route_<rpc>_test.go` — One test file per RPC

Each file mirrors the corresponding `route_<rpc>.go` and contains **two parent test functions**:

- `Test<Endpoint>_Errors` — uses `setupHandler(t)` at the parent level (no DB). Covers all
  interceptor-level errors: unauthenticated, invalid argument, permission denied. Subtests
  share the setup and run in parallel via `t.Parallel()`.
- `Test<Endpoint>_Success` — uses `setupHandlerWithDB(t)` at the parent level (with DB). Covers
  domain-level errors (not found, already exists) and all success scenarios. Subtests share the
  setup and run in parallel via `t.Parallel()`. There may be multiple success subtests to cover
  different aspects of the API contract.

Each test picks the appropriate client for the access level being tested:
- `clients.anonymous` for unauthenticated tests
- `clients.standard` for normal user tests
- `clients.admin` for admin-only operation tests
- `clients.elevated` for system-level tests

##### If Connect-RPC

```go
// route_create_content_test.go
package apicontentv1_test

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contentv1 "<module>/gen/sdk/content/v1"
)

func TestCreateContent_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("unauthenticated — no token", func(t *testing.T) {
		t.Parallel()
		_, err := clients.anonymous.CreateContent(ctx, connect.NewRequest(validCreateRequest()))
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("invalid argument — empty title", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateContent(ctx, connect.NewRequest(&contentv1.CreateContentRequest{
			Title:  "",
			Body:   "some body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})
}

func TestCreateContent_Success(t *testing.T) {
	clients, ctx := setupHandlerWithDB(t)

	t.Run("creates with required fields", func(t *testing.T) {
		t.Parallel()
		resp, err := clients.standard.CreateContent(ctx, connect.NewRequest(validCreateRequest()))
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Msg.Content.Id)
		assert.Equal(t, "my title", resp.Msg.Content.Title)
	})

	t.Run("creates with optional tags", func(t *testing.T) {
		t.Parallel()
		resp, err := clients.standard.CreateContent(ctx, connect.NewRequest(&contentv1.CreateContentRequest{
			Title:  "tagged content",
			Body:   "body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			Tags:   []string{"go", "grpc"},
		}))
		require.NoError(t, err)
		assert.Equal(t, []string{"go", "grpc"}, resp.Msg.Content.Tags)
	})
}
```

```go
// route_delete_content_test.go — example of admin-only operation
package apicontentv1_test

func TestDeleteContent_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("unauthenticated — no token", func(t *testing.T) {
		t.Parallel()
		_, err := clients.anonymous.DeleteContent(ctx, connect.NewRequest(&contentv1.DeleteContentRequest{
			Id: uuid.Must(uuid.NewV4()).String(),
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})

	t.Run("permission denied — standard user", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.DeleteContent(ctx, connect.NewRequest(&contentv1.DeleteContentRequest{
			Id: uuid.Must(uuid.NewV4()).String(),
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	})
}

func TestDeleteContent_Success(t *testing.T) {
	clients, ctx := setupHandlerWithDB(t)

	t.Run("not found — nonexistent ID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.admin.DeleteContent(ctx, connect.NewRequest(&contentv1.DeleteContentRequest{
			Id: uuid.Must(uuid.NewV4()).String(),
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})

	t.Run("deletes existing resource", func(t *testing.T) {
		t.Parallel()
		created, err := clients.standard.CreateContent(ctx, connect.NewRequest(validCreateRequest()))
		require.NoError(t, err)

		resp, err := clients.admin.DeleteContent(ctx, connect.NewRequest(&contentv1.DeleteContentRequest{
			Id: created.Msg.Content.Id,
		}))
		require.NoError(t, err)
		assert.True(t, resp.Msg.Success)
	})
}
```

##### If gRPC

```go
// route_create_content_test.go
package apicontentv1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	contentv1 "<module>/gen/sdk/content/v1"
)

func TestCreateContent_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("unauthenticated — no token", func(t *testing.T) {
		t.Parallel()
		_, err := clients.anonymous.CreateContent(ctx, validCreateRequest())
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("invalid argument — empty title", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateContent(ctx, &contentv1.CreateContentRequest{
			Title:  "",
			Body:   "some body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestCreateContent_Success(t *testing.T) {
	clients, ctx := setupHandlerWithDB(t)

	t.Run("creates with required fields", func(t *testing.T) {
		t.Parallel()
		resp, err := clients.standard.CreateContent(ctx, validCreateRequest())
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Content.Id)
		assert.Equal(t, "my title", resp.Content.Title)
	})

	t.Run("creates with optional tags", func(t *testing.T) {
		t.Parallel()
		resp, err := clients.standard.CreateContent(ctx, &contentv1.CreateContentRequest{
			Title:  "tagged content",
			Body:   "body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			Tags:   []string{"go", "grpc"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"go", "grpc"}, resp.Content.Tags)
	})
}
```

```go
// route_delete_content_test.go — example of admin-only operation
package apicontentv1_test

func TestDeleteContent_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("unauthenticated — no token", func(t *testing.T) {
		t.Parallel()
		_, err := clients.anonymous.DeleteContent(ctx, &contentv1.DeleteContentRequest{
			Id: uuid.Must(uuid.NewV4()).String(),
		})
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("permission denied — standard user", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.DeleteContent(ctx, &contentv1.DeleteContentRequest{
			Id: uuid.Must(uuid.NewV4()).String(),
		})
		require.Error(t, err)
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
}

func TestDeleteContent_Success(t *testing.T) {
	clients, ctx := setupHandlerWithDB(t)

	t.Run("not found — nonexistent ID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.admin.DeleteContent(ctx, &contentv1.DeleteContentRequest{
			Id: uuid.Must(uuid.NewV4()).String(),
		})
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("deletes existing resource", func(t *testing.T) {
		t.Parallel()
		created, err := clients.standard.CreateContent(ctx, validCreateRequest())
		require.NoError(t, err)

		resp, err := clients.admin.DeleteContent(ctx, &contentv1.DeleteContentRequest{
			Id: created.Content.Id,
		})
		require.NoError(t, err)
		assert.True(t, resp.Success)
	})
}
```

Key differences: gRPC tests pass proto messages directly (no `connect.NewRequest` wrapper),
access response fields directly (no `.Msg`), and check errors via `status.Code(err)` instead
of `connect.CodeOf(err)`.

### 4. Outbox Worker Tests — `internal/outbox/<domain>/event_<concern>_test.go`

Test that workers process jobs correctly:

- Verify job args are constructed correctly from events
- Test worker execution (may require testcontainers for opensearch if indexing)

## Test Patterns

### Assertions: `assert` vs `require`

- Use `require` when a failure would cause a panic in subsequent lines (e.g., nil pointer).
  `require` stops the test immediately on failure.
- Use `assert` for all other checks. `assert` records the failure but continues the test.
- Rule of thumb: `require.NoError` after create/setup calls, `assert` for the actual assertions.

```go
// require: without this the next line panics on nil resp
resp, err := client.GetContent(ctx, req)
require.NoError(t, err)

// assert: safe to continue even if this fails
assert.Equal(t, "expected", resp.Msg.Content.Title)
```

### No table-driven tests

Do NOT use `[]struct{ name string; ... }` table-driven patterns. Instead, use a parent
test function with explicit `t.Run()` blocks. Each subtest should be readable on its own.

### Test case ordering

**API layer** — each route file has two parent tests:

`Test<Endpoint>_Errors` (interceptor-level, no DB):
unauthenticated (anonymous) → invalid argument (standard) → permission denied (standard on admin-only ops)

`Test<Endpoint>_Success` (domain-level, with DB):
not found → already exists → success cases (one or more, covering the API contract)

Subtests within each parent run in parallel via `t.Parallel()`.

**Domain layer** (tested in `op_*_test.go`):
not found → already exists → precondition failed → success

The domain never sees unauthenticated, invalid argument, or permission denied —
those are caught by interceptors at the API layer before reaching the domain.

### Access level testing

Each RPC should be tested with the relevant access levels:
- **All RPCs**: test unauthenticated via `clients.anonymous` → expect `Unauthenticated`
- **Standard operations** (create, get, list, update): test success via `clients.standard`
- **Admin-only operations** (delete, bulk operations): test `clients.standard` → expect `PermissionDenied`, test success via `clients.admin`
- **Elevated operations** (system triggers, internal-only RPCs): test `clients.admin` → expect `PermissionDenied`, test success via `clients.elevated`

The access level required for each RPC is domain-specific — the user will specify which operations require which levels.

### Test naming

Parent test functions: `Test<Endpoint>_Errors` and `Test<Endpoint>_Success`.

Subtest names in `_Errors`: `<error code> — <description>`
Subtest names in `_Success`: `<description>` (descriptive of the scenario being tested)

```go
// _Errors subtests
t.Run("unauthenticated — no token", func(t *testing.T) { ... })
t.Run("invalid argument — empty title", func(t *testing.T) { ... })
t.Run("permission denied — standard user", func(t *testing.T) { ... })

// _Success subtests (domain errors + success scenarios)
t.Run("not found — nonexistent ID", func(t *testing.T) { ... })
t.Run("already exists — duplicate name", func(t *testing.T) { ... })
t.Run("creates with required fields", func(t *testing.T) { ... })
t.Run("creates with optional tags", func(t *testing.T) { ... })
```

### File structure: setup file + one test file per operation/route

- `service_test.go` and `handler_test.go` contain **only** setup (no `Test*` functions)
- One `op_<operation>_test.go` per domain operation
- One `route_<rpc>_test.go` per API route, containing two parent tests: `Test<Endpoint>_Errors` + `Test<Endpoint>_Success`

### No mocks for infrastructure

Use testcontainers for real postgres and opensearch. Only use a no-op implementation
for the outbox in domain tests (to isolate from river).

### Test isolation

Domain tests: each parent test calls `setupService(t)` at the parent level.
API tests: each parent test calls its setup at the parent level — `_Errors` calls `setupHandler(t)`,
`_Success` calls `setupHandlerWithDB(t)`. Subtests within a parent share the setup and run in
parallel via `t.Parallel()`. No shared state between parent tests.

## Post-Generation

1. Run `make test` — all tests should pass
2. Verify testcontainers start and stop cleanly (no leaked containers)

## Checklist

- [ ] `pkg/testkit/containers.go` with testcontainers helpers (if not present)
- [ ] `service_test.go` contains only setup — no `Test*` functions
- [ ] One `op_<operation>_test.go` per domain operation
- [ ] `handler_test.go` contains only setup — no `Test*` functions
- [ ] `handler_test.go` defines `accessLevel` enum: `anonymous`, `standard`, `admin`, `elevated`
- [ ] `handler_test.go` defines `testClients[T]` struct with all four access levels
- [ ] `handler_test.go` defines `panicService` struct for interceptor-only tests
- [ ] `setupHandler(t)` — no database, uses panic service (for unauthenticated/invalid arg/permission denied)
- [ ] `setupHandlerWithDB(t)` — testcontainers postgres, real service (for not found/already exists/success)
- [ ] `startServer(t, handler)` — shared helper that creates server + clients (called by both setup functions)
- [ ] Connect-RPC: setup uses `httptest.NewServer` + per-level Connect clients with auth interceptor
- [ ] gRPC: setup uses `bufconn` + per-level gRPC clients with auth metadata interceptor
- [ ] One `route_<rpc>_test.go` per API route
- [ ] Each route file has two parent tests: `Test<Endpoint>_Errors` + `Test<Endpoint>_Success`
- [ ] `_Errors` calls `setupHandler(t)` at parent level (no DB, interceptor-level errors)
- [ ] `_Success` calls `setupHandlerWithDB(t)` at parent level (with DB, domain errors + success)
- [ ] Subtests within each parent call `t.Parallel()`
- [ ] Route tests use `clients.anonymous` for unauthenticated, `clients.standard`/`admin`/`elevated` for auth tests
- [ ] Every RPC tests unauthenticated path via `clients.anonymous` in `_Errors`
- [ ] Admin-only RPCs test permission denied via `clients.standard` in `_Errors`
- [ ] Domain errors (not found, already exists) go in `_Success` (they need the DB)
- [ ] Multiple success subtests cover different aspects of the API contract
- [ ] Connect-RPC: tests use `connect.NewRequest`, `connect.CodeOf(err)`, `resp.Msg`
- [ ] gRPC: tests pass proto directly, use `status.Code(err)`, access response fields directly
- [ ] `_Errors` subtest names: `<error code> — <description>`
- [ ] `_Success` subtest names: descriptive of the scenario
- [ ] `require` used for setup/create calls that would panic on failure
- [ ] `assert` used for all other assertions
- [ ] No table-driven tests
- [ ] No mocks for databases — testcontainers only
- [ ] No-op outbox used in domain tests
- [ ] `t.Cleanup()` used for all resource teardown
- [ ] Outbox tests verify job args construction
- [ ] `make test` passes
