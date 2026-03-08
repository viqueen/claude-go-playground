# Scaffold Agent

Generate the full project skeleton with empty stubs. The PR should compile, boot, and do nothing.
No business logic — just structure.

## What to generate

### Build & Config Files

- `go.mod` — module name based on the repository
- `Makefile` — targets: `codegen`, `tidy`, `vet`, `build`, `test`, `start`, `stop`, `clean`
- `Dockerfile` — multi-stage: generate (buf + sqlc) → build → runtime
- `docker-compose.yml` — postgres, opensearch, opensearch-dashboards, codegen (profile), api
- `buf.gen.yaml` — v2 config with managed mode, `go_package_prefix` set to `<module>/gen/proto`
- `sqlc.yaml` — empty `sql:` list (domains add entries later)
- `.env` — default env vars (DATABASE_URL, OPENSEARCH_URL, SERVER_ADDR)
- `.gitignore` — include `gen/`, `.env`, binaries

### pkg/ — Generic Reusable Packages (interfaces + minimal implementations)

Each package exposes an interface. Implementations are empty stubs or minimal.

- `pkg/config/config.go` — `Config` struct, `Load()` via godotenv
- `pkg/connectapp/app.go` — `App` interface with `Handle()` + `Run()`, h2c server, `/health` endpoint
- `pkg/connectutil/errors.go` — `NewErrorFrom(err, mappings)` helper
- `pkg/connectutil/interceptors.go` — `NewInterceptors()` returning recovery + logging + validate
- `pkg/cache/cache.go` — `Cache[K,V]` interface + in-memory implementation
- `pkg/outbox/outbox.go` — `Outbox[T]` interface + `Event` struct (no implementation yet)
- `pkg/migrate/migrate.go` — goose wrapper with `Run(db, migrations, dir)`

### cmd/server/ — Entry Point with Empty Wiring

- `cmd/server/main.go` — signal context, load config, call setup functions, run app
- `cmd/server/setup_connections.go` — `Connections` struct with `Pool` + `RiverClient`, empty `setupConnections()` that connects to postgres, runs migrations, starts river
- `cmd/server/setup_domains.go` — `Domains` struct (empty), `setupDomains()` returns empty `Domains`
- `cmd/server/setup_gateway.go` — `setupGateway()` creates app with interceptors, no handlers registered yet, includes gRPC reflection

### sql/migrations/

- `sql/migrations/migrations.go` — `go:embed` for `*.sql` files, exports `FS`
- No migration files yet (domains add them)

### internal/ — Empty directories

Create placeholder `.gitkeep` files in:
- `internal/api/`
- `internal/domain/`
- `internal/outbox/`

## Conventions

Follow all conventions from CLAUDE.md:
- Interface-first, Dependencies struct pattern
- Single h2c server on `:8080`
- All env vars in `pkg/config`

## Makefile Targets

```makefile
.PHONY: codegen tidy vet build test start stop clean

codegen:
	docker compose --profile codegen run --rm codegen

tidy: codegen
	go mod tidy

vet: tidy
	go vet ./...

build: vet
	docker compose build api

test: vet
	go test ./...

start:
	docker compose up -d
	@echo "waiting for api to be healthy..."
	@until curl -sf http://localhost:8080/health > /dev/null 2>&1; do sleep 1; done
	@echo "api is up"

stop:
	docker compose down

clean:
	docker compose down -v
	rm -rf gen/
```

## Post-Generation

1. Run `make vet` — fix all errors before proceeding
2. Run `make build` — confirm Docker build works
3. Run `make start` — confirm server boots and `/health` responds
4. Run `make stop`

## Checklist

- [ ] `go.mod` with correct module path
- [ ] `Makefile` with all targets
- [ ] `Dockerfile` multi-stage build
- [ ] `docker-compose.yml` with postgres, opensearch, codegen, api
- [ ] `buf.gen.yaml` with managed mode
- [ ] `sqlc.yaml` (empty sql list)
- [ ] All `pkg/` packages with interfaces and minimal implementations
- [ ] `cmd/server/` with main + setup stubs
- [ ] `sql/migrations/migrations.go` with embed
- [ ] `.gitignore` covers `gen/` and `.env`
- [ ] `make vet` passes
- [ ] `make build` succeeds
- [ ] `make start` boots, `/health` returns 200
