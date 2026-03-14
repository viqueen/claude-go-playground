---
description: Review a test PR
argument-hint: <pr-number>
allowed-tools: Read, Bash, Glob, Grep
disable-model-invocation: true
context: fork
---

# Review Test Agent

Audit a test PR. Answer the question: **"Is this adequately tested?"**

## How to review

1. Fetch the PR diff:
   ```
   gh pr diff <number>
   ```

2. Identify the domain being tested and read the full files (not just the diff).

3. Identify the project from the PR file paths: `connect-rpc-backend/` or `grpc-backend/`.

4. Check every item below. For each, report **PASS** or **FAIL** with a brief explanation.
   Items marked **(Connect-RPC)** or **(gRPC)** only apply to the corresponding project.

## Checklist

### Testcontainers Setup — `pkg/testkit/containers.go`

- [ ] `SetupPostgres()` starts a postgres container via `testcontainers-go/modules/postgres`
- [ ] `SetupPostgres()` runs goose migrations against the container
- [ ] `SetupPostgres()` returns a connection string
- [ ] Container teardown uses `t.Cleanup()` (not `defer` in test functions)
- [ ] No hardcoded ports (testcontainers assigns dynamic ports)

### File Structure

- [ ] No `service_test.go` or `op_*_test.go` files (domain tests are redundant — API integration tests cover the full stack)
- [ ] `handler_test.go` contains **only** setup (no `Test*` functions)
- [ ] One `route_<rpc>_test.go` per API route
- [ ] Outbox tests in `event_<concern>_test.go`

### API Tests — `internal/api/<domain>/v1/`

- [ ] `handler_test.go` defines `accessLevel` enum: `anonymous`, `standard`, `admin`, `elevated`
- [ ] `handler_test.go` defines `testClients[T]` struct with all four client fields
- [ ] `handler_test.go` defines `panicService` for interceptor-only tests
- [ ] `setupHandler(t)` — no database, uses panic service (interceptor-level errors)
- [ ] `setupHandlerWithDB(t)` — testcontainers postgres, real service (domain errors + success)
- [ ] `startServer(t, handler)` — shared helper creating server + clients (used by both setup functions)
- [ ] Each client injects auth via interceptor (Connect-RPC: header, gRPC: metadata)
- [ ] `anonymous` client sends no auth credentials
- [ ] **(Connect-RPC)** Uses `httptest.NewServer` with per-level Connect clients
- [ ] **(gRPC)** Uses `bufconn` with per-level gRPC clients
- [ ] `setupHandlerWithDB` uses testcontainers for the database backend (no mocks)
- [ ] Each `route_*_test.go` has two parent tests: `Test<Endpoint>_Errors` + `Test<Endpoint>_Success`
- [ ] `_Errors` calls `setupHandler(t)` at parent level — interceptor errors only (no DB)
- [ ] `_Success` calls `setupHandlerWithDB(t)` at parent level — domain errors + success cases (with DB)
- [ ] Subtests within each parent call `t.Parallel()`
- [ ] Tests cover all RPCs: create, get, list, update, delete
- [ ] Every RPC tests unauthenticated path via `clients.anonymous` in `_Errors`
- [ ] Admin-only RPCs test `clients.standard` → `PermissionDenied` in `_Errors`
- [ ] Elevated RPCs test `clients.admin` → `PermissionDenied` in `_Errors`
- [ ] Domain errors (not found, already exists) go in `_Success` (they need the DB)
- [ ] Multiple success subtests cover different API contract scenarios
- [ ] Tests verify correct error codes:
  - [ ] **(Connect-RPC)** `connect.CodeUnauthenticated`, `connect.CodePermissionDenied`, `connect.CodeNotFound`, etc.
  - [ ] **(gRPC)** `codes.Unauthenticated`, `codes.PermissionDenied`, `codes.NotFound`, etc. (via `status.Code(err)`)

### Outbox Worker Tests — `internal/outbox/<domain>/`

- [ ] Tests verify `JobArgs` construction from `outbox.Event`
- [ ] Tests verify `Kind()` returns correct job type string

### Test Case Ordering

- [ ] `_Errors`: unauthenticated (anonymous) → invalid argument (standard) → permission denied (standard on admin ops)
- [ ] `_Success`: not found → already exists → success cases (one or more)

### Test Naming

- [ ] Parent test functions: `Test<Endpoint>_Errors` and `Test<Endpoint>_Success`
- [ ] `_Errors` subtest names: `<error code> — <description>` (e.g., `"unauthenticated — no token"`)
- [ ] `_Success` subtest names: descriptive of scenario (e.g., `"creates with optional tags"`, `"not found — nonexistent ID"`)
- [ ] No numbered test names (e.g., `TestCreate1`)

### Assertions

- [ ] Uses `github.com/stretchr/testify/assert` for general assertions
- [ ] Uses `github.com/stretchr/testify/require` for setup calls that would panic on failure
- [ ] `require.NoError` used after create/setup calls
- [ ] `assert` used for value comparisons and error code checks
- [ ] No use of raw `if err != nil { t.Fatal() }` patterns

### Anti-patterns

- [ ] No table-driven tests (`[]struct{ name string; ... }`)
- [ ] No mocks for databases — testcontainers only (when DB is needed)
- [ ] No `Test*` functions in setup files (`handler_test.go`)
- [ ] No `service_test.go` or `op_*_test.go` files (domain tests are redundant)
- [ ] No shared state between parent tests
- [ ] No testcontainers in `_Errors` tests (interceptor-only, must use `setupHandler`)
- [ ] No missing `t.Parallel()` in route subtests
- [ ] No empty-state assertions in parallel subtests sharing a database (siblings may insert rows concurrently)
- [ ] Update test proto messages populate all validated required fields (protovalidate validates entire nested messages, even for partial updates)
- [ ] List queries use deterministic `ORDER BY` with a tiebreaker column (e.g., `created_at, id`)

## Output format

```
## Test PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### File Structure
| Expected File | Present | Setup Only | Status |
|---------------|---------|------------|--------|
| handler_test.go | yes | yes | PASS |
| route_create_content_test.go | yes | — | PASS |
| route_delete_content_test.go | yes | — | PASS |
| ... | ... | ... | ... |

### Coverage Matrix
| Operation | _Errors (no DB) | _Success (with DB) | t.Parallel | Status |
|-----------|----------------|-------------------|------------|--------|
| CreateContent | unauth, invalid arg | success (x2) | yes | PASS |
| DeleteContent | unauth, perm denied | not found, success | yes | PASS |
| ... | ... | ... | ... | ... |

### Assertion Usage
| File | require (setup) | assert (checks) | Status |
|------|----------------|-----------------|--------|
| op_create_test.go | yes | yes | PASS |
| ... | ... | ... | ... |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```

## PR Context

- PR diff: !`gh pr diff $ARGUMENTS`
- PR info: !`gh pr view $ARGUMENTS --json number,title,body,state,baseRefName,headRefName,url`
