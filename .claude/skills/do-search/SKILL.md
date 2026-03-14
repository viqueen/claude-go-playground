---
description: Implement search indexing for a domain using OpenSearch
argument-hint: <domain> <project>
allowed-tools: Read, Write, Edit, Bash, Glob, Grep
disable-model-invocation: true
context: fork
---

# Search Agent

Implement OpenSearch indexing for a domain. This PR is auditable as: **"Is the search indexing correct?"**

Depends on: `do-domain` agent PR (`internal/domain/<domain>/` must exist with event constants).

## Project Root

All file paths are relative to the chosen project: `connect-rpc-backend/` or `grpc-backend/`.
The user will specify which project. All `make` commands must be run from the project root.

## Inputs

The user will specify:
- The **domain name** (e.g., `space`, `content`)
- Which **fields** to index and their OpenSearch mapping types
- Any **analyzers** or custom mappings (optional — sensible defaults are used)

## What to generate

### 1. `pkg/search/search.go` — Search client interface and constructor

If this is the first domain being indexed, create the shared search package. If it already exists, skip this step.

```go
package search

import (
	"context"
)

// Search defines the interface for indexing and deleting documents.
type Search interface {
	// Index indexes or updates a document in the given index.
	Index(ctx context.Context, index string, id string, document any) error
	// Delete removes a document from the given index.
	Delete(ctx context.Context, index string, id string) error
	// CreateIndexIfNotExists ensures an index exists with the given mapping.
	CreateIndexIfNotExists(ctx context.Context, index string, mapping string) error
}
```

Backed by `github.com/opensearch-project/opensearch-go/v4/opensearchapi`.

Constructor:

```go
func New(address string) (Search, error) {
	// Create opensearch client with the given address
	// Return interface (not struct)
}
```

Conventions:
- **Interface-first**: `Search` interface is public, implementation struct is private
- Constructor returns `(Search, error)` — the error covers connection/config issues
- Use `opensearchapi` client (v4) — not the legacy v2 client
- Logging via `zerolog` context logger on errors

### 2. `pkg/search/index_<domain>.go` — Index name and mapping per domain

One file per domain defining the index name constant and the OpenSearch mapping JSON:

```go
package search

const <Domain>Index = "<domain>s"

const <Domain>Mapping = `{
	"mappings": {
		"properties": {
			"id":         { "type": "keyword" },
			"name":       { "type": "text", "analyzer": "standard" },
			"status":     { "type": "integer" },
			"created_at": { "type": "date" },
			"updated_at": { "type": "date" }
		}
	}
}`
```

Conventions:
- Index name is plural lowercase: `spaces`, `contents`
- Index name and mapping are exported constants
- File naming: `index_<domain>.go`
- Map proto/SQL types to OpenSearch types:
  - `UUID` / `TEXT` with keyword semantics → `keyword`
  - `TEXT` with full-text search → `text` with `standard` analyzer
  - `INT` / enum → `integer`
  - `TEXT[]` → `keyword` (array — OpenSearch handles arrays natively)
  - `TIMESTAMPTZ` → `date`
  - `BOOLEAN` → `boolean`

### 3. `pkg/search/document_<domain>.go` — Document struct per domain

A plain struct representing the OpenSearch document for this domain. This is what gets serialized to JSON and indexed.

```go
package search

import "time"

// <Domain>Document represents the search document for a <domain>.
type <Domain>Document struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    int32     `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
```

Include a mapping function from the sqlc model:

```go
import db<domain> "<module>/gen/db/<domain>"

