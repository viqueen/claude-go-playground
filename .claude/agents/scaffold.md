# Scaffold Agent

Generate the full project skeleton with empty stubs. The PR should compile, boot, and do nothing.
No business logic — just structure.

## What to generate

### .gitignore

```gitignore
gen/
.env
/server
```

### go.mod

Module name based on the repository (e.g., `github.com/<org>/<repo>`).
Run `go mod init <module>` then add dependencies with `go get`.

### buf.gen.yaml

Uses buf v2 config with managed mode to rewrite `go_package` imports to match the Go module path.

```yaml
version: v2
managed:
  enabled: true
  disable:
    - file_option: go_package
      module: buf.build/bufbuild/protovalidate
  override:
    - file_option: go_package_prefix
      value: <module>/gen/sdk
plugins:
  - protoc_builtin: go
    out: gen/sdk
    opt: paths=source_relative
  - remote: buf.build/connectrpc/go
    out: gen/sdk
    opt: paths=source_relative
```

Replace `<module>` with the actual Go module name.

### sqlc.yaml

Empty sql list — domains add entries later via the `proto` agent.

```yaml
version: "2"
sql: []
```

### .env

```env
DATABASE_URL=postgres://playground:playground@localhost:5432/playground?sslmode=disable
OPENSEARCH_URL=http://localhost:9200
SERVER_ADDR=:8080
```

### Makefile

```makefile
.PHONY: codegen tidy vet build test start stop clean

# --- codegen ---

codegen:
	docker compose --profile codegen run --rm codegen

tidy: codegen
	go mod tidy

# --- checks ---

vet: tidy
	go vet ./...

build: vet
	docker compose build api

test: vet
	go test ./...

# --- run ---

start:
	docker compose up -d
	@echo "waiting for api to be healthy..."
	@until curl -sf http://localhost:8080/health > /dev/null 2>&1; do sleep 1; done
	@echo "api is up"

stop:
	docker compose down

# --- cleanup ---

clean:
	docker compose down -v
	rm -rf gen/
```

### Dockerfile

Multi-stage build: generate (buf + sqlc) → build → runtime.

```dockerfile
# Stage 1: Generate proto + sqlc code
FROM golang:1.24-alpine AS generate

RUN apk add --no-cache git
RUN go install github.com/bufbuild/buf/cmd/buf@latest
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

WORKDIR /build

# buf generate
COPY buf.gen.yaml ./
COPY protos/ protos/
RUN buf dep update protos/
RUN buf generate protos/

# sqlc generate
COPY sqlc.yaml ./
COPY sql ./sql
RUN sqlc generate

# Stage 2: Build
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
COPY --from=generate /build/gen ./gen
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

# Stage 3: Runtime
FROM alpine:3.21

RUN apk add --no-cache curl
COPY --from=builder /server /server

EXPOSE 8080
CMD ["/server"]
```

### docker-compose.yml

```yaml
services:
  postgres:
    image: postgres:17-alpine
    environment:
      POSTGRES_DB: playground
      POSTGRES_USER: playground
      POSTGRES_PASSWORD: playground
    ports:
      - "5432:5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U playground"]
      interval: 2s
      timeout: 2s
      retries: 10

  opensearch:
    image: opensearchproject/opensearch:2.19.1
    environment:
      - discovery.type=single-node
      - DISABLE_SECURITY_PLUGIN=true
    ports:
      - "9200:9200"

  opensearch-dashboards:
    image: opensearchproject/opensearch-dashboards:2.19.1
    environment:
      - OPENSEARCH_HOSTS=["http://opensearch:9200"]
      - DISABLE_SECURITY_DASHBOARDS_PLUGIN=true
    ports:
      - "5601:5601"
    depends_on:
      - opensearch

  codegen:
    build:
      context: .
      dockerfile: Dockerfile
      target: generate
    volumes:
      - ./gen:/out/gen
    entrypoint: ["cp", "-r", "/build/gen/.", "/out/gen/"]
    profiles:
      - codegen

  api:
    build:
      context: .
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://playground:playground@postgres:5432/playground?sslmode=disable
      - OPENSEARCH_URL=http://opensearch:9200
    depends_on:
      postgres:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 10s
```

---

## pkg/ — Generic Reusable Packages

Each package exposes an interface as its public API. Structs are unexported.
Constructors return the interface type.

### pkg/config/config.go

Env-based configuration loaded via godotenv. Consolidates all environment variables
into a typed struct. Loaded once at startup in `main.go`.

```go
package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL   string
	OpenSearchURL string
	ServerAddr    string
}

func Load() (*Config, error) {
	_ = godotenv.Load() // .env file is optional, env vars take precedence
	return &Config{
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://playground:playground@localhost:5432/playground?sslmode=disable"),
		OpenSearchURL: getEnv("OPENSEARCH_URL", "http://localhost:9200"),
		ServerAddr:    getEnv("SERVER_ADDR", ":8080"),
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

### pkg/connectapp/app.go

Reusable Connect RPC application lifecycle. Single server with h2c, path-based routing,
graceful shutdown. Health and API handlers served from different paths on the same port.

```go
package connectapp

