# CLAUDE.md

## Project Overview

RPC backend in Go (Connect-RPC or gRPC), following a layered architecture.
See `_architecture/platform-backend.png` for the visual mental model.

## Tech Stack

- **Language**: Go
- **RPC Framework**: Connect-RPC (`connectrpc.com/connect`) or gRPC (`google.golang.org/grpc`) — chosen per project at scaffold time
- **Protobuf**: Buf CLI for proto management and code generation
- **SQL**: sqlc for type-safe queries, goose for migrations
- **Task Runner**: Make — all commands go through `Makefile`
- **Service Orchestration**: Docker Compose
- **Integration Tests**: testcontainers-go
- **Outbox**: River (transactional job queue on postgres)
- **Logging**: zerolog
- **Config**: godotenv

## Repo Layout

This repo contains two independent Go projects, one per RPC framework:

```
connect-rpc-backend/        # Connect-RPC project
grpc-backend/               # gRPC project
```

All agents write code into the chosen project root. The user specifies which project
when invoking an agent. All `make` commands must be run from inside the project folder.

## Project Directory Structure

Each project (`connect-rpc-backend/` or `grpc-backend/`) follows this layout:

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
    └── <domain>/           # event_<concern>.go, index.go, document.go per domain
        └── mappings/       # //go:embed *.json for OpenSearch index mappings
pkg/
├── config/config.go
├── connectapp/app.go        # Connect-RPC project only
├── connectutil/errors.go    # Connect-RPC project only
├── connectutil/interceptors.go  # Connect-RPC project only
├── grpcapp/app.go           # gRPC project only
├── grpcutil/errors.go       # gRPC project only
├── grpcutil/interceptors.go # gRPC project only
├── cache/cache.go
├── outbox/outbox.go
├── search/search.go         # generic OpenSearch client interface (no domain knowledge)
├── pagination/pagination.go
├── migrate/migrate.go
└── testkit/containers.go
gen/
├── sdk/                    # buf-generated (gitignored)
└── db/<schema>/            # sqlc-generated (gitignored), grouped by schema (e.g. collaboration)
sql/
├── migrations/
│   ├── migrations.go       # go:embed for .sql files
│   └── 0001_create_<schema>.sql
└── queries/<schema>/       # sqlc queries grouped by schema (e.g. collaboration/space.sql)
protos/<domain>/v1/         # .proto files
```

## Conventions

- All tasks run via `make <target>` from the project root — never run go/buf/docker commands directly
- `make infra` starts infrastructure (docker compose), `make start` starts the server locally via air, `make debug` starts with delve
- `make codegen` uses `docker build --target generate` to run buf + sqlc in a container
- **Interface-first**: every package exposes an interface as its public API. Structs are unexported. Constructors return the interface type.
- **Dependencies struct**: each layer defines an exported `Dependencies` struct. Constructors take it as the single parameter. The private struct inlines the fields directly.
- **File prefixes**: `route_<rpc>.go` in api, `op_<operation>.go` in domain, `event_<concern>.go` in outbox.
- **API versioning**: `internal/api/<domain>/v1/` mirrors the proto package `<domain>.v1`.
- **Single server**: one server on `:8080` — `/health` (no interceptors) and RPC paths (with per-handler interceptors). Connect-RPC uses h2c; gRPC uses native gRPC server with a health endpoint.
- Generated code goes to `gen/` (gitignored).
- Connect-RPC: use `connect.NewError(connect.CodeXxx, err)` for RPC errors.
- gRPC: use `status.Errorf(codes.Xxx, msg)` for RPC errors.
- Proto files live under `protos/` with buf module configuration.
- **No magic values**: never hardcode protocol/database constants as raw literals. Use named constants from well-known libraries (e.g., `pgerrcode.UniqueViolation` not `"23505"`, `codes.NotFound` not `5`). If no library constant exists, define a named constant with a doc reference to the spec.

## Layer Rules

- `pkg/` depends on nothing — purely generic, extractable as a shared module
- `internal/domain/` depends on `gen/db/` + `pkg/`
- `internal/outbox/` depends on `gen/db/` + `pkg/outbox` + `pkg/search` + river
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

## Skills

Skills are defined in `.claude/skills/` as slash commands. Each skill runs in a forked context
with self-contained instructions (no separate agent files).

### Build Skills

| Skill | Usage | PR character |
|-------|-------|--------------|
| `/do-scaffold` | `/do-scaffold <project>` | "Does the structure match our architecture?" |
| `/do-proto` | `/do-proto <domain> <project>` | "Is the API contract right?" |
| `/do-entity-store` | `/do-entity-store <domain> <project>` | "Is the data model right?" |
| `/do-domain` | `/do-domain <domain> <project>` | "Is the logic correct?" |
| `/do-search` | `/do-search <domain> <project>` | "Is the search indexing correct?" |
| `/do-integrate` | `/do-integrate <domain> <project>` | "Is this wired correctly?" |
| `/do-test` | `/do-test <domain> <project>` | "Is this adequately tested?" |

### Review Skills

| Skill | Usage | Audit question |
|-------|-------|----------------|
| `/review-scaffold` | `/review-scaffold <pr-number>` | Does the structure match our architecture? |
| `/review-proto` | `/review-proto <pr-number>` | Is the API contract right? |
| `/review-entity-store` | `/review-entity-store <pr-number>` | Is the data model right? |
| `/review-domain` | `/review-domain <pr-number>` | Is the logic correct? |
| `/review-search` | `/review-search <pr-number>` | Is the search indexing correct? |
| `/review-integrate` | `/review-integrate <pr-number>` | Is this wired correctly? |
| `/review-test` | `/review-test <pr-number>` | Is this adequately tested? |