func New<Domain>Document(model *db<domain>.<Entity>) <Domain>Document {
	return <Domain>Document{
		ID:        model.ID.String(),
		Name:      model.Name,
		Status:    model.Status,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
```

Conventions:
- Document structs use JSON tags matching the OpenSearch mapping field names
- UUIDs are serialized as strings
- Timestamps use `time.Time` (serialized as ISO 8601 by default — compatible with OpenSearch `date` type)
- The `New<Domain>Document` function maps from sqlc model to search document — this is the single source of truth for the mapping

### 4. Update `internal/outbox/<domain>/event_index.go` — Wire index workers to OpenSearch

Update the existing index worker to actually index/delete documents via the search client. The worker needs two new dependencies: the search client and the sqlc queries (to re-fetch the entity).

**Before** (current placeholder):

```go
type IndexWorker struct {
	river.WorkerDefaults[IndexArgs]
}
```

**After**:

```go
type IndexDependencies struct {
	Search  search.Search
	Queries *db<domain>.Queries
}

type IndexWorker struct {
	river.WorkerDefaults[IndexArgs]
	search  search.Search
	queries *db<domain>.Queries
}

func NewIndexWorker(deps IndexDependencies) *IndexWorker {
	return &IndexWorker{
		search:  deps.Search,
		queries: deps.Queries,
	}
}
```

The `Work` method:

```go
func (w *IndexWorker) Work(ctx context.Context, job *river.Job[IndexArgs]) error {
	switch job.Args.EventType {
	case "<domain>.created", "<domain>.updated":
		id, err := uuid.FromString(job.Args.<Domain>ID)
		if err != nil {
			return err
		}
		entity, err := w.queries.Get<Entity>(ctx, id)
		if err != nil {
			return err
		}
		doc := search.New<Domain>Document(&entity)
		return w.search.Index(ctx, search.<Domain>Index, doc.ID, doc)
	case "<domain>.deleted":
		return w.search.Delete(ctx, search.<Domain>Index, job.Args.<Domain>ID)
	default:
		log.Ctx(ctx).Warn().Str("event_type", job.Args.EventType).Msg("unknown event type")
		return nil
	}
}
```

Key patterns:
- **Re-fetch from DB**: the worker fetches the current entity state from the database, not from job args — this ensures indexed data is consistent with DB state
- **Event type switch**: create/update → index, delete → delete from index
- **Use event constants**: reference domain event constants (e.g., `<domain>domain.EventCreated`) — do NOT hardcode event type strings. Import the domain package for constants only.
- **Delete by ID**: delete events only need the entity ID, no DB fetch needed

### 5. Update `cmd/server/setup_connections.go` — Add search client and wire dependencies

Add the search client to the `Connections` struct and initialize it:

```go
type Connections struct {
	Pool         *pgxpool.Pool
	RiverClient  *river.Client[pgx.Tx]
	SearchClient search.Search
}
```

In `setupConnections`:
1. Create search client: `search.New(cfg.OpenSearchURL)`
2. Create indexes on startup: `searchClient.CreateIndexIfNotExists(ctx, search.<Domain>Index, search.<Domain>Mapping)`
3. Pass search client and queries to `NewIndexWorker` when registering workers

Worker registration changes from:
```go
river.AddWorker(workers, &<domain>events.IndexWorker{})
```
To:
```go
river.AddWorker(workers, <domain>events.NewIndexWorker(<domain>events.IndexDependencies{
	Search:  searchClient,
	Queries: db<domain>.New(pool),
}))
```

### 6. No changes needed to

- `internal/domain/` — domain layer does not know about search
- `internal/api/` — search is triggered asynchronously via outbox, not synchronously in handlers
- `pkg/outbox/` — outbox interface is unchanged
- `internal/outbox/river.go` — event mapping is unchanged (index jobs already created)

## Conventions

- **Interface-first**: `Search` interface is public, `search` struct is private
- **Dependencies struct**: `IndexDependencies` exported, used in constructor
- **File naming**: `index_<domain>.go` for mappings, `document_<domain>.go` for document structs
- **Re-fetch pattern**: index workers always re-fetch from DB for consistency
- **Startup index creation**: indexes created with `CreateIndexIfNotExists` during server boot
- **No search in domain layer**: domain emits events, outbox workers handle indexing — clean separation

## Layer Rules

- `pkg/search/` depends on: `gen/db/<domain>` (for document mapping only), opensearch-go client
- `internal/outbox/<domain>/` can now additionally depend on: `pkg/search/`, `gen/db/<domain>`
- `internal/domain/` must NOT depend on `pkg/search/`
- `internal/api/` must NOT depend on `pkg/search/` (search queries will be a separate concern)

## Post-Generation

1. Run `go get github.com/opensearch-project/opensearch-go/v4` from the project root
2. Run `make vet` — fix all compilation errors
3. Run `make build` — confirm Docker build works
4. Run `make infra` — start infrastructure (OpenSearch must be healthy)
5. Run `make start` — create an entity via gRPC/Connect, verify the index worker logs show successful indexing
6. Verify the document in OpenSearch: `curl http://localhost:9200/<domain>s/_search?pretty`
7. Run `make teardown`

## Checklist

- [ ] `pkg/search/search.go` with `Search` interface, private struct, `New()` constructor
- [ ] Uses `opensearch-go/v4` client (not legacy v2)
- [ ] `pkg/search/index_<domain>.go` with index name constant and mapping JSON
- [ ] Index name is plural lowercase
- [ ] Mapping types align with SQL/proto types
- [ ] `pkg/search/document_<domain>.go` with document struct and `New<Domain>Document()` mapper
- [ ] Document JSON tags match mapping field names
- [ ] `internal/outbox/<domain>/event_index.go` updated with `IndexDependencies` and `NewIndexWorker`
- [ ] Index worker re-fetches entity from DB (not from job args)
- [ ] Index worker uses event type constants from domain package (not hardcoded strings)
- [ ] Create/update events → `search.Index()`, delete events → `search.Delete()`
- [ ] `setup_connections.go` creates search client and passes to index workers
- [ ] `setup_connections.go` calls `CreateIndexIfNotExists` on startup for each domain index
- [ ] No imports of `pkg/search/` in `internal/domain/` or `internal/api/`
- [ ] `go get opensearch-go/v4` added to dependencies
- [ ] `make vet` passes
- [ ] `make build` succeeds
