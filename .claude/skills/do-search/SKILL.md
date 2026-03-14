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

### 1. `pkg/embed/embed.go` — Generic embedder interface

If this is the first domain being indexed, create the embedder package. If it already exists, skip this step.

This package is **purely generic** — provider-agnostic, no domain knowledge.

```go
package embed

import "context"

// Embedder generates vector embeddings from text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
```

The provider is chosen at wire time in `cmd/server/`. Implementations live in `pkg/embed/`:

- `pkg/embed/opensearch.go` — uses OpenSearch's built-in ML plugin (`_plugins/_ml/models/<model_id>/_predict`). The model ID is configured via env var.
- Additional providers (OpenAI, Cohere, etc.) can be added as `pkg/embed/<provider>.go` — each returns `Embedder` from a constructor.

```go
// OpenSearch ML plugin implementation
func NewOpenSearch(client *opensearchapi.Client, modelID string) Embedder {
	return &openSearchEmbedder{client: client, modelID: modelID}
}
```

Conventions:
- **Interface-first**: `Embedder` interface is public, implementations are private
- Each provider is a separate file with its own constructor
- Constructors return `Embedder` (the interface)
- **No domain imports**: `pkg/embed/` must NOT import `gen/`, `internal/`, or any domain-specific code

### 2. `pkg/search/search.go` — Generic search client interface and constructor

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

// Vector represents a k-NN vector search on a knn_vector field.
type Vector struct {
	Field  string
	Values []float32
	K      int
}

// Criteria defines a typed search query. The implementation translates this
// into an OpenSearch hybrid query internally:
// - Filters become term clauses (exact match, no scoring)
// - Matches become match clauses (full-text, scored)
// - Vector becomes a k-NN clause (semantic similarity, scored)
// Filters, matches, and vector are combined for hybrid search.
type Criteria struct {
	Filters   []Filter
	Matches   []Match
	Vector    *Vector
	PageSize  int32
	PageToken string
}

// Page represents a paginated set of search results.
type Page struct {
	Hits          []Hit
	NextPageToken string
}

// Hit represents a single search result with its raw JSON source.
type Hit struct {
	ID     uuid.UUID
	Score  float32
	Source json.RawMessage
}

