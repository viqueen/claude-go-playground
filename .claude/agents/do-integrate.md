# Integrate Agent

Wire a domain into the server — API handler, outbox workers, and cmd/server registration.
This PR is auditable as: **"Is this wired correctly?"**

Depends on: `do-domain` agent PR (internal/domain/<domain>/ must exist).

## Inputs

The user will specify:
- The **domain name** (e.g., `content`)
- The **project**: `connect-rpc-backend` or `grpc-backend`
- Any specific mapping concerns (e.g., custom proto ↔ sqlc field mappings)

## Project Root

All file paths below are relative to the chosen project folder.
All `make` commands must be run from the project root.

The framework is determined by the project:
- `connect-rpc-backend` → Connect-RPC (uses `connectutil`, `connectapp`, `connect.Request` wrappers)
- `grpc-backend` → gRPC (uses `grpcutil`, `grpcapp`, direct proto messages, `Unimplemented*Server` embedding)

## What to generate

### 1. `internal/api/<domain>/v1/handler.go`

Private struct implementing the generated service interface.

The Go package name must be `api<domain><version>` (e.g., `apicontentv1` for `internal/api/content/v1/`).
Import aliases must follow a consistent naming convention.

#### If Connect-RPC

```go
package apicontentv1

import (
	"connectrpc.com/connect"

	contentv1 "<module>/gen/sdk/content/v1"
	contentv1connect "<module>/gen/sdk/content/v1/contentv1connect"
	dbcontent "<module>/gen/db/content"
	contentdomain "<module>/internal/domain/content"
	"<module>/pkg/connectutil"
)

// Dependencies defines the dependencies for the content API handler.
type Dependencies struct {
	Service contentdomain.Service
}

// New returns the Connect-generated handler interface. Struct is private.
func New(deps Dependencies) contentv1connect.ContentServiceHandler {
	return &handler{service: deps.Service}
}

type handler struct {
	service contentdomain.Service
}

var errorMappings = map[error]connect.Code{
	contentdomain.ErrNotFound:      connect.CodeNotFound,
	contentdomain.ErrAlreadyExists: connect.CodeAlreadyExists,
}
```

Import alias conventions (Connect-RPC):
- `<domain>v1` for proto types: `contentv1 "<module>/gen/sdk/content/v1"`
- `<domain>v1connect` for connect service: `contentv1connect "<module>/gen/sdk/content/v1/contentv1connect"`
- `db<domain>` for sqlc types: `dbcontent "<module>/gen/db/content"`
- `<domain>domain` for domain service: `contentdomain "<module>/internal/domain/content"`

#### If gRPC

```go
package apicontentv1

import (
	contentv1 "<module>/gen/sdk/content/v1"
	contentv1grpc "<module>/gen/sdk/content/v1/contentv1grpc"
	dbcontent "<module>/gen/db/content"
	contentdomain "<module>/internal/domain/content"
	"<module>/pkg/grpcutil"

	"google.golang.org/grpc/codes"
)

// Dependencies defines the dependencies for the content API handler.
type Dependencies struct {
	Service contentdomain.Service
}

// New returns the gRPC-generated service server. Struct is private.
func New(deps Dependencies) contentv1grpc.ContentServiceServer {
	return &handler{service: deps.Service}
}

type handler struct {
	contentv1grpc.UnimplementedContentServiceServer
	service contentdomain.Service
}

var errorMappings = map[error]codes.Code{
	contentdomain.ErrNotFound:      codes.NotFound,
	contentdomain.ErrAlreadyExists: codes.AlreadyExists,
}
```

Import alias conventions (gRPC):
- `<domain>v1` for proto types: `contentv1 "<module>/gen/sdk/content/v1"`
- `<domain>v1grpc` for gRPC service: `contentv1grpc "<module>/gen/sdk/content/v1/contentv1grpc"`
- `db<domain>` for sqlc types: `dbcontent "<module>/gen/db/content"`
- `<domain>domain` for domain service: `contentdomain "<module>/internal/domain/content"`

Note: gRPC handlers embed `Unimplemented<Service>Server` for forward compatibility.

### 2. `internal/api/<domain>/v1/mapper.go`

Mapping functions between proto types and sqlc models:

