---
description: Implement business logic for a domain
argument-hint: <domain> <project>
allowed-tools: Read, Write, Edit, Bash, Glob, Grep
disable-model-invocation: true
context: fork
---

Domain: $0
Project: $1

# Domain Agent

Implement the business logic for a domain. This PR is auditable as: **"Is the logic correct?"**

Depends on: `do-entity-store` agent PR (`gen/db/` must exist).

## Project Root

All file paths are relative to the chosen project: `connect-rpc-backend/` or `grpc-backend/`.
The user will specify which project. All `make` commands must be run from the project root.

## Inputs

The user will specify:
- The **domain name** (e.g., `content`)
- Any specific business rules, validation logic, or error conditions

## What to generate

### 1. `internal/domain/<domain>/errors.go`

Sentinel errors for the domain:

```go
package <domain>

import "errors"

var (
    ErrNotFound      = errors.New("<domain> not found")
    ErrAlreadyExists = errors.New("<domain> already exists")
    // ... domain-specific errors
)
```

### 2. `internal/domain/<domain>/events.go`

Event type constants for outbox events. These are shared between the domain ops and the outbox `mapEvent` — never hardcode event type strings.

```go
package <domain>

const (
    EventCreated = "<domain>.created"
    EventUpdated = "<domain>.updated"
    EventDeleted = "<domain>.deleted"
    // ... additional events for cascade side-effects
)
```

### 3. `internal/domain/<domain>/service.go`

Service interface + private struct + constructor:

```go
package <domain>

type Service interface {
    Create(ctx context.Context, params db<domain>.Create<Resource>Params) (*db<domain>.<Resource>, error)
    Get(ctx context.Context, id uuid.UUID) (*db<domain>.<Resource>, error)
    List(ctx context.Context, pageSize int32, pageToken string) ([]db<domain>.<Resource>, string, error)
    Update(ctx context.Context, id uuid.UUID, params db<domain>.Update<Resource>Params) (*db<domain>.<Resource>, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

type Dependencies struct {
    Pool    *pgxpool.Pool
    Queries *db<domain>.Queries
    Cache   cache.Cache[uuid.UUID, *db<domain>.<Resource>]
    Outbox  outbox.Outbox[pgx.Tx]
}

func New(deps Dependencies) Service {
    return &service{ /* inline deps */ }
}
```

### 4. `internal/domain/<domain>/op_<operation>.go` — One file per operation

Each write operation:
- Opens a transaction
- Executes the sqlc query within the transaction
- Maps `pgerrcode.UniqueViolation` to `ErrAlreadyExists` when the SQL schema has unique constraints (use `github.com/jackc/pgerrcode` — never hardcode postgres error codes)
- Emits outbox events within the transaction
- Commits
- Updates cache after commit

Cascade deletes (e.g., deleting a parent soft-deletes children):
- Emit a separate outbox event for the cascade side-effect so consumers can invalidate related caches

Each read operation:
- Checks cache first
- Falls back to sqlc query
- Populates cache on miss
- Maps `pgx.ErrNoRows` → `ErrNotFound`

## Conventions

- **Interface-first**: `Service` interface is public, `service` struct is private
- **Dependencies struct**: exported, used only in constructor signature
- **Private struct inlines fields**: does NOT embed `Dependencies`
- **File per operation**: `op_create.go`, `op_get.go`, `op_list.go`, `op_update.go`, `op_delete.go`
- **Event type constants**: define in `events.go`, use in ops — never hardcode event type strings
- **Transactional outbox**: events emitted inside the DB transaction, before commit
- **Cache after commit**: only update cache after successful commit
- **No store abstraction**: sqlc IS the store — `Queries` used directly

## Layer Rules

- Can depend on: `gen/db/<domain>`, `pkg/cache`, `pkg/outbox`, `pkg/pagination`
- Must NOT depend on: `gen/sdk/`, `internal/api/`, `internal/outbox/`, `cmd/`
- Use `pkg/pagination.DecodePageToken` and `pkg/pagination.NextPageToken` for list operations — do NOT duplicate pagination logic in domain packages

## Post-Generation

1. Run `make vet` — fix all compilation errors
2. Review that each `op_*.go` follows the transaction + outbox + cache pattern

## Checklist

- [ ] `errors.go` with domain-specific sentinel errors
- [ ] `events.go` with event type constants (`Event*`) — used by ops and outbox
- [ ] `service.go` with interface, Dependencies, constructor
- [ ] One `op_*.go` per operation
- [ ] Outbox events use constants from `events.go` (not hardcoded strings)
- [ ] All writes use transaction + outbox + cache pattern
- [ ] Create maps `pgerrcode.UniqueViolation` → `ErrAlreadyExists` when unique constraints exist
- [ ] Cascade deletes emit outbox events for affected related entities
- [ ] All reads check cache first
- [ ] List operations use `pkg/pagination` (not local helpers)
- [ ] `pgx.ErrNoRows` mapped to `ErrNotFound`
- [ ] No imports from `internal/api/`, `internal/outbox/`, `cmd/`, or `gen/sdk/`
- [ ] `make vet` passes
