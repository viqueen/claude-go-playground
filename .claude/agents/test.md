# Test Agent

Add unit and integration tests for a domain. This PR is auditable as: **"Is this adequately tested?"**

Depends on: `integrate` agent PR (full domain must be wired).

## Inputs

The user will specify:
- The **domain name** (e.g., `content`)
- Any specific test scenarios or edge cases to cover

## What to generate

### 1. Testcontainers Setup — `internal/testutil/containers.go`

If not already present, create a shared testcontainers helper:

```go
package testutil

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

### 2. Domain Unit Tests — `internal/domain/<domain>/service_test.go`

Test business logic in isolation. For each operation:

```go
func TestCreate(t *testing.T)  { /* ... */ }
func TestGet(t *testing.T)     { /* ... */ }
func TestList(t *testing.T)    { /* ... */ }
func TestUpdate(t *testing.T)  { /* ... */ }
func TestDelete(t *testing.T)  { /* ... */ }
```

These are **integration tests using testcontainers** — they run against a real postgres:
- Use `testutil.SetupPostgres()` to get a real database
- Create real `sqlc.Queries`, `pgxpool.Pool`, cache, and a no-op outbox
- Test actual SQL queries and transaction behavior
- Test error cases: not found, already exists, etc.

### 3. API Integration Tests — `internal/api/<domain>/handler_test.go`

Test the full RPC lifecycle using Connect's test utilities:

```go
func TestCreateRPC(t *testing.T)  { /* ... */ }
func TestGetRPC(t *testing.T)     { /* ... */ }
func TestListRPC(t *testing.T)    { /* ... */ }
func TestUpdateRPC(t *testing.T)  { /* ... */ }
func TestDeleteRPC(t *testing.T)  { /* ... */ }
```

- Start a real Connect server using `httptest.NewServer` with the handler
- Use a generated Connect client to make RPC calls
- Assert on responses and error codes
- Use testcontainers for the database backend

### 4. Outbox Worker Tests — `internal/outbox/<domain>/event_<concern>_test.go`

Test that workers process jobs correctly:

- Verify job args are constructed correctly from events
- Test worker execution (may require testcontainers for opensearch if indexing)

## Test Patterns

### No mocks for infrastructure
Use testcontainers for real postgres and opensearch. Only use a no-op implementation for the outbox
in domain unit tests (to isolate from river).

### No-op Outbox for domain tests

```go
type noopOutbox[T any] struct{}

func (n *noopOutbox[T]) Emit(ctx context.Context, tx T, events ...outbox.Event) error {
    return nil
}
```

### Table-driven tests
Use subtests with `t.Run()` for related test cases.

### Test naming
Use descriptive names: `TestCreate_DuplicateTitle_ReturnsAlreadyExists`, not `TestCreate1`.

## Post-Generation

1. Run `make test` — all tests should pass
2. Verify testcontainers start and stop cleanly (no leaked containers)

## Checklist

- [ ] `internal/testutil/containers.go` with testcontainers helpers (if not present)
- [ ] Domain tests cover all operations + error cases
- [ ] API tests cover full RPC lifecycle with real Connect client
- [ ] Outbox tests verify job args construction
- [ ] No mocks for databases — testcontainers only
- [ ] `t.Cleanup()` used for all resource teardown
- [ ] `make test` passes