import (
	"context"
	"net/http"

	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// App is the public interface for the Connect RPC application.
type App interface {
	Handle(path string, handler http.Handler)
	Run(ctx context.Context) error
}

type Option func(*app)

func WithAddr(addr string) Option { return func(a *app) { a.addr = addr } }

func New(opts ...Option) App {
	a := &app{addr: ":8080", mux: http.NewServeMux()}
	for _, o := range opts {
		o(a)
	}
	a.mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"up"}`))
	})
	return a
}

type app struct {
	addr string
	mux  *http.ServeMux
}

func (a *app) Handle(path string, handler http.Handler) {
	a.mux.Handle(path, handler)
}

func (a *app) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:    a.addr,
		Handler: h2c.NewHandler(a.mux, &http2.Server{}),
	}

	log.Info().Str("addr", a.addr).Msg("server started")

	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()

	select {
	case <-ctx.Done():
		return server.Close()
	case err := <-errCh:
		return err
	}
}
```

### pkg/connectutil/errors.go

Map domain sentinel errors to connect error codes.

```go
package connectutil

import (
	"errors"

	"connectrpc.com/connect"
)

func NewErrorFrom(err error, mappings map[error]connect.Code) *connect.Error {
	for sentinel, code := range mappings {
		if errors.Is(err, sentinel) {
			return connect.NewError(code, err)
		}
	}
	return connect.NewError(connect.CodeInternal, err)
}
```

### pkg/connectutil/interceptors.go

Shared interceptors: recovery, logging, buf validate.

```go
package connectutil

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/rs/zerolog/log"
)

func NewInterceptors() []connect.Interceptor {
	validateInterceptor := validate.NewInterceptor()
	return []connect.Interceptor{
		NewRecoveryInterceptor(),
		NewLoggingInterceptor(),
		validateInterceptor,
	}
}

func NewRecoveryInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (resp connect.AnyResponse, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = connect.NewError(connect.CodeInternal, fmt.Errorf("panic: %v", r))
				}
			}()
			return next(ctx, req)
		}
	}
}

func NewLoggingInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			evt := log.Info()
			if err != nil {
				evt = log.Error().Err(err)
			}
			evt.
				Str("procedure", req.Spec().Procedure).
				Dur("duration", time.Since(start)).
				Msg("rpc")
			return resp, err
		}
	}
}
```

### pkg/cache/cache.go

Generic cache interface with in-memory implementation.

```go
package cache

import (
	"sync"
	"time"
)

// Cache is the public interface. Implementations are private.
type Cache[K comparable, V any] interface {
	Get(key K) (V, bool)
	Set(key K, value V, ttl time.Duration)
	Delete(key K)
}

// NewInMemory returns a Cache backed by a sync.RWMutex map.
func NewInMemory[K comparable, V any]() Cache[K, V] {
	return &inMemory[K, V]{items: make(map[K]entry[V])}
}

type inMemory[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]entry[V]
}

type entry[V any] struct {
	value     V
	expiresAt time.Time
}

func (c *inMemory[K, V]) Get(key K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[key]
	if !ok || (!e.expiresAt.IsZero() && time.Now().After(e.expiresAt)) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *inMemory[K, V]) Set(key K, value V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	c.items[key] = entry[V]{value: value, expiresAt: exp}
}

func (c *inMemory[K, V]) Delete(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}
```

### pkg/outbox/outbox.go

Outbox interface for emitting domain events. The domain doesn't know about jobs or queues —
it just emits events. The implementation decides what to do.

```go
package outbox

import "context"

// Event represents a domain event to be processed asynchronously.
type Event struct {
	Type string
	ID   string
	Data any
}

// Outbox emits domain events within a transaction.
// Generic over the transaction type to avoid unsafe casts while keeping pkg dependency-free.
type Outbox[T any] interface {
	Emit(ctx context.Context, tx T, events ...Event) error
}
```

### pkg/migrate/migrate.go

Goose wrapper with embedded migrations.

```go
package migrate

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

func Run(db *sql.DB, migrations embed.FS, dir string) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(db, dir)
}
```

---

## sql/migrations/

### sql/migrations/migrations.go

Embed directory for goose migration files. No migration files yet — domains add them.

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

Note: `go:embed *.sql` requires at least one `.sql` file to exist. Create an empty
placeholder `sql/migrations/.gitkeep` and use `//go:embed` with `all:` prefix if needed,
or create a `000_init.sql` no-op migration:

```sql
-- +goose Up
-- initial migration placeholder

-- +goose Down
```

---

## cmd/server/ — Entry Point with Empty Wiring

### cmd/server/main.go

```go
package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"<module>/pkg/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	connections := setupConnections(ctx, cfg)
	defer connections.Close(ctx)

	domains := setupDomains(connections)
	application := setupGateway(cfg, domains)

	if err := application.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}
```

Replace `<module>` with the actual Go module name.

### cmd/server/setup_connections.go

Establishes infrastructure connections: database pool, migrations, river client.

```go
package main

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/rs/zerolog/log"

	_ "github.com/jackc/pgx/v5/stdlib"

	"<module>/pkg/config"
	"<module>/pkg/migrate"
	migrations "<module>/sql/migrations"
)

type Connections struct {
	Pool        *pgxpool.Pool
	RiverClient *river.Client[pgx.Tx]
}

func (c *Connections) Close(ctx context.Context) {
	if err := c.RiverClient.Stop(ctx); err != nil {
		log.Error().Err(err).Msg("failed to stop river client")
	}
	c.Pool.Close()
}

func setupConnections(ctx context.Context, cfg *config.Config) *Connections {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}

	// River migrations
	riverMigrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create river migrator")
	}
	if _, err := riverMigrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		log.Fatal().Err(err).Msg("failed to run river migrations")
	}

	// Domain migrations (goose)
	stdDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open sql connection")
	}
	if err := migrate.Run(stdDB, migrations.FS, "."); err != nil {
		log.Fatal().Err(err).Msg("failed to run domain migrations")
	}
	stdDB.Close()

	// River client — no workers registered yet, domains add them via the integrate agent
	workers := river.NewWorkers()

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 100}},
		Workers: workers,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create river client")
	}
	if err := riverClient.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start river client")
	}

	return &Connections{Pool: pool, RiverClient: riverClient}
}
```

### cmd/server/setup_domains.go

Empty for now — domains are added by the `integrate` agent.

```go
package main

// Domains holds all domain services. Fields are added by the integrate agent.
type Domains struct{}

func setupDomains(_ *Connections) *Domains {
	return &Domains{}
}
```

### cmd/server/setup_gateway.go

Creates the app with interceptors. No handlers registered yet — domains add them
via the `integrate` agent.

```go
package main

import (
	"<module>/pkg/config"
	"<module>/pkg/connectapp"
)

func setupGateway(cfg *config.Config, _ *Domains) connectapp.App {
	application := connectapp.New(connectapp.WithAddr(cfg.ServerAddr))

	// Handlers are registered here by the integrate agent.
	// Each domain adds its Connect handler with interceptors.

	return application
}
```

---

## internal/ — Empty Directories

Create placeholder `.gitkeep` files in:
- `internal/api/.gitkeep`
- `internal/domain/.gitkeep`
- `internal/outbox/.gitkeep`

---

## Post-Generation

After writing all files, run through these steps in order. Fix any errors before proceeding.

1. **`make vet`** — generates code (codegen), resolves dependencies (tidy), then runs `go vet ./...`. Fix all issues before continuing.
2. **`make build`** — builds the Docker image end-to-end. Confirms the full build pipeline works.
3. **`make start`** — starts all services and waits for health check. Confirms the server boots.
4. **`make stop`** — tears down services.

If `make vet` fails, read the errors carefully — common issues:
- Unused imports: remove them (only import packages directly referenced in the file)
- Wrong return count: check the actual signature of third-party functions (e.g., `validate.NewInterceptor()` returns 1 value)
- Missing `go:embed` pattern match: ensure at least one `.sql` file exists for the embed directive

## Checklist

- [ ] `.gitignore` covers `gen/`, `.env`, binaries
- [ ] `go.mod` with correct module path
- [ ] `Makefile` with all targets
- [ ] `Dockerfile` multi-stage build (generate → build → runtime)
- [ ] `docker-compose.yml` with postgres, opensearch, codegen, api
- [ ] `buf.gen.yaml` with managed mode and `go_package_prefix` pointing to `gen/sdk`
- [ ] `sqlc.yaml` with empty sql list
- [ ] `.env` with defaults
- [ ] `pkg/config` — Config struct + Load()
- [ ] `pkg/connectapp` — App interface + h2c server + /health
- [ ] `pkg/connectutil` — NewErrorFrom + interceptors (recovery, logging, validate)
- [ ] `pkg/cache` — Cache[K,V] interface + in-memory implementation
- [ ] `pkg/outbox` — Outbox[T] interface + Event struct
- [ ] `pkg/migrate` — goose wrapper
- [ ] `sql/migrations/migrations.go` with embed + placeholder migration
- [ ] `cmd/server/main.go` — signal context, config, setup, run
- [ ] `cmd/server/setup_connections.go` — pool, river migrations, goose migrations, river client
- [ ] `cmd/server/setup_domains.go` — empty Domains struct
- [ ] `cmd/server/setup_gateway.go` — app with no handlers
- [ ] `internal/` placeholder directories
- [ ] `make vet` passes
- [ ] `make build` succeeds
- [ ] `make start` boots, `/health` returns 200
