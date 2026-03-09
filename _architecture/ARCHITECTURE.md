# Architecture

Written companion to [`platform-backend.png`](platform-backend.png).

---

## Codebase Layers

The backend is organised into four layers, each with a clear responsibility and
strict dependency rules. Every layer maps to a directory under the project root.

```
cmd/server/           APP        entry point — bootstrap, wiring
internal/api/         API        request handling, proto ↔ domain mapping
internal/domain/      DOMAIN     business logic, transactions, canonical store
internal/outbox/      OUTBOX     async event processing, data projections
```

Supporting packages live outside the layers:

```
pkg/                  shared utilities — config, cache, outbox interface, grpcapp/connectapp, etc.
gen/sdk/              buf-generated proto + RPC stubs (gitignored)
gen/db/<schema>/      sqlc-generated query code, grouped by schema (gitignored)
protos/               protobuf source definitions
sql/                  migrations + sqlc query files
```

### APP — `cmd/server/`

Application bootstrap. Loads config, opens connections, runs migrations, wires
domains, and starts the RPC server. No business logic lives here.

Files:
- `main.go` — signal context, config, setup sequence, run
- `setup_connections.go` — postgres pool, river client, goose + river migrations
- `setup_domains.go` — domain service constructors, dependency injection
- `setup_gateway.go` — RPC handlers registered on the server with interceptors

### API — `internal/api/<domain>/v1/`

Maps between the external proto contract and the internal domain model. Each
domain's API lives in a versioned package mirroring the proto package
(`content.v1` → `internal/api/content/v1/`, package `apicontentv1`).

Files per domain:
- `handler.go` — `Dependencies` struct, constructor, error mappings
- `mapper.go` — `toProto`, `fromProtoCreate`, `fromProtoUpdate`
- `route_<rpc>.go` — one file per RPC method

Responsibilities:
- Validate state (field presence, permission checks)
- Delegate to domain service
- Map domain errors → RPC error codes
- Map sqlc models → proto responses

Errors originating here: `PERMISSION_DENIED`, `NOT_FOUND`, `ALREADY_EXISTS`, `PRECONDITION_FAILED`

### DOMAIN — `internal/domain/<domain>/`

Pure business logic. Operates on the canonical datastore (postgres via sqlc).
Everything that "must happen together" happens in a single transaction.

Files per domain:
- `errors.go` — sentinel errors (`ErrNotFound`, `ErrAlreadyExists`, etc.)
- `service.go` — `Service` interface, `Dependencies` struct, `New()` constructor
- `op_<operation>.go` — one file per operation (create, get, list, update, delete)

Patterns:
- **Write path**: begin tx → execute query → emit outbox events → commit → update cache
- **Read path**: check cache → query on miss → populate cache → return
- **Error mapping**: `pgx.ErrNoRows` → `ErrNotFound`

Errors originating here: `INTERNAL`, `UNAVAILABLE`, `DEADLINE_EXCEEDED`

### OUTBOX — `internal/outbox/`

Asynchronous event processing. Domain events emitted inside transactions are
picked up by River workers and projected into different query stores or
side-effects.

Files:
- `river.go` — `NewRiverOutbox` constructor, `mapEvent` switch (event → jobs)
- `<domain>/event_<concern>.go` — one file per concern (index, audit, analytics, graph, etc.)

Worker concerns (from the architecture diagram):
- **audit** — immutable event log
- **index** — search index projections (opensearch)
- **analytics** — analytics data projections
- **graph** — relationship graph projections

---

## Request Lifecycle

Every RPC follows the same path through the layers:

```
RPC(ctx, Request) → (Response, error)

  ┌─────────────────────────────────────────────────┐
  │ Interceptors (pkg/grpcutil or pkg/connectutil)   │
  │   validate ctx  → UNAUTHENTICATED               │
  │   validate request (buf/validate) → INVALID_ARG  │
  ├─────────────────────────────────────────────────┤
  │ Handler (internal/api/<domain>/v1/)             │
  │   validate state → PERMISSION_DENIED             │
  │   call domain service                            │
  │   map response → NOT_FOUND, ALREADY_EXISTS, etc. │
  ├─────────────────────────────────────────────────┤
  │ Service (internal/domain/<domain>/)             │
  │   synchronous: cache, DB, index, external        │
  │   → INTERNAL, UNAVAILABLE, DEADLINE_EXCEEDED     │
  ├─────────────────────────────────────────────────┤
  │ Workers (internal/outbox/)                      │
  │   asynchronous: audit, index, analytics, graph   │
  └─────────────────────────────────────────────────┘
```

### Synchronous dependencies (service layer)

| Dependency | Purpose | Implementation |
|---|---|---|
| **DB** | Canonical store, source of truth | postgres via sqlc |
| **Cache** | Read-through cache, invalidated on writes | `pkg/cache.Cache[K,V]` |
| **Index** | Search projections (read in domain if needed) | opensearch client |
| **External** | Calls to other services or APIs | domain-specific |

