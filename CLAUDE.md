# CLAUDE.md

## Project Overview

Connect-RPC backend in Go, following a layered architecture.
See `_architecture/platform-backend.png` for the visual mental model.

## Tech Stack

- **Language**: Go
- **RPC Framework**: Connect-RPC (`connectrpc.com/connect`)
- **Protobuf**: Buf CLI for proto management and code generation
- **SQL**: sqlc for type-safe queries, goose for migrations
- **Task Runner**: Make — all commands go through `Makefile`
- **Service Orchestration**: Docker Compose
- **Integration Tests**: testcontainers-go
- **Outbox**: River (transactional job queue on postgres)
- **Logging**: zerolog
- **Config**: godotenv

## Directory Structure

```
cmd/server/
├── main.go
├── setup_connections.go
├── setup_domains.go
└── setup_gateway.go
internal/
├── api/<domain>/v1/        # handler, mapper, route_<rpc>.go (versioned to match proto package)
├── domain/<domain>/        # service, errors, op_<operation>.go
└── outbox/
    ├── river.go            # River implementation of pkg/outbox.Outbox
    └── <domain>/           # event_<concern>.go per domain
pkg/
├── config/config.go
├── connectapp/app.go
├── connectutil/errors.go
├── connectutil/interceptors.go
├── cache/cache.go
├── outbox/outbox.go
└── migrate/migrate.go
gen/
├── sdk/                    # buf-generated (gitignored)
└── db/<domain>/            # sqlc-generated (gitignored)
sql/
├── migrations/
│   ├── migrations.go       # go:embed for .sql files
│   └── 001_create_<domain>.sql
└── queries/<domain>/<domain>.sql
protos/<domain>/v1/         # .proto files
```

## Conventions

- All tasks run via `make <target>` — never run go/buf/docker commands directly
- **Interface-first**: every package exposes an interface as its public API. Structs are unexported. Constructors return the interface type.
- **Dependencies struct**: each layer defines an exported `Dependencies` struct. Constructors take it as the single parameter. The private struct inlines the fields directly.
- **File prefixes**: `route_<rpc>.go` in api, `op_<operation>.go` in domain, `event_<concern>.go` in outbox.
- **API versioning**: `internal/api/<domain>/v1/` mirrors the proto package `<domain>.v1`.
- **Single server**: one h2c server on `:8080` — `/health` (no interceptors) and Connect RPC paths (with per-handler interceptors).
- Generated code goes to `gen/` (gitignored).
- Use `connect.NewError(connect.CodeXxx, err)` for RPC errors.
- Proto files live under `protos/` with buf module configuration.

## Layer Rules

- `pkg/` depends on nothing — purely generic, extractable as a shared module
- `internal/domain/` depends on `gen/db/` + `pkg/`
- `internal/outbox/` depends on `gen/db/` + `pkg/outbox` + river
- `internal/api/` depends on `internal/domain/`, `gen/sdk/`, `gen/db/`, `pkg/`
- `cmd/` wires all layers together

## gRPC Error Mapping

| gRPC Code           | HTTP | When                           |
|---------------------|------|--------------------------------|
| CANCELED            | 499  | Client cancelled               |
| UNAUTHENTICATED     | 401  | Missing/invalid credentials    |
| INVALID_ARGUMENT    | 400  | Request validation failed      |
| PERMISSION_DENIED   | 403  | Insufficient permissions       |
| NOT_FOUND           | 404  | Resource does not exist        |
| ALREADY_EXISTS      | 409  | Duplicate resource             |
| PRECONDITION_FAILED | 412  | State precondition not met     |
| INTERNAL            | 500  | Unexpected server error        |
| UNAVAILABLE         | 503  | Service dependency unavailable |
| DEADLINE_EXCEEDED   | 504  | Timeout                        |

## Agents

Agents are defined in `.claude/agents/`. Use them via `claude --agent <name>`.

### Build Agents

| Agent | Purpose | PR character |
|-------|---------|--------------|
| `scaffold` | Project skeleton with empty stubs | "Does the structure match our architecture?" |
| `proto` | Proto definition for a domain | "Is the API contract right?" |
| `entity-store` | SQL migration + sqlc queries for a domain | "Is the data model right?" |
| `domain` | Business logic for a domain | "Is the logic correct?" |
| `integrate` | API handler + outbox + wiring for a domain | "Is this wired correctly?" |
| `test` | Unit + integration tests for a domain | "Is this adequately tested?" |

### Review Agents (subagents for PR review sessions)

| Agent | Reviews PRs from | Audit question |
|-------|-----------------|----------------|
| `review-scaffold` | `scaffold` | Does the structure match our architecture? |
| `review-proto` | `proto` | Is the API contract right? |
| `review-entity-store` | `entity-store` | Is the data model right? |
| `review-domain` | `domain` | Is the logic correct? |
| `review-integrate` | `integrate` | Is this wired correctly? |
| `review-test` | `test` | Is this adequately tested? |