// Search defines the interface for indexing, deleting, and querying documents.
type Search interface {
	// Index indexes or updates a document in the given index.
	Index(ctx context.Context, index string, id uuid.UUID, document any) error
	// Delete removes a document from the given index.
	Delete(ctx context.Context, index string, id uuid.UUID) error
	// Find searches an index using typed criteria and returns a paginated result.
	Find(ctx context.Context, index string, criteria Criteria) (*Page, error)
	// CreateIndexIfNotExists ensures an index exists with the given mapping.
	CreateIndexIfNotExists(ctx context.Context, index string, mapping []byte) error
}
```

Backed by `github.com/opensearch-project/opensearch-go/v4/opensearchapi`.

The implementation translates `Criteria` into an OpenSearch query:
- Each `Filter` becomes a `term` clause in `filter` (exact match, no scoring)
- Each `Match` becomes a `match` clause in `must` (full-text, scored)
- `Vector` becomes a `knn` clause (k-NN similarity search)
- When both text matches and vector are present, use OpenSearch hybrid search to blend scores
- An empty `Criteria` (no filters, matches, or vector) matches all documents
- `PageSize` and `PageToken` map to OpenSearch `size` and `search_after` — the token is an opaque encoding of the sort values (consistent with the gRPC list RPC pagination pattern using `pkg/pagination`)

Constructor:

```go
func New(address string) (Search, error) {
	// Create opensearch client with the given address
	// Return interface (not struct)
}
```

Conventions:
- **Interface-first**: `Search` interface is public, implementation struct is private
- **Typed queries**: callers use `Criteria` with `Filter`, `Match`, and `Vector` — never raw JSON. The implementation owns the OpenSearch query DSL translation.
- **Pagination**: uses `PageSize`/`PageToken` pattern consistent with gRPC list RPCs. Implementation uses OpenSearch `search_after` for efficient deep pagination. Token is an opaque base64-encoded sort value.
- **Hybrid search**: when `Vector` is set alongside `Matches`, the implementation uses OpenSearch's hybrid query to blend lexical and semantic scores.
- Constructor returns `(Search, error)` — the error covers connection/config issues
- Use `opensearchapi` client (v4) — not the legacy v2 client
- `CreateIndexIfNotExists` accepts `[]byte` (raw embedded JSON), not `string`
- Logging via `zerolog` context logger on errors
- **No domain imports**: `pkg/search/` must NOT import `gen/`, `internal/`, or any domain-specific code

### 3. `internal/outbox/<domain>/mappings/` — Embedded JSON mapping files

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

The mapping must distinguish between three field categories:

- **Reference fields** → `keyword`: unique identifiers, foreign keys, enum-like values, tags.
  These are fields users filter or look up by exact value.
- **Searchable fields** → `text` with analyzer: human-readable text users search within.
- **Vector fields** → `knn_vector`: embedding vectors for semantic search. Only include when
  the entity has text fields worth embedding (e.g., `body`, `description`).

**Denormalization for cross-domain search**: when an entity references a parent via FK,
include the parent's reference fields in the child's mapping so a single query can filter by both
entity and parent criteria. Name denormalized fields with the parent prefix (e.g., `<parent>_<field>`).
This avoids multi-index fan-out queries.

Cross-reference the SQL schema and proto definitions to identify which fields are references
(unique indexes, foreign keys, enums, arrays of labels) vs. searchable (names, titles, descriptions, bodies).

Example for a root entity (no parent FK):

```json
{
  "settings": {
    "index": {
      "knn": true
    }
  },
  "mappings": {
    "properties": {
      "<unique_key>":  { "type": "keyword" },
      "<name_field>":  { "type": "text", "analyzer": "standard" },
      "<text_field>":  { "type": "text", "analyzer": "standard" },
      "<enum_field>":  { "type": "integer" },
      "embedding": {
        "type": "knn_vector",
        "dimension": 1536,
        "method": {
          "name": "hnsw",
          "space_type": "cosinesimil",
          "engine": "lucene"
        }
      }
    }
  }
}
```

Example for a child entity (has parent FK).
Note denormalized parent fields for cross-domain search:

```json
{
  "settings": {
    "index": {
      "knn": true
    }
  },
  "mappings": {
    "properties": {
      "<parent>_id":         { "type": "keyword" },
      "<parent>_<ref_field>": { "type": "keyword" },
      "<parent>_<text_field>": { "type": "text", "analyzer": "standard" },
      "<parent>_<enum_field>": { "type": "integer" },
      "<name_field>":        { "type": "text", "analyzer": "standard" },
      "<text_field>":        { "type": "text", "analyzer": "standard" },
      "<enum_field>":        { "type": "integer" },
      "<array_field>":       { "type": "keyword" },
      "embedding": {
        "type": "knn_vector",
        "dimension": 1536,
        "method": {
          "name": "hnsw",
          "space_type": "cosinesimil",
          "engine": "lucene"
        }
      }
    }
  }
}
```

Conventions:
- File naming: `<domain>.json`
- One file per domain index
- Pure JSON — no Go string escaping, no backtick literals
- **Do not index `id`** — OpenSearch uses `_id` (the document ID) natively for lookups by ID.
- **Do not index `created_at` / `updated_at`** unless the domain requires time-range search.
- **Do not index `deleted_at`** — soft-deleted entities are removed from the index on delete events.
- **Denormalize parent reference fields** into child documents for cross-domain search. Prefix with parent name (`<parent>_<field>`).
- **Vector field**: use `knn_vector` with `dimension` matching the embedder's output size, `hnsw` method, `cosinesimil` space type, `lucene` engine. Include `"index": { "knn": true }` in settings.
- Type mapping rules:
  - UUID foreign keys → `keyword` (exact-match filter)
  - Unique keys → `keyword` (exact-match lookup)
  - Enums / integers → `integer` (exact-match filter)
  - Arrays of labels → `keyword` (OpenSearch handles arrays natively)
  - Human-readable text → `text` with `standard` analyzer
  - Embedding vectors → `knn_vector`
  - Booleans → `boolean`

### 4. `internal/outbox/<domain>/index.go` — Index name, mapping, and document struct

All domain-specific search concerns live in the outbox domain package — the index name constant,
the embedded mapping loader, and the document struct with its mapper from sqlc models.

For entities with parent references, the document struct includes denormalized parent fields
and the mapper accepts both the entity and its parent model.

```go
package <domain>