- `toProto(model) *proto.Resource` — sqlc model → proto response
- `fromProtoCreate(msg) sqlcparams` — proto create request → sqlc create params
- `validateUpdateMask(paths) error` — validates update_mask paths are non-empty and supported
- `fromProtoUpdate(msg) sqlcparams` — proto update request → sqlc update params
  - Uses `pgtype.Text{String: val, Valid: true}` for nullable string fields from `sqlc.narg()`
  - Uses `pgtype.Int4{Int32: val, Valid: true}` for nullable int fields

The `validateUpdateMask` function must:
- Reject empty `paths` with an error
- Reject unsupported field paths with an error
- Define a `supportedUpdatePaths` map as package-level var listing all valid paths

### 3. `internal/api/<domain>/v1/route_<rpc>.go` — One file per RPC

Each file contains a single method on the handler:

#### If Connect-RPC

```go
func (h *handler) Create<Resource>(
    ctx context.Context,
    req *connect.Request[<domain>v1.Create<Resource>Request],
) (*connect.Response[<domain>v1.Create<Resource>Response], error) {
    result, err := h.service.Create(ctx, fromProtoCreate(req.Msg))
    if err != nil {
        return nil, connectutil.NewErrorFrom(err, errorMappings)
    }
    return connect.NewResponse(&<domain>v1.Create<Resource>Response{
        <Resource>: toProto(result),
    }), nil
}
```

#### If gRPC

```go
func (h *handler) Create<Resource>(
    ctx context.Context,
    req *<domain>v1.Create<Resource>Request,
) (*<domain>v1.Create<Resource>Response, error) {
    result, err := h.service.Create(ctx, fromProtoCreate(req))
    if err != nil {
        return nil, grpcutil.NewErrorFrom(err, errorMappings)
    }
    return &<domain>v1.Create<Resource>Response{
        <Resource>: toProto(result),
    }, nil
}
```

Key differences: gRPC handlers take proto messages directly (no `connect.Request` wrapper)
and return proto messages directly (no `connect.Response` wrapper).

#### Route Error Handling Rules

- **UUID parsing errors**: return `codes.InvalidArgument` (or `connect.CodeInvalidArgument`) directly via `status.Errorf` — do NOT pass through `errorMappings` (which would fall through to `codes.Internal`)
- **Update id mismatch**: if the request has both a top-level `id` and a nested resource `id` (e.g., `req.id` and `req.space.id`), reject with `InvalidArgument` when they differ
- **Update mask validation**: call `validateUpdateMask()` before `fromProtoUpdate()` — reject empty or unsupported paths with `InvalidArgument`
- **Domain errors**: use `grpcutil.NewErrorFrom(err, errorMappings)` or `connectutil.NewErrorFrom(err, errorMappings)` for errors from the service layer

### 4. `internal/outbox/river.go` — Update event mapping

If this is the first domain, create `internal/outbox/river.go` with the `NewRiverOutbox` constructor
and `mapEvent` switch. If it already exists, add cases for the new domain's event types.

```go
package outbox

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	contentdomain "<module>/internal/domain/content"
	contentevents "<module>/internal/outbox/content"
	"<module>/pkg/outbox"
)

func NewRiverOutbox(client *river.Client[pgx.Tx]) outbox.Outbox[pgx.Tx] {
	return &riverOutbox{client: client}
}

type riverOutbox struct {
	client *river.Client[pgx.Tx]
}

func (o *riverOutbox) Emit(ctx context.Context, tx pgx.Tx, events ...outbox.Event) error {
	for _, event := range events {
		jobs, err := o.mapEvent(event)
		if err != nil {
			return err
		}
		for _, args := range jobs {
			if _, err := o.client.InsertTx(ctx, tx, args, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// mapEvent fans out a domain event into one or more river jobs.
func (o *riverOutbox) mapEvent(event outbox.Event) ([]river.JobArgs, error) {
	switch event.Type {
	case contentdomain.EventCreated:
		return []river.JobArgs{
			contentevents.NewIndexArgs(event),
			contentevents.NewAuditArgs(event),
		}, nil
	case contentdomain.EventUpdated:
		return []river.JobArgs{
			contentevents.NewIndexArgs(event),
			contentevents.NewAuditArgs(event),
		}, nil
	case contentdomain.EventDeleted:
		return []river.JobArgs{
			contentevents.NewIndexArgs(event),
			contentevents.NewAuditArgs(event),
		}, nil
	default:
		return nil, fmt.Errorf("unknown event type: %s", event.Type)
	}
}
```

