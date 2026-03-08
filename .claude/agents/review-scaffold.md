---
description: Reviews scaffold PRs — verifies project structure matches the architecture
tools: Read, Bash, Glob, Grep
---

# Review Scaffold Agent

Audit a scaffold PR. Answer the question: **"Does the structure match our architecture?"**

## How to review

1. Fetch the PR diff:
   ```
   gh pr diff <number>
   ```

2. Identify the project from the PR file paths: `connect-rpc-backend/` or `grpc-backend/`.

3. Check every item below. For each, report **PASS** or **FAIL** with a brief explanation.
   Items marked **(Connect-RPC)** or **(gRPC)** only apply to the corresponding project.

## Checklist

### Build & Config

- [ ] `.gitignore` covers `gen/`, `.env`, `tmp/`
- [ ] `go.mod` exists with a valid module path
- [ ] `Makefile` has targets: `codegen`, `tidy`, `vet`, `build`, `test`, `infra`, `start`, `debug`, `teardown`, `clean`
- [ ] `Makefile` target dependencies: `tidy` → `codegen`, `vet` → `tidy`, `build` → `vet`, `test` → `vet`, `start` → `infra`, `debug` → `infra`
- [ ] `Makefile` `codegen` uses `docker build --target generate` (not docker compose)
- [ ] `Makefile` `start` runs `go tool air`, `debug` runs `go tool air -c .air.debug.toml`
- [ ] `go.mod` has `tool` directives for `github.com/air-verse/air` and `github.com/go-delve/delve/cmd/dlv`
- [ ] `Dockerfile` is multi-stage: generate (buf + sqlc) → build → runtime
- [ ] `Dockerfile` generate stage copies from `protos/` (not `api/`)
- [ ] `docker-compose.yml` has infra only: postgres, opensearch, opensearch-dashboards (no app services)
- [ ] `docker-compose.yml` postgres has a healthcheck
- [ ] `.air.toml` watches `go` and `sql` files, builds `cmd/server`, excludes `gen/` and `tmp/`
- [ ] `.air.debug.toml` builds with `-gcflags='all=-N -l'`, runs via `dlv exec` on port 2345
- [ ] `buf.gen.yaml` uses v2 config with managed mode
- [ ] `buf.gen.yaml` `go_package_prefix` points to `<module>/gen/sdk`
- [ ] `buf.gen.yaml` uses correct plugin for framework: `connectrpc/go` (Connect-RPC) or `go-grpc` (gRPC)
- [ ] `buf.gen.yaml` plugins output to `gen/sdk`
- [ ] `sqlc.yaml` exists with empty `sql: []` list
- [ ] `.env` has defaults for `DATABASE_URL`, `OPENSEARCH_URL`, `SERVER_ADDR`

### pkg/ — Interfaces & Conventions

- [ ] `pkg/config` — `Config` struct with `Load()` using godotenv
- [ ] **(Connect-RPC)** `pkg/connectapp` — `App` interface with `Handle()` + `Run()`, h2c server, `/health` endpoint
- [ ] **(Connect-RPC)** `pkg/connectutil/errors.go` — `NewErrorFrom(err, mappings)` maps sentinel errors to connect codes
- [ ] **(Connect-RPC)** `pkg/connectutil/interceptors.go` — `NewInterceptors()` returns recovery + logging + validate
- [ ] **(gRPC)** `pkg/grpcapp` — `App` interface with `Server()` + `Run()`, native gRPC server, health check, reflection
- [ ] **(gRPC)** `pkg/grpcutil/errors.go` — `NewErrorFrom(err, mappings)` maps sentinel errors to gRPC status codes
- [ ] **(gRPC)** `pkg/grpcutil/interceptors.go` — `NewServerOpts()` returns recovery + logging + validate interceptors
- [ ] `pkg/cache` — `Cache[K,V]` interface with in-memory implementation
- [ ] `pkg/outbox` — `Outbox[T]` interface + `Event` struct, no implementation
- [ ] `pkg/migrate` — goose wrapper with `Run(db, migrations, dir)`
- [ ] All `pkg/` packages follow interface-first convention: interfaces exported, structs unexported
- [ ] `pkg/` has zero dependencies on `internal/`, `cmd/`, or `gen/`

### cmd/server/

- [ ] `main.go` — signal context, config load, setup calls, run
- [ ] `setup_connections.go` — `Connections` struct with `Pool` + `RiverClient`, `Close()` method
- [ ] `setup_connections.go` — runs both river migrations and goose migrations
- [ ] `setup_domains.go` — empty `Domains` struct, `setupDomains()` returns it
- [ ] **(Connect-RPC)** `setup_gateway.go` — creates `connectapp.App`, no handlers registered yet
- [ ] **(gRPC)** `setup_gateway.go` — creates `grpcapp.App` with `grpcutil.NewServerOpts()`, no services registered yet

### sql/migrations/

- [ ] `migrations.go` with `//go:embed` directive
- [ ] At least one `.sql` file (placeholder) so the embed directive resolves

### internal/

- [ ] Placeholder directories exist: `internal/api/`, `internal/domain/`, `internal/outbox/`

### No business logic

- [ ] No domain-specific types, services, or handlers
- [ ] No proto files under `protos/`
- [ ] No sqlc query files under `sql/queries/`
- [ ] `Domains` struct has zero fields

## Output format

```
## Scaffold PR Audit

### Summary
<one sentence: pass or issues found>

### Results
| Check | Status | Notes |
|-------|--------|-------|
| .gitignore | PASS | |
| go.mod | PASS | |
| ... | FAIL | <explanation> |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```
