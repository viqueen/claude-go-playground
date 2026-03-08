---
description: Reviews test PRs — verifies test coverage and testcontainers usage
tools: Read, Bash, Glob, Grep
---

# Review Test Agent

Audit a test PR. Answer the question: **"Is this adequately tested?"**

## How to review

1. Fetch the PR diff:
   ```
   gh pr diff <number>
   ```

2. Identify the domain being tested and read the full files (not just the diff).

3. Check every item below. For each, report **PASS** or **FAIL** with a brief explanation.

## Checklist

### Testcontainers Setup — `internal/testutil/containers.go`

- [ ] `SetupPostgres()` starts a postgres container via `testcontainers-go/modules/postgres`
- [ ] `SetupPostgres()` runs goose migrations against the container
- [ ] `SetupPostgres()` returns a connection string
- [ ] Container teardown uses `t.Cleanup()` (not `defer` in test functions)
- [ ] No hardcoded ports (testcontainers assigns dynamic ports)

### Domain Tests — `internal/domain/<domain>/service_test.go`

- [ ] Tests use `testutil.SetupPostgres()` for a real database (no mocks)
- [ ] Uses a no-op outbox implementation to isolate from river
- [ ] Tests cover all operations: create, get, list, update, delete
- [ ] Tests cover error cases:
  - [ ] Get non-existent resource → `ErrNotFound`
  - [ ] Delete non-existent resource → `ErrNotFound`
  - [ ] Create duplicate (if applicable) → `ErrAlreadyExists`
- [ ] Tests verify cache behavior:
  - [ ] Get after create returns cached value
  - [ ] Get after delete returns not found
- [ ] Tests verify pagination (list with page_size + page_token)
- [ ] Uses `t.Run()` subtests for related cases
- [ ] Test names are descriptive (e.g., `TestGet_NotFound_ReturnsError`)

### API Integration Tests — `internal/api/<domain>/v1/handler_test.go`

- [ ] Tests use `httptest.NewServer` with the Connect handler
- [ ] Tests use the generated Connect client to make RPC calls
- [ ] Tests cover all RPCs: create, get, list, update, delete
- [ ] Tests verify correct Connect error codes on failures:
  - [ ] Not found → `connect.CodeNotFound`
  - [ ] Already exists → `connect.CodeAlreadyExists`
  - [ ] Invalid argument → `connect.CodeInvalidArgument`
- [ ] Tests verify response structure (correct fields populated)
- [ ] Tests use a real database via testcontainers (not mocked)

### Outbox Worker Tests — `internal/outbox/<domain>/`

- [ ] Tests verify `JobArgs` construction from `outbox.Event`
- [ ] Tests verify `Kind()` returns correct job type string
- [ ] Tests verify worker execution (if workers have real logic)

### Test Patterns

- [ ] No mocks for databases — testcontainers only
- [ ] No-op outbox used only in domain tests (to isolate from river)
- [ ] Table-driven tests with `t.Run()` where applicable
- [ ] Descriptive test names: `Test<Operation>_<Scenario>_<Expected>`
- [ ] No test pollution: each test creates its own resources, no shared state across tests
- [ ] All resources cleaned up via `t.Cleanup()`

### No Missing Coverage

Cross-reference with the domain and integrate PRs:

- [ ] Every domain operation has at least one happy-path test
- [ ] Every domain operation has at least one error-path test
- [ ] Every RPC has at least one integration test
- [ ] Every outbox event type has args construction verified

## Output format

```
## Test PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### Coverage Matrix
| Layer | Operation | Happy Path | Error Path | Status |
|-------|-----------|------------|------------|--------|
| Domain | Create | TestCreate | TestCreate_Duplicate | PASS |
| Domain | Get | TestGet | TestGet_NotFound | PASS |
| API | CreateRPC | TestCreateRPC | TestCreateRPC_InvalidArg | PASS |
| ... | ... | ... | ... | ... |

### Testcontainers Usage
| Container | Setup | Teardown | Migrations | Status |
|-----------|-------|----------|------------|--------|
| Postgres | SetupPostgres | t.Cleanup | goose | PASS |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```