### Asynchronous dependencies (worker layer)

| Worker | Purpose | Triggered by |
|---|---|---|
| **audit** | Immutable event log | created, deleted |
| **index** | Search index projection | created, updated, deleted |
| **analytics** | Analytics data projection | domain-specific events |
| **graph** | Relationship graph projection | domain-specific events |

---

## gRPC Error Mapping

Errors are assigned at the layer where they originate. Each layer has a defined
set of error codes it may return.

### Interceptor errors (before handler)

| gRPC Code | HTTP | Cause |
|---|---|---|
| `CANCELED` | 499 | Client cancelled the request |
| `UNAUTHENTICATED` | 401 | Missing or invalid credentials |
| `INVALID_ARGUMENT` | 400 | Request validation failed (buf/validate) |

### Handler errors (API layer)

| gRPC Code | HTTP | Cause |
|---|---|---|
| `PERMISSION_DENIED` | 403 | Caller lacks required permissions |
| `NOT_FOUND` | 404 | Resource does not exist |
| `ALREADY_EXISTS` | 409 | Duplicate resource |
| `PRECONDITION_FAILED` | 412 | State precondition not met |

### Service errors (domain layer)

| gRPC Code | HTTP | Cause |
|---|---|---|
| `INTERNAL` | 500 | Unexpected server error |
| `UNAVAILABLE` | 503 | Service dependency unavailable |
| `DEADLINE_EXCEEDED` | 504 | Operation timed out |

Error flow: domain raises sentinel errors → handler maps them to RPC error codes
via `grpcutil.NewErrorFrom(err, errorMappings)` (gRPC) or `connectutil.NewErrorFrom(err, errorMappings)` (Connect-RPC).

---

## Layer Dependency Rules

```
pkg/               → nothing (zero internal dependencies)
internal/domain/   → gen/db/, pkg/cache, pkg/outbox
internal/outbox/   → gen/db/, pkg/outbox, river
internal/api/      → internal/domain/, gen/sdk/, gen/db/, pkg/grpcutil or pkg/connectutil
cmd/server/        → everything (wiring layer)
```

Forbidden:
- `internal/domain/` must NOT import `gen/sdk/`, `internal/api/`, `internal/outbox/`, or `cmd/`
- `internal/outbox/` must NOT import `internal/domain/` or `internal/api/`
- `internal/api/` must NOT import `internal/outbox/`
- `pkg/` must NOT import `internal/`, `cmd/`, or `gen/`

---

## Conventions

### Interface-first

Every package exposes an **interface** as its public API. Structs are unexported.
Constructors return the interface type.

```go
type Service interface { ... }         // exported
type service struct { ... }            // unexported
func New(deps Dependencies) Service    // returns interface
```

### Dependencies struct

Each layer defines an exported `Dependencies` struct. Constructors accept it as
the single parameter. The private struct inlines the fields (does NOT embed
Dependencies).

### File naming

| Layer | Pattern | Example |
|---|---|---|
| API | `route_<rpc>.go` | `route_create_content.go` |
| Domain | `op_<operation>.go` | `op_create.go` |
| Outbox | `event_<concern>.go` | `event_index.go` |

### Import aliases

| Alias pattern | Example | Points to |
|---|---|---|
| `<domain>v1` | `spacev1` | `gen/sdk/space/v1` |
| `<domain>v1connect` | `spacev1connect` | `gen/sdk/space/v1/spacev1connect` (Connect-RPC only) |
| `db<schema>` | `dbcollaboration` | `gen/db/collaboration` |
| `<domain>domain` | `spacedomain` | `internal/domain/space` |
| `<domain>events` | `spaceevents` | `internal/outbox/space` |

### Go package naming

API packages are named `api<domain><version>` (e.g., `apicontentv1`), matching
the versioned directory path `internal/api/content/v1/`.

### Soft deletes

All entity tables have a `deleted_at TIMESTAMPTZ` nullable column. Active rows
have `deleted_at IS NULL`. Soft-deleted rows have a timestamp. All read and
update queries filter with `AND deleted_at IS NULL`.

### Transactional outbox

Write operations follow: begin tx → query → emit events → commit → update cache.
Events are inserted as River jobs within the same transaction, guaranteeing
at-least-once delivery.

---

## Extension Points

The following concerns are shown in the architecture but not included in the
initial scaffold. They are added per-domain as needed:

| Concern | Where | When to add |
|---|---|---|
| **Authentication** | Interceptor in `pkg/grpcutil` or `pkg/connectutil` | When the service requires caller identity |
| **Authorization** | Handler-level check in `route_*.go` | When RPCs have permission requirements |
| **External service calls** | Domain `Dependencies` struct | When a domain needs to call another service |
| **Analytics workers** | `internal/outbox/<domain>/event_analytics.go` | When analytics projections are needed |
| **Graph workers** | `internal/outbox/<domain>/event_graph.go` | When relationship graph projections are needed |
