---
description: Review a domain PR
argument-hint: <pr-number>
allowed-tools: Read, Bash, Glob, Grep
disable-model-invocation: true
context: fork
---

# Review Domain Agent

Audit a domain PR. Answer the question: **"Is the logic correct?"**

## Project Root

The PR targets one project: `connect-rpc-backend/` or `grpc-backend/`.
Identify which project from the PR file paths.

## How to review

1. Fetch the PR diff:
   ```
   gh pr diff <number>
   ```

2. Identify the domain being added and read the full files (not just the diff).

3. Check every item below. For each, report **PASS** or **FAIL** with a brief explanation.

## Checklist

### File Structure — `internal/domain/<domain>/`

- [ ] `errors.go` exists with domain-specific sentinel errors
- [ ] `events.go` exists with event type constants (`Event*`) for all outbox events
- [ ] `service.go` exists with `Service` interface, `Dependencies` struct, and `New()` constructor
- [ ] One `op_<operation>.go` file per operation (create, get, list, update, delete)
- [ ] No extra files beyond errors, events, service, and op_ files

### Interface-First Convention

- [ ] `Service` interface is exported
- [ ] `service` struct is unexported (lowercase)
- [ ] `Dependencies` struct is exported
- [ ] `New()` constructor takes `Dependencies` and returns `Service` (the interface, not the struct)
- [ ] Private struct inlines dependency fields directly (does NOT embed `Dependencies`)

### Layer Rules — Imports

Scan all imports in the domain package:

- [ ] Imports from `gen/db/<domain>` — ALLOWED
- [ ] Imports from `pkg/cache` — ALLOWED
- [ ] Imports from `pkg/outbox` — ALLOWED
- [ ] Imports from `pkg/pagination` — ALLOWED
- [ ] NO imports from `gen/sdk/` (proto/connect generated code)
- [ ] NO imports from `internal/api/`
- [ ] NO imports from `internal/outbox/`
- [ ] NO imports from `cmd/`

### Write Operations (create, update, delete)

For each write `op_*.go`, verify the transactional outbox pattern:

- [ ] Opens a transaction with `pool.Begin(ctx)`
- [ ] Has `defer tx.Rollback(ctx)` immediately after
- [ ] Executes sqlc query with `queries.WithTx(tx)`
- [ ] Emits outbox events within the transaction via `outbox.Emit(ctx, tx, ...)`
- [ ] Commits the transaction with `tx.Commit(ctx)`
- [ ] Updates cache AFTER successful commit (not before)
- [ ] Returns errors without wrapping (domain errors are sentinel)

### Read Operations (get, list)

For each read `op_*.go`, verify the cache-first pattern:

- [ ] `op_get.go` checks cache before querying the database
- [ ] `op_get.go` populates cache on cache miss
- [ ] `op_get.go` maps `pgx.ErrNoRows` to `ErrNotFound`
- [ ] `op_list.go` uses `pkg/pagination` (no local pagination helpers)
- [ ] `op_list.go` returns a `nextPageToken` when more results exist

### Delete Operation

- [ ] `op_delete.go` checks affected rows and returns `ErrNotFound` if zero
- [ ] `op_delete.go` invalidates cache after successful delete
- [ ] `op_delete.go` emits outbox event within transaction
- [ ] Cascade deletes emit a separate outbox event for the side-effect (e.g., `space.content_deleted`) so consumers can invalidate related caches

### Error Handling

- [ ] All sentinel errors defined in `errors.go` using `errors.New()`
- [ ] `ErrNotFound` exists
- [ ] `ErrAlreadyExists` exists (if SQL schema has unique constraints) — check the migration for unique indexes
- [ ] Create operation maps `pgerrcode.UniqueViolation` via `pgconn.PgError` to `ErrAlreadyExists` — no hardcoded postgres error codes
- [ ] No error wrapping that would break `errors.Is()` matching
- [ ] No generic error returns where a sentinel would be appropriate

### Outbox Events

- [ ] Event types defined as constants in `events.go` (e.g., `EventCreated = "<domain>.created"`)
- [ ] Ops use constants from `events.go` — no hardcoded event type strings
- [ ] Event types follow `<domain>.<action>` naming (e.g., `content.created`)
- [ ] Event ID is the resource ID as string
- [ ] Events emitted for: create, update, delete

## Output format

```
## Domain PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### Operation Matrix
| Operation | File | Tx | Outbox | Cache | Errors | Status |
|-----------|------|----|--------|-------|--------|--------|
| Create | op_create.go | yes | yes | set after commit | — | PASS |
| Get | op_get.go | — | — | check first | ErrNotFound | PASS |
| ... | ... | ... | ... | ... | ... | ... |

### Import Audit
| Import | Allowed | Status |
|--------|---------|--------|
| gen/db/<domain> | yes | PASS |
| gen/sdk/... | NO | FAIL |
| ... | ... | ... |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```

## PR Context

- PR diff: !`gh pr diff $ARGUMENTS`
- PR info: !`gh pr view $ARGUMENTS --json number,title,body,state,baseRefName,headRefName,url`
