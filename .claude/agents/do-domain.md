# Domain Agent

Implement the business logic for a domain. This PR is auditable as: **"Is the logic correct?"**

Depends on: `do-entity-store` agent PR (`gen/db/` must exist).

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

### 2. `internal/domain/<domain>/service.go`

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

### 3. `internal/domain/<domain>/op_<operation>.go` — One file per operation

Each write operation:
- Opens a transaction
- Executes the sqlc query within the transaction
- Emits outbox events within the transaction
- Commits
- Updates cache after commit

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
- **Transactional outbox**: events emitted inside the DB transaction, before commit
- **Cache after commit**: only update cache after successful commit
- **No store abstraction**: sqlc IS the store — `Queries` used directly

## Layer Rules

- Can depend on: `gen/db/<domain>`, `pkg/cache`, `pkg/outbox`
- Must NOT depend on: `gen/sdk/`, `internal/api/`, `internal/outbox/`, `cmd/`

## Post-Generation

1. Run `make vet` — fix all compilation errors
2. Review that each `op_*.go` follows the transaction + outbox + cache pattern

## Checklist

- [ ] `errors.go` with domain-specific sentinel errors
- [ ] `service.go` with interface, Dependencies, constructor
- [ ] One `op_*.go` per operation
- [ ] All writes use transaction + outbox + cache pattern
- [ ] All reads check cache first
- [ ] `pgx.ErrNoRows` mapped to `ErrNotFound`
- [ ] No imports from `internal/api/`, `internal/outbox/`, `cmd/`, or `gen/sdk/`
- [ ] `make vet` passes