**Key rules:**
- **Shared constants**: use `<domain>domain.Event*` constants from `internal/domain/<domain>/events.go` — never hardcode event type strings
- **Audit all events**: every event type must fan out to the audit worker (all operations are auditable)

Import alias conventions for outbox:
- `<domain>domain` for event constants: `contentdomain "<module>/internal/domain/content"`
- `<domain>events` for domain event workers: `contentevents "<module>/internal/outbox/content"`

### 5. `internal/outbox/<domain>/event_<concern>.go` — One file per concern

Each file contains river `JobArgs` + `Worker` for a specific concern:

- `event_index.go` — indexing concern
- `event_audit.go` — auditing concern
- Add more as needed (analytics, graph, etc.)

Workers must accept `ctx context.Context` (not `_`) and use `log.Ctx(ctx)` for context-aware logging, even if the current implementation is a stub.

### 6. `cmd/server/` — Wiring updates

Update the setup files to register the new domain:

- `setup_connections.go` — register outbox workers with river (`river.AddWorker`)
- `setup_domains.go` — add domain to `Domains` struct, wire service with dependencies
- `setup_gateway.go`:
  - **Connect-RPC**: register Connect handler with `connect.WithInterceptors`, add path to mux
  - **gRPC**: register gRPC service on `application.Server()` via generated `Register<Service>Server()`

## Conventions

- **Go package naming**: `api<domain><version>` (e.g., `apicontentv1` for `internal/api/content/v1/`)
- **API versioning**: `internal/api/<domain>/v1/` mirrors the proto package `<domain>.v1`. When a v2 proto is introduced, handlers go under `internal/api/<domain>/v2/`.
- **Import aliases**: `<domain>v1` (proto), `<domain>v1connect` or `<domain>v1grpc` (service), `db<domain>` (sqlc), `<domain>domain` (domain service), `<domain>events` (outbox events)
- **File prefixes**: `route_<rpc>.go` in api, `event_<concern>.go` in outbox
- **Error mappings**: defined as package-level var in `handler.go`, used by all routes via `connectutil.NewErrorFrom` (Connect-RPC) or `grpcutil.NewErrorFrom` (gRPC)
- **Mapper isolation**: all proto ↔ sqlc conversion lives in `mapper.go`, nowhere else
- **One method per file**: route files contain exactly one handler method

## Layer Rules

- `internal/api/` can depend on: `internal/domain/`, `gen/sdk/`, `gen/db/`, `pkg/connectutil` or `pkg/grpcutil`
- `internal/outbox/river.go` can depend on: `internal/domain/` (for event constants only), `internal/outbox/<domain>/`, `pkg/outbox`, river
- `internal/outbox/<domain>/` can depend on: `pkg/outbox`, river — must NOT import `internal/domain/` or `internal/api/`
- `cmd/` wires everything together

## Post-Generation

1. Run `make vet` — fix all compilation errors
2. Run `make build` — confirm Docker build works
3. Run `make start` — starts infra + server via air, confirm `/health` returns 200
4. Run `make teardown` — stops infra

## Checklist

- [ ] Go package name is `api<domain><version>` (e.g., `apicontentv1`)
- [ ] Import aliases follow convention: `<domain>v1`, `<domain>v1connect`, `db<domain>`, `<domain>domain`, `<domain>events`
- [ ] `handler.go` with Dependencies, constructor, error mappings
- [ ] `mapper.go` with toProto + fromProtoCreate + validateUpdateMask + fromProtoUpdate
- [ ] One `route_*.go` per RPC method
- [ ] UUID parse errors return `InvalidArgument` directly (not via errorMappings)
- [ ] Update route validates id mismatch and update_mask before calling service
- [ ] `internal/outbox/river.go` updated with new event types using domain constants
- [ ] All event types fan out to audit worker (audit all operations)
- [ ] One `event_*.go` per outbox concern, workers accept `ctx`
- [ ] `setup_connections.go` registers new workers
- [ ] `setup_domains.go` wires new domain service
- [ ] `setup_gateway.go` registers new handler + reflection
- [ ] `make vet` passes
- [ ] `make build` succeeds
- [ ] `make start` boots with `/health` returning 200
