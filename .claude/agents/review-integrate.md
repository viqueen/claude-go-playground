---
description: Reviews integrate PRs — verifies API handler, outbox, and server wiring correctness
tools: Read, Bash, Glob, Grep
---

# Review Integrate Agent

Audit an integrate PR. Answer the question: **"Is this wired correctly?"**

## How to review

1. Fetch the PR diff:
   ```
   gh pr diff <number>
   ```

2. Identify the domain being integrated and read the full files (not just the diff).

3. Identify the project from the PR file paths: `connect-rpc-backend/` or `grpc-backend/`.

4. Check every item below. For each, report **PASS** or **FAIL** with a brief explanation.
   Items marked **(Connect-RPC)** or **(gRPC)** only apply to the corresponding project.

## Checklist

### File Structure — `internal/api/<domain>/v1/`

- [ ] `handler.go` exists with `Dependencies`, constructor, and error mappings
- [ ] `mapper.go` exists with `toProto`, `fromProtoCreate`, `fromProtoUpdate` functions
- [ ] One `route_<rpc>.go` file per RPC method
- [ ] Package path is versioned: `internal/api/<domain>/v1/` (matches proto package `<domain>.v1`)

### API Versioning

- [ ] Go package lives under `internal/api/<domain>/v1/`
- [ ] Imports generated code from `gen/sdk/<domain>/v1/`
- [ ] **(Connect-RPC)** Handler implements the v1 connect service interface
- [ ] **(gRPC)** Handler embeds `Unimplemented<Service>Server` and implements the v1 gRPC service interface

### Package & Import Conventions

- [ ] Go package name is `api<domain><version>` (e.g., `apicontentv1`)
- [ ] Import alias `<domain>v1` for proto types (e.g., `contentv1`)
- [ ] **(Connect-RPC)** Import alias `<domain>v1connect` for connect service (e.g., `contentv1connect`)
- [ ] **(gRPC)** Import alias `<domain>v1grpc` for gRPC service (e.g., `contentv1grpc`)
- [ ] Import alias `db<domain>` for sqlc types (e.g., `dbcontent`)
- [ ] Import alias `<domain>domain` for domain service (e.g., `contentdomain`)

### Handler Convention

- [ ] `Dependencies` struct has a `Service` field typed to the domain `Service` interface
- [ ] **(Connect-RPC)** `New()` returns the connect-generated handler interface (not a concrete struct)
- [ ] **(gRPC)** `New()` returns the gRPC-generated service server interface (not a concrete struct)
- [ ] `handler` struct is unexported
- [ ] `handler` struct has a `service` field (inlined, not embedded Dependencies)
- [ ] **(Connect-RPC)** `errorMappings` maps domain errors → `connect.Code`
- [ ] **(gRPC)** `errorMappings` maps domain errors → `codes.Code`

### Mapper

- [ ] `toProto()` maps sqlc model → proto response message
- [ ] `fromProtoCreate()` maps proto create request → sqlc create params
- [ ] `validateUpdateMask()` rejects empty masks and unsupported field paths
- [ ] `fromProtoUpdate()` maps proto update request → sqlc update params
- [ ] Update mapper uses `pgtype.Text{String: val, Valid: true}` for nullable strings (not `*string`)
- [ ] Update mapper uses `pgtype.Int4{Int32: val, Valid: true}` for nullable ints (not `*int32`)
- [ ] All proto ↔ sqlc conversion is isolated in `mapper.go` (no mapping in route files)

### Route Files

- [ ] Each `route_*.go` contains exactly one handler method
- [ ] Route methods call domain service (not sqlc directly)
- [ ] **(Connect-RPC)** Route methods use `connectutil.NewErrorFrom(err, errorMappings)` for domain errors
- [ ] **(gRPC)** Route methods use `grpcutil.NewErrorFrom(err, errorMappings)` for domain errors
- [ ] UUID parse errors return `InvalidArgument` directly (not via errorMappings which falls through to Internal)
- [ ] Update route rejects mismatched resource id vs request id with `InvalidArgument`
- [ ] Update route calls `validateUpdateMask()` before `fromProtoUpdate()`
- [ ] Route methods use mapper functions for proto ↔ sqlc conversion
- [ ] Every RPC in the proto service has a corresponding route file

### Outbox — `internal/outbox/`

- [ ] `river.go` exists with `NewRiverOutbox` constructor returning `outbox.Outbox[pgx.Tx]`
- [ ] Import alias `<domain>domain` for event constants, `<domain>events` for workers
- [ ] `mapEvent()` uses shared constants from `<domain>domain.Event*` (no hardcoded strings)
- [ ] `mapEvent()` has cases for all event types emitted by the domain (created, updated, deleted, etc.)
- [ ] Every event type fans out to the audit worker (all operations are auditable)
- [ ] No unhandled event types (default case returns an error)

### Outbox Workers — `internal/outbox/<domain>/`

- [ ] One `event_<concern>.go` per concern (index, audit, etc.)
- [ ] Each file has `JobArgs` struct with `Kind()` method
- [ ] Each file has `Worker` struct embedding `river.WorkerDefaults`
- [ ] Workers accept `ctx context.Context` (not `_`) and use `log.Ctx(ctx)` for logging
- [ ] `NewXxxArgs()` constructor maps from `outbox.Event`
- [ ] Job `Kind()` follows `<domain>.<concern>` naming

### Server Wiring — `cmd/server/`

- [ ] `setup_connections.go` — new workers registered with `river.AddWorker(workers, ...)`
- [ ] `setup_domains.go` — `Domains` struct has new field for this domain's `Service`
- [ ] `setup_domains.go` — service wired with correct dependencies (pool, queries, cache, outbox)
- [ ] `setup_gateway.go` — handler created via `New(Dependencies{...})`
- [ ] **(Connect-RPC)** `setup_gateway.go` — handler registered with `connect.WithInterceptors(interceptors...)`
- [ ] **(Connect-RPC)** `setup_gateway.go` — handler path registered on connectapp mux
- [ ] **(gRPC)** `setup_gateway.go` — service registered via `Register<Service>Server(application.Server(), handler)`

### Layer Rules — Imports

- [ ] `internal/api/<domain>/v1/` imports: `internal/domain/<domain>`, `gen/sdk/`, `gen/db/`, `pkg/connectutil` or `pkg/grpcutil` — ALLOWED
- [ ] `internal/outbox/river.go` imports `internal/domain/<domain>` for event constants only — ALLOWED
- [ ] `internal/outbox/<domain>/` imports: `pkg/outbox`, river — ALLOWED
- [ ] `internal/outbox/<domain>/` does NOT import `internal/domain/` or `internal/api/`
- [ ] `internal/api/<domain>/v1/` does NOT import `internal/outbox/`

## Output format

```
## Integrate PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### Wiring Matrix
| Component | File | Registered | Dependencies | Status |
|-----------|------|------------|-------------|--------|
| Handler | setup_gateway.go | yes | Service | PASS |
| Service | setup_domains.go | yes | Pool, Queries, Cache, Outbox | PASS |
| IndexWorker | setup_connections.go | yes | — | PASS |
| AuditWorker | setup_connections.go | yes | — | PASS |

### Route Coverage
| Proto RPC | Route File | Uses Mapper | Uses ErrorMappings | Status |
|-----------|------------|-------------|-------------------|--------|
| Create | route_create.go | yes | yes | PASS |
| ... | ... | ... | ... | ... |

### Outbox Event Coverage
| Domain Event | Workers Triggered | Status |
|-------------|-------------------|--------|
| <domain>.created | index, audit | PASS |
| ... | ... | ... |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```
