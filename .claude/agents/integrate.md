# Integrate Agent

Wire a domain into the server — API handler, outbox workers, and cmd/server registration.
This PR is auditable as: **"Is this wired correctly?"**

Depends on: `domain` agent PR (internal/domain/<domain>/ must exist).

## Inputs

The user will specify:
- The **domain name** (e.g., `content`)
- Any specific mapping concerns (e.g., custom proto ↔ sqlc field mappings)

## What to generate

### 1. `internal/api/<domain>/v1/handler.go`

Private struct implementing the Connect-generated service interface:

```go
package <domain>

type Dependencies struct {
    Service <domain>domain.Service
}

func New(deps Dependencies) <domain>v1connect.<Resource>ServiceHandler {
    return &handler{service: deps.Service}
}

type handler struct {
    service <domain>domain.Service
}

var errorMappings = map[error]connect.Code{
    <domain>domain.ErrNotFound:      connect.CodeNotFound,
    <domain>domain.ErrAlreadyExists: connect.CodeAlreadyExists,
}
```

### 2. `internal/api/<domain>/v1/mapper.go`

Mapping functions between proto types and sqlc models:

- `toProto(model) *proto.Resource` — sqlc model → proto response
- `fromProtoCreate(msg) sqlcparams` — proto create request → sqlc create params
- `fromProtoUpdate(msg) sqlcparams` — proto update request → sqlc update params
  - Uses `pgtype.Text{String: val, Valid: true}` for nullable string fields from `sqlc.narg()`
  - Uses `pgtype.Int4{Int32: val, Valid: true}` for nullable int fields

### 3. `internal/api/<domain>/v1/route_<rpc>.go` — One file per RPC

Each file contains a single method on the handler:

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

### 4. `internal/outbox/river.go` — Update event mapping

If this is the first domain, create `internal/outbox/river.go` with the `NewRiverOutbox` constructor
and `mapEvent` switch. If it already exists, add cases for the new domain's event types.

### 5. `internal/outbox/<domain>/event_<concern>.go` — One file per concern

Each file contains river `JobArgs` + `Worker` for a specific concern:

- `event_index.go` — indexing concern
- `event_audit.go` — auditing concern
- Add more as needed (analytics, graph, etc.)

### 6. `cmd/server/` — Wiring updates

Update the setup files to register the new domain:

- `setup_connections.go` — register outbox workers with river (`river.AddWorker`)
- `setup_domains.go` — add domain to `Domains` struct, wire service with dependencies
- `setup_gateway.go` — register Connect handler with interceptors, add to reflection

## Conventions

- **API versioning**: `internal/api/<domain>/v1/` mirrors the proto package `<domain>.v1`. When a v2 proto is introduced, handlers go under `internal/api/<domain>/v2/`.
- **File prefixes**: `route_<rpc>.go` in api, `event_<concern>.go` in outbox
- **Error mappings**: defined as package-level var in `handler.go`, used by all routes via `connectutil.NewErrorFrom`
- **Mapper isolation**: all proto ↔ sqlc conversion lives in `mapper.go`, nowhere else
- **One method per file**: route files contain exactly one handler method

## Layer Rules

- `internal/api/` can depend on: `internal/domain/`, `gen/sdk/`, `gen/db/`, `pkg/connectutil`
- `internal/outbox/` can depend on: `gen/db/`, `pkg/outbox`, river
- `cmd/` wires everything together

## Post-Generation

1. Run `make vet` — fix all compilation errors
2. Run `make build` — confirm Docker build works
3. Run `make start` — confirm server boots with new handler registered
4. Run `make stop`

## Checklist

- [ ] `handler.go` with Dependencies, constructor, error mappings
- [ ] `mapper.go` with toProto + fromProtoCreate + fromProtoUpdate
- [ ] One `route_*.go` per RPC method
- [ ] `internal/outbox/river.go` updated with new event types
- [ ] One `event_*.go` per outbox concern
- [ ] `setup_connections.go` registers new workers
- [ ] `setup_domains.go` wires new domain service
- [ ] `setup_gateway.go` registers new handler + reflection
- [ ] `make vet` passes
- [ ] `make build` succeeds
- [ ] `make start` boots with `/health` returning 200
