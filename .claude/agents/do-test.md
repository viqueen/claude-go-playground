# Test Agent

Add unit and integration tests for a domain. This PR is auditable as: **"Is this adequately tested?"**

Depends on: `do-integrate` agent PR (full domain must be wired).

## Inputs

The user will specify:
- The **domain name** (e.g., `content`)
- Any specific test scenarios or edge cases to cover

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

func setupHandler(t *testing.T) (contentv1connect.ContentServiceClient, context.Context) {
	t.Helper()
	ctx := context.Background()

	// Wire real service with testcontainers postgres
	// ...

	handler := apicontentv1.New(apicontentv1.Dependencies{Service: svc})
	path, h := contentv1connect.NewContentServiceHandler(handler)

	mux := http.NewServeMux()
	mux.Handle(path, h)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := contentv1connect.NewContentServiceClient(
		http.DefaultClient,
		server.URL,
	)
	return client, ctx
}
```

#### `route_<rpc>_test.go` — One test file per RPC

Each file mirrors the corresponding `route_<rpc>.go` and contains a single parent
test function with nested `t.Run()` subtests. Error cases come first, success last.

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

func TestCreateContent(t *testing.T) {
	client, ctx := setupHandler(t)

	t.Run("invalid argument — empty title", func(t *testing.T) {
		_, err := client.CreateContent(ctx, connect.NewRequest(&contentv1.CreateContentRequest{
			Title: "",
			Body:  "some body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("success", func(t *testing.T) {
		resp, err := client.CreateContent(ctx, connect.NewRequest(&contentv1.CreateContentRequest{
			Title:  "my title",
			Body:   "my body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		}))
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Msg.Content.Id)
		assert.Equal(t, "my title", resp.Msg.Content.Title)
	})
}
```

```go
// route_get_content_test.go
package apicontentv1_test

func TestGetContent(t *testing.T) {
	client, ctx := setupHandler(t)

	t.Run("not found", func(t *testing.T) {
		_, err := client.GetContent(ctx, connect.NewRequest(&contentv1.GetContentRequest{
			Id: uuid.Must(uuid.NewV4()).String(),
		}))
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})

	t.Run("success", func(t *testing.T) {
		created, err := client.CreateContent(ctx, connect.NewRequest(validCreateRequest()))
		require.NoError(t, err)

		resp, err := client.GetContent(ctx, connect.NewRequest(&contentv1.GetContentRequest{
			Id: created.Msg.Content.Id,
		}))
		require.NoError(t, err)
		assert.Equal(t, created.Msg.Content.Id, resp.Msg.Content.Id)
	})
}
```

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

Within each parent test, order subtests as:
1. Error cases first
2. Success case last

**API layer errors** (tested in `route_*_test.go`):
unauthenticated → invalid argument → permission denied → not found → already exists → success

**Domain layer errors** (tested in `op_*_test.go`):
not found → already exists → precondition failed → success

The domain never sees unauthenticated, invalid argument, or permission denied —
those are caught by interceptors at the API layer before reaching the domain.

### Test naming

Subtest names follow the pattern: `<error code or outcome> — <description>`

```go
t.Run("not found — nonexistent ID", func(t *testing.T) { ... })
t.Run("invalid argument — empty title", func(t *testing.T) { ... })
t.Run("already exists — duplicate name", func(t *testing.T) { ... })
t.Run("success", func(t *testing.T) { ... })
t.Run("success — with optional tags", func(t *testing.T) { ... })
```

### File structure: setup file + one test file per operation/route

- `service_test.go` and `handler_test.go` contain **only** setup (no `Test*` functions)
- One `op_<operation>_test.go` per domain operation
- One `route_<rpc>_test.go` per API route

### No mocks for infrastructure

Use testcontainers for real postgres and opensearch. Only use a no-op implementation
for the outbox in domain tests (to isolate from river).

### Test isolation

Each parent test calls `setupService(t)` or `setupHandler(t)` to get a clean context.
No shared state between parent tests. Subtests within a parent may share setup.

## Post-Generation

1. Run `make test` — all tests should pass
2. Verify testcontainers start and stop cleanly (no leaked containers)

## Checklist

- [ ] `pkg/testkit/containers.go` with testcontainers helpers (if not present)
- [ ] `service_test.go` contains only setup — no `Test*` functions
- [ ] One `op_<operation>_test.go` per domain operation
- [ ] `handler_test.go` contains only setup — no `Test*` functions
- [ ] One `route_<rpc>_test.go` per API route
- [ ] Error cases come before success cases in every parent test
- [ ] Subtest names follow `<outcome> — <description>` pattern
- [ ] `require` used for setup/create calls that would panic on failure
- [ ] `assert` used for all other assertions
- [ ] No table-driven tests
- [ ] No mocks for databases — testcontainers only
- [ ] No-op outbox used in domain tests
- [ ] `t.Cleanup()` used for all resource teardown
- [ ] Outbox tests verify job args construction
- [ ] `make test` passes