import (
	db<schema> "<module>/gen/db/<schema>"
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

// EmbeddingField is the mapping field name for the vector embedding.
const EmbeddingField = "embedding"

// <Domain>Document represents the search document for a <domain>.
// Fields match the mapping properties in mappings/<domain>.json exactly.
type <Domain>Document struct {
	// Denormalized parent fields (only when entity has a parent FK)
	<Parent>ID       string `json:"<parent>_id"`
	<Parent><Field>  string `json:"<parent>_<field>"`
	// Entity's own fields
	<Field>   string    `json:"<field>"`
	<Enum>    int32     `json:"<enum>"`
	<Array>   []string  `json:"<array>"`
	Embedding []float32 `json:"embedding,omitempty"`
}

// New<Domain>Document maps sqlc models to a search document.
// When the entity has a parent FK, accepts both the entity and its parent.
func New<Domain>Document(
	entity *db<schema>.<Entity>,
	parent *db<schema>.<Parent>,  // omit if no parent FK
) <Domain>Document {
	return <Domain>Document{
		<Parent>ID:      entity.<Parent>ID.String(),
		<Parent><Field>: parent.<Field>,
		<Field>:         entity.<Field>,
		<Enum>:          entity.<Enum>,
		<Array>:         entity.<Array>,
	}
}

// EmbeddingText returns the text to embed for this document.
// Concatenate the entity's searchable text fields.
func (d <Domain>Document) EmbeddingText() string {
	return d.<TextField1> + "\n" + d.<TextField2>
}
```

Conventions:
- Index name is plural lowercase: `<domain>s`
- Mapping loaded from embedded FS at package init — panics on missing file (build-time guarantee)
- `var` (not `const`) because `[]byte` cannot be a const
- **Document struct fields = mapping properties**: only include fields that are in the mapping JSON.
- Document struct JSON tags must match the property names in the corresponding `<domain>.json` mapping file exactly
- **Denormalized fields**: when the entity has a parent FK, the mapper accepts both models and populates parent fields. Prefixed with parent name (`<Parent><Field>` → `"<parent>_<field>"`).
- **Embedding field**: `[]float32` with `omitempty` — set by the index worker after calling the embedder. The `EmbeddingText()` method returns the text to embed (concatenation of the entity's searchable text fields).
- **EmbeddingField constant**: exported constant for the mapping field name, used by the index worker when constructing vector queries.
- UUID foreign keys are serialized as strings in the document

### 5. Update `internal/outbox/<domain>/event_index.go` — Wire index workers to OpenSearch

Update the existing index worker to actually index/delete documents via the search client.
The worker needs dependencies for search, queries, and the embedder.

**Before** (current placeholder):

```go
type IndexWorker struct {
	river.WorkerDefaults[IndexArgs]
}
```

**After**:

```go
type IndexDependencies struct {
	Search   search.Search
	Embedder embed.Embedder
	Queries  *db<domain>.Queries
}

type IndexWorker struct {
	river.WorkerDefaults[IndexArgs]
	search   search.Search
	embedder embed.Embedder
	queries  *db<domain>.Queries
}

func NewIndexWorker(deps IndexDependencies) *IndexWorker {
	return &IndexWorker{
		search:   deps.Search,
		embedder: deps.Embedder,
		queries:  deps.Queries,
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
		// For entities with parent references, also fetch the parent
		parent, err := w.queries.Get<Parent>(ctx, entity.<Parent>ID)
		if err != nil {
			return err
		}
		doc := New<Domain>Document(&entity, &parent)
		// Generate embedding from searchable text fields
		embedding, err := w.embedder.Embed(ctx, doc.EmbeddingText())
		if err != nil {
			return err
		}
		doc.Embedding = embedding
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
- **Denormalization**: when the entity has a parent FK, the worker fetches both entity and parent to populate denormalized fields
- **Embedding generation**: the worker calls `embedder.Embed()` with the document's searchable text, then sets the embedding before indexing. This keeps embedding off the request path.
- **Event type switch**: create/update → embed + index, delete → delete from index
- **Use event constants**: reference domain event constants (e.g., `<domain>domain.EventCreated`) — do NOT hardcode event type strings.
- **Same-package references**: `IndexName`, `New<Domain>Document` come from `index.go` in the same package
- **Delete by ID**: delete events only need the entity ID, no DB fetch or embedding needed

### 6. Update `cmd/server/setup_connections.go` — Add search client, embedder, and wire dependencies

Add the search client and embedder to the `Connections` struct and initialize them:

```go
type Connections struct {
	Pool         *pgxpool.Pool
	RiverClient  *river.Client[pgx.Tx]
	SearchClient search.Search
	Embedder     embed.Embedder
}
```

In `setupConnections`:
1. Create search client: `search.New(cfg.OpenSearchURL)`
2. Create embedder: `embed.NewOpenSearch(searchClient, cfg.EmbedModelID)` (or other provider)
3. Create indexes on startup: `searchClient.CreateIndexIfNotExists(ctx, <domain>events.IndexName, <domain>events.IndexMapping)`
4. Pass search client, embedder, and queries to `NewIndexWorker` when registering workers

Worker registration changes from:
```go
river.AddWorker(workers, &<domain>events.IndexWorker{})
```
To:
```go
river.AddWorker(workers, <domain>events.NewIndexWorker(<domain>events.IndexDependencies{
	Search:   searchClient,
	Embedder: embedder,
	Queries:  db<domain>.New(pool),
}))
```

### 7. No changes needed to

- `internal/domain/` — domain layer does not know about search or embeddings
- `internal/api/` — search is triggered asynchronously via outbox, not synchronously in handlers
- `pkg/outbox/` — outbox interface is unchanged
- `internal/outbox/river.go` — event mapping is unchanged (index jobs already created)

## Conventions

- **Interface-first**: `Search` and `Embedder` interfaces are public, implementations are private
- **Generic `pkg/search/` and `pkg/embed/`**: contain only interfaces and clients — zero domain knowledge
- **Dependencies struct**: `IndexDependencies` exported, includes `Search`, `Embedder`, and `Queries`
- **Embedded JSON mappings**: mappings live as `.json` files in `internal/outbox/<domain>/mappings/`, loaded via `//go:embed` — never inline JSON in Go code
- **Domain-specific search types in outbox**: index name, mapping, document struct, and mapper all live in `internal/outbox/<domain>/` alongside the index worker
- **Denormalization**: child entities include parent reference fields in their search documents to enable single-index cross-domain queries
- **Embedding in workers**: embedding generation happens in the async River worker, not on the request path
- **Hybrid search**: `Criteria` supports `Filters`, `Matches`, and `Vector` for combined keyword + semantic search
- **Pagination**: `Find` returns `*Page` with `NextPageToken`, consistent with gRPC list RPCs
- **File naming**: `index.go` for index name + mapping + document struct, `event_index.go` for the worker, `mappings/<domain>.json` for mapping definitions
- **Re-fetch pattern**: index workers always re-fetch from DB for consistency
- **Startup index creation**: indexes created with `CreateIndexIfNotExists` during server boot
- **No search in domain layer**: domain emits events, outbox workers handle indexing — clean separation

## Layer Rules

- `pkg/search/` depends on nothing domain-specific — purely generic, extractable as a shared module
- `pkg/embed/` depends on nothing domain-specific — purely generic, provider implementations may depend on opensearch-go
- `internal/outbox/<domain>/` depends on: `pkg/search/`, `pkg/embed/`, `pkg/outbox`, `gen/db/<domain>`, `internal/domain/<domain>` (for event constants only), river
- `internal/outbox/<domain>/mappings/` depends on nothing — pure embedded data
- `internal/domain/` must NOT depend on `pkg/search/` or `pkg/embed/`
- `internal/api/` must NOT depend on `pkg/search/` or `pkg/embed/` (search queries will be a separate concern)

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

- [ ] `pkg/embed/embed.go` with `Embedder` interface
- [ ] `pkg/embed/opensearch.go` with OpenSearch ML plugin implementation
- [ ] `pkg/embed/` has zero imports from `gen/`, `internal/`, or any domain-specific code
- [ ] `pkg/search/search.go` with `Search` interface, `Criteria`, `Filter`, `Match`, `Vector`, `Page`, `Hit`
- [ ] `Criteria` includes `PageSize` and `PageToken` for pagination
- [ ] `Find` returns `*Page` with `NextPageToken` (consistent with gRPC list RPCs)
- [ ] `Criteria.Vector` supports optional k-NN search
- [ ] Hybrid search: implementation blends text matches and vector scores when both are present
- [ ] `pkg/search/` has zero imports from `gen/`, `internal/`, or any domain-specific code
- [ ] Uses `opensearch-go/v4` client (not legacy v2)
- [ ] `internal/outbox/<domain>/mappings/mappings.go` with `//go:embed *.json` and exported `FS`
- [ ] `internal/outbox/<domain>/mappings/<domain>.json` with valid JSON mapping
- [ ] Mapping includes `knn_vector` field with dimension, hnsw method, cosinesimil, lucene engine
- [ ] Mapping includes `"settings": { "index": { "knn": true } }`
- [ ] Child entity mappings include denormalized parent reference fields (prefixed with parent name)
- [ ] Mapping JSON validates with `jq`
- [ ] No inline JSON mapping strings in Go code
- [ ] `internal/outbox/<domain>/index.go` with `IndexName`, `IndexMapping`, `EmbeddingField`, document struct, mapper, and `EmbeddingText()`
- [ ] Index name is plural lowercase
- [ ] Document struct includes denormalized parent fields when entity has a parent FK
- [ ] Document struct includes `Embedding []float32` with `omitempty`
- [ ] Mapper for child entities accepts both entity and parent models
- [ ] `EmbeddingText()` concatenates searchable text fields
- [ ] Document JSON tags match mapping property names in `<domain>.json` exactly
- [ ] `internal/outbox/<domain>/event_index.go` updated with `IndexDependencies` including `Embedder`
- [ ] Index worker calls `embedder.Embed()` before indexing on create/update
- [ ] Index worker fetches parent entity for denormalization (when applicable)
- [ ] Index worker uses event type constants from domain package (not hardcoded strings)
- [ ] Create/update events → embed + `search.Index()`, delete events → `search.Delete()`
- [ ] `setup_connections.go` creates search client, embedder, and passes both to index workers
- [ ] `setup_connections.go` calls `CreateIndexIfNotExists` on startup
- [ ] No imports of `pkg/search/` or `pkg/embed/` in `internal/domain/` or `internal/api/`
- [ ] `make vet` passes
- [ ] `make build` succeeds
