# claude-connect-rpc-playground

A Connect-RPC backend in Go, built incrementally using Claude Code agents.

## Architecture

See [`_architecture/platform-backend.png`](_architecture/platform-backend.png) for the visual mental model.

```
cmd/server/              # entry point — server bootstrap, wiring
internal/
├── api/<domain>/v1/     # handlers, mappers, routes (versioned to match proto)
├── domain/<domain>/     # business logic, operations, errors
└── outbox/<domain>/     # async event workers
pkg/                     # shared utilities — config, cache, connectapp, etc.
protos/<domain>/v1/      # protobuf definitions
sql/                     # migrations + sqlc queries
gen/
├── sdk/                 # buf-generated proto + connect stubs
└── db/<domain>/         # sqlc-generated query code
```

## Tech Stack

- [Go](https://go.dev/doc/)
- [Connect-RPC](https://connectrpc.com/docs/go/getting-started)
- [Buf](https://buf.build/docs/)
- [sqlc](https://docs.sqlc.dev/)
- [goose](https://pressly.github.io/goose/)
- [River](https://riverqueue.com/docs)
- [testcontainers-go](https://golang.testcontainers.org/)
- [Docker Compose](https://docs.docker.com/compose/)
- [Make](https://www.gnu.org/software/make/manual/make.html)

## Agents

Each agent produces a small, auditable PR:

### Build Agents

Each produces a focused, auditable PR:

| Agent | `claude --agent <name>` | PR audit question |
|-------|------------------------|-------------------|
| **scaffold** | `claude --agent scaffold` | Does the structure match our architecture? |
| **proto** | `claude --agent proto` | Is the API contract right? |
| **entity-store** | `claude --agent entity-store` | Is the data model right? |
| **domain** | `claude --agent domain` | Is the logic correct? |
| **integrate** | `claude --agent integrate` | Is this wired correctly? |
| **test** | `claude --agent test` | Is this adequately tested? |

### Review Agents

Subagents invoked during PR review sessions to audit changes:

| Agent | Reviews PRs from | Audit output |
|-------|-----------------|--------------|
| **review-scaffold** | scaffold | Structure & conventions checklist |
| **review-proto** | proto | API contract & validation annotations |
| **review-entity-store** | entity-store | Schema, queries, proto ↔ SQL consistency |
| **review-domain** | domain | Logic, layer rules, transaction patterns |
| **review-integrate** | integrate | Wiring, route coverage, outbox events |
| **review-test** | test | Coverage matrix, testcontainers usage |

### Workflow

```mermaid
graph LR
    scaffold --> proto
    scaffold --> entity-store
    proto --> integrate
    entity-store --> domain
    domain --> integrate
    integrate --> test
    test -.->|next domain| proto
    test -.->|next domain| entity-store
```

## Getting Started

```bash
make vet      # codegen + tidy + go vet
make build    # docker build
make start    # docker compose up + health check
make test     # unit + integration tests
make stop     # tear down
```