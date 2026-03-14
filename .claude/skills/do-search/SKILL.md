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

### 1. `pkg/search/search.go` — Generic search client interface and constructor

If this is the first domain being indexed, create the shared search package. If it already exists, skip this step.

This package is **purely generic** — no domain-specific types, no imports from `gen/` or `internal/`.
It is extractable as a shared module, consistent with the `pkg/` layer rule.

```go
package search

import (
	"context"
	"encoding/json"

	"github.com/gofrs/uuid/v5"
)

// Filter represents an exact-match constraint on a keyword or integer field.
type Filter struct {
	Field string
	Value any
}

// Match represents a full-text search on a text field.
type Match struct {
	Field string
	Query string
}

// Criteria defines a typed search query — filters for reference fields,
// matches for searchable fields. The implementation translates this into
// an OpenSearch bool query internally.
type Criteria struct {
	Filters []Filter
	Matches []Match
}

// Hit represents a single search result with its raw JSON source.
type Hit struct {
	ID     uuid.UUID
	Source json.RawMessage
}

// Search defines the interface for indexing, deleting, and querying documents.
type Search interface {
	// Index indexes or updates a document in the given index.
	Index(ctx context.Context, index string, id uuid.UUID, document any) error
	// Delete removes a document from the given index.
	Delete(ctx context.Context, index string, id uuid.UUID) error
	// Find searches an index using typed criteria and returns matching hits.
	Find(ctx context.Context, index string, criteria Criteria) ([]Hit, error)
	// CreateIndexIfNotExists ensures an index exists with the given mapping.
	CreateIndexIfNotExists(ctx context.Context, index string, mapping []byte) error
}
```

Backed by `github.com/opensearch-project/opensearch-go/v4/opensearchapi`.

The implementation translates `Criteria` into an OpenSearch `bool` query:
- Each `Filter` becomes a `term` clause in `filter` (exact match, no scoring)
- Each `Match` becomes a `match` clause in `must` (full-text, scored)
- An empty `Criteria` matches all documents

Constructor:

```go
func New(address string) (Search, error) {
	// Create opensearch client with the given address
	// Return interface (not struct)
}
```

Conventions:
- **Interface-first**: `Search` interface is public, implementation struct is private
- **Typed queries**: callers use `Criteria` with `Filter` and `Match` — never raw JSON. The implementation owns the OpenSearch query DSL translation.
- Constructor returns `(Search, error)` — the error covers connection/config issues
- Use `opensearchapi` client (v4) — not the legacy v2 client
- `CreateIndexIfNotExists` accepts `[]byte` (raw embedded JSON), not `string`
- Logging via `zerolog` context logger on errors
- **No domain imports**: `pkg/search/` must NOT import `gen/`, `internal/`, or any domain-specific code

### 2. `internal/outbox/<domain>/mappings/` — Embedded JSON mapping files

Mappings are standalone `.json` files loaded via `//go:embed`, following the same pattern as
`sql/migrations/migrations.go`. This keeps mappings reviewable, lintable, and out of Go code.

Domain-specific mappings live under the outbox domain package — not in `pkg/search/` — because
they are tied to a specific domain's schema and belong in the `internal/` layer.

#### `internal/outbox/<domain>/mappings/mappings.go`

```go
package mappings

import "embed"

//go:embed *.json
var FS embed.FS
```

#### `internal/outbox/<domain>/mappings/<domain>.json` — One JSON file per index

Plain JSON, one file per domain. The file name matches the domain name (not the index name).

The mapping must distinguish between **reference fields** (used for exact-match filtering and lookups)
and **searchable fields** (used for full-text search). This distinction determines the OpenSearch type:

- **Reference fields** → `keyword`: unique identifiers, foreign keys, enum-like values, tags.
  These are fields users filter or look up by exact value (e.g., find space by key, find content by space_id, filter by tag).
- **Searchable fields** → `text` with analyzer: human-readable text users search within
  (e.g., search spaces by name, search content by title or body).

Cross-reference the SQL schema and proto definitions to identify which fields are references
(unique indexes, foreign keys, enums, arrays of labels) vs. searchable (names, titles, descriptions, bodies).

Example for a space entity (SQL has: name, key, description, status, visibility):

```json
{
  "mappings": {
    "properties": {
      "key":         { "type": "keyword" },
      "name":        { "type": "text", "analyzer": "standard" },
      "description": { "type": "text", "analyzer": "standard" },
      "status":      { "type": "integer" },
      "visibility":  { "type": "integer" }
    }
  }
}
```

Example for a content entity (SQL has: space_id FK, title, body, status, tags[]):

```json
{
  "mappings": {
    "properties": {
      "space_id": { "type": "keyword" },
      "title":    { "type": "text", "analyzer": "standard" },
      "body":     { "type": "text", "analyzer": "standard" },
      "status":   { "type": "integer" },
      "tags":     { "type": "keyword" }
    }
  }
}
```

Conventions:
- File naming: `<domain>.json` (e.g., `space.json`, `content.json`)
- One file per domain index
- Pure JSON — no Go string escaping, no backtick literals
- **Do not index `id`** — OpenSearch uses `_id` (the document ID) natively for lookups by ID. Indexing `id` as a field is redundant.
- **Do not index `created_at` / `updated_at`** unless the domain requires time-range search. Timestamps are metadata, not reference or search fields.
- **Do not index `deleted_at`** — soft-deleted entities are removed from the index on delete events.
- Type mapping rules:
  - UUID foreign keys (e.g., `space_id`) → `keyword` (exact-match filter)
  - Unique keys (e.g., space `key`) → `keyword` (exact-match lookup)
  - Enums / integers (e.g., `status`, `visibility`) → `integer` (exact-match filter)
  - Arrays of labels (e.g., `tags`) → `keyword` (OpenSearch handles arrays natively, each element is a keyword)
  - Human-readable text (e.g., `name`, `title`, `body`) → `text` with `standard` analyzer (full-text search)
  - Booleans → `boolean`

### 3. `internal/outbox/<domain>/index.go` — Index name, mapping, and document struct

All domain-specific search concerns live in the outbox domain package — the index name constant,
the embedded mapping loader, and the document struct with its mapper from sqlc models.

Example for a space entity:

```go
package space

import (
	db<domain> "<module>/gen/db/<domain>"
	"<module>/internal/outbox/<domain>/mappings"
)

// Index name — plural lowercase
const IndexName = "<domain>s"

// Mapping loaded from embedded JSON
var IndexMapping = must(mappings.FS.ReadFile("<domain>.json"))

func must(data []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return data
}

// <Domain>Document represents the search document for a <domain>.
// Fields match the mapping properties in mappings/<domain>.json exactly.
type <Domain>Document struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      int32  `json:"status"`
	Visibility  int32  `json:"visibility"`
}

// New<Domain>Document maps a sqlc model to a search document.
func New<Domain>Document(model *db<domain>.<Entity>) <Domain>Document {
	return <Domain>Document{
		Key:         model.Key,
		Name:        model.Name,
		Description: model.Description,
		Status:      model.Status,
		Visibility:  model.Visibility,
	}
}
```

Conventions:
- Index name is plural lowercase: `spaces`, `contents`
- Mapping loaded from embedded FS at package init — panics on missing file (build-time guarantee)
- `var` (not `const`) because `[]byte` cannot be a const
- **Document struct fields = mapping properties**: only include fields that are in the mapping JSON. Do not include `id` (OpenSearch `_id` handles this), `created_at`, `updated_at`, or `deleted_at`.
- Document struct JSON tags must match the property names in the corresponding `<domain>.json` mapping file exactly
- UUID foreign keys (e.g., `space_id`) are serialized as strings in the document
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

The `Work` method references constants and types from the same package (`index.go`):

```go
func (w *IndexWorker) Work(ctx context.Context, job *river.Job[IndexArgs]) error {
	id, err := uuid.FromString(job.Args.<Domain>ID)
	if err != nil {
		return err
	}
	switch job.Args.EventType {
	case <domain>domain.EventCreated, <domain>domain.EventUpdated:
		entity, err := w.queries.Get<Entity>(ctx, id)
		if err != nil {
			return err
		}
		doc := New<Domain>Document(&entity)
		return w.search.Index(ctx, IndexName, id, doc)
	case <domain>domain.EventDeleted:
		return w.search.Delete(ctx, IndexName, id)
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
- **Same-package references**: `IndexName`, `New<Domain>Document` come from `index.go` in the same package — no cross-package coupling for domain-specific search types
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
2. Create indexes on startup: `searchClient.CreateIndexIfNotExists(ctx, <domain>events.IndexName, <domain>events.IndexMapping)`
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
- **Generic `pkg/search/`**: contains only the interface and OpenSearch client — zero domain knowledge
- **Dependencies struct**: `IndexDependencies` exported, used in constructor
- **Embedded JSON mappings**: mappings live as `.json` files in `internal/outbox/<domain>/mappings/`, loaded via `//go:embed` — never inline JSON in Go code
- **Domain-specific search types in outbox**: index name, mapping, document struct, and mapper all live in `internal/outbox/<domain>/` alongside the index worker
- **File naming**: `index.go` for index name + mapping + document struct, `event_index.go` for the worker, `mappings/<domain>.json` for mapping definitions
- **Re-fetch pattern**: index workers always re-fetch from DB for consistency
- **Startup index creation**: indexes created with `CreateIndexIfNotExists` during server boot
- **No search in domain layer**: domain emits events, outbox workers handle indexing — clean separation

## Layer Rules

- `pkg/search/` depends on nothing domain-specific — purely generic, extractable as a shared module
- `internal/outbox/<domain>/` depends on: `pkg/search/`, `pkg/outbox`, `gen/db/<domain>`, `internal/domain/<domain>` (for event constants only), river
- `internal/outbox/<domain>/mappings/` depends on nothing — pure embedded data
- `internal/domain/` must NOT depend on `pkg/search/`
- `internal/api/` must NOT depend on `pkg/search/` (search queries will be a separate concern)

## Post-Generation

1. Run `go get github.com/opensearch-project/opensearch-go/v4` from the project root
2. Validate mappings: `for f in internal/outbox/<domain>/mappings/*.json; do jq . "$f" > /dev/null || echo "INVALID: $f"; done`
3. Run `make vet` — fix all compilation errors
4. Run `make build` — confirm Docker build works
5. Run `make infra` — start infrastructure (OpenSearch must be healthy)
6. Run `make start` — create an entity via gRPC/Connect, verify the index worker logs show successful indexing
7. Verify the document in OpenSearch: `curl http://localhost:9200/<domain>s/_search?pretty`
8. Run `make teardown`

## Checklist

- [ ] `pkg/search/search.go` with `Search` interface, private struct, `New()` constructor
- [ ] `pkg/search/` has zero imports from `gen/`, `internal/`, or any domain-specific code
- [ ] `CreateIndexIfNotExists` accepts `[]byte` (not `string`)
- [ ] Uses `opensearch-go/v4` client (not legacy v2)
- [ ] `internal/outbox/<domain>/mappings/mappings.go` with `//go:embed *.json` and exported `FS`
- [ ] `internal/outbox/<domain>/mappings/<domain>.json` with valid JSON mapping
- [ ] Mapping JSON validates with `jq`
- [ ] No inline JSON mapping strings in Go code
- [ ] `internal/outbox/<domain>/index.go` with `IndexName` constant, `IndexMapping` var, document struct, and mapper
- [ ] Index name is plural lowercase
- [ ] Mapping loaded from embedded FS via `mappings.FS.ReadFile()`
- [ ] Mapping types align with SQL/proto types
- [ ] Document JSON tags match mapping property names in `<domain>.json` exactly
- [ ] `New<Domain>Document()` maps from sqlc model to search document
- [ ] `internal/outbox/<domain>/event_index.go` updated with `IndexDependencies` and `NewIndexWorker`
- [ ] Index worker re-fetches entity from DB (not from job args)
- [ ] Index worker uses event type constants from domain package (not hardcoded strings)
- [ ] Index worker references `IndexName` and `New<Domain>Document` from same package (not from `pkg/search/`)
- [ ] Create/update events → `search.Index()`, delete events → `search.Delete()`
- [ ] `setup_connections.go` creates search client and passes to index workers
- [ ] `setup_connections.go` calls `CreateIndexIfNotExists` on startup using `<domain>events.IndexName` and `<domain>events.IndexMapping`
- [ ] No imports of `pkg/search/` in `internal/domain/` or `internal/api/`
- [ ] `go get opensearch-go/v4` added to dependencies
- [ ] `make vet` passes
- [ ] `make build` succeeds
