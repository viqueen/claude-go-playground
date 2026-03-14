---
description: Review a search indexing PR
argument-hint: <pr-number>
allowed-tools: Read, Bash, Glob, Grep
disable-model-invocation: true
context: fork
---

# Review Search Agent

Audit a search indexing PR. Answer the question: **"Is the search indexing correct?"**

## Project Root

The PR targets one project: `connect-rpc-backend/` or `grpc-backend/`.
Identify which project from the PR file paths.

## How to review

1. Fetch the PR diff:
   ```
   gh pr diff <number>
   ```

2. Identify the domain(s) being indexed and read the full files (not just the diff).

3. Check every item below. For each, report **PASS** or **FAIL** with a brief explanation.

## Checklist

### Embedder Package — `pkg/embed/`

- [ ] `embed.go` defines `Embedder` interface with `Embed(ctx, text) ([]float32, error)`
- [ ] At least one provider implementation (e.g., `opensearch.go`)
- [ ] OpenSearch implementation uses `_plugins/_ml/models/<model_id>/_predict` API
- [ ] Implementation struct is private, constructor returns `Embedder` interface
- [ ] **No domain imports**: `pkg/embed/` must NOT import `gen/`, `internal/`, or any domain-specific code

### Search Package — `pkg/search/`

- [ ] `search.go` defines `Search` interface with `Index`, `Delete`, `Find`, `CreateIndexIfNotExists` methods
- [ ] `Index` and `Delete` accept `uuid.UUID` for the `id` parameter (not `string`)
- [ ] `Find` accepts typed `Criteria` (not raw JSON) and returns `*Page`
- [ ] `Criteria` struct has `Filters []Filter`, `Matches []Match`, `Vector *Vector`, `PageSize int32`, `PageToken string`
- [ ] `Filter` struct has `Field string` and `Value any` (for keyword/integer exact-match)
- [ ] `Match` struct has `Field string` and `Query string` (for text full-text search)
- [ ] `Vector` struct has `Field string`, `Values []float32`, `K int` (for k-NN)
- [ ] `Page` struct has `Hits []Hit` and `NextPageToken string`
- [ ] `Hit` struct has `ID uuid.UUID`, `Score float32`, and `Source json.RawMessage`
- [ ] Implementation translates `Criteria` into OpenSearch query internally — no raw JSON leaks
- [ ] Hybrid search: when both `Matches` and `Vector` are set, implementation blends scores
- [ ] Pagination uses `search_after` (not `from`/`size` offset) with opaque token encoding
- [ ] `CreateIndexIfNotExists` accepts `[]byte` (embedded JSON), not `string`
- [ ] **No domain imports**: `pkg/search/` must NOT import `gen/`, `internal/`, or any domain-specific code

### Embedded Mappings — `internal/outbox/<domain>/mappings/`

- [ ] `mappings.go` exists with `//go:embed *.json` and exported `FS embed.FS`
- [ ] One `<domain>.json` file per domain index
- [ ] No inline JSON mapping strings anywhere in Go code
- [ ] Each `.json` file is valid JSON (parseable by `jq`)
- [ ] Mappings live under `internal/outbox/<domain>/` (not under `pkg/search/`)

### Mapping Field Types — `internal/outbox/<domain>/mappings/<domain>.json`

- [ ] **Reference fields** mapped as `keyword`: unique keys, foreign keys (UUIDs), tag arrays
- [ ] **Searchable fields** mapped as `text` with analyzer: names, titles, descriptions, bodies
- [ ] Enums / integers mapped as `integer`
- [ ] Arrays of labels (e.g., `tags`) mapped as `keyword`
- [ ] `id` is NOT in the mapping — OpenSearch `_id` handles document identity
- [ ] `created_at` / `updated_at` NOT indexed unless time-range search is required
- [ ] `deleted_at` NOT indexed
- [ ] All reference and searchable fields from the entity are included
- [ ] Field type choices cross-referenced against SQL schema

### Denormalization — child entities with parent FK

- [ ] Child entity mapping includes parent reference fields (e.g., `space_key`, `space_name`, `space_status`)
- [ ] Denormalized fields prefixed with parent name
- [ ] Denormalized fields use same types as in parent mapping (keyword, text, integer)
- [ ] No separate parent index query needed — single-index search covers both

### Vector Embedding — `knn_vector` field

- [ ] Mapping includes `"settings": { "index": { "knn": true } }`
- [ ] `embedding` field has type `knn_vector` with `dimension`, `method.name: "hnsw"`, `method.space_type: "cosinesimil"`, `method.engine: "lucene"`
- [ ] Dimension matches the embedder's output size (e.g., 1536 for OpenAI ada-002)

### Index Definition & Document — `internal/outbox/<domain>/index.go`

- [ ] `IndexName` constant is plural lowercase (e.g., `"contents"`)
- [ ] `IndexMapping` loaded from embedded FS via `mappings.FS.ReadFile("<domain>.json")`
- [ ] `EmbeddingField` constant matches the mapping field name
- [ ] Document struct does NOT include `id`, `created_at`, `updated_at`, or `deleted_at`
- [ ] Document struct includes `Embedding []float32` with `json:"embedding,omitempty"`
- [ ] Child entity document struct includes denormalized parent fields
- [ ] `New<Domain>Document()` mapper accepts both entity and parent models (when applicable)
- [ ] `EmbeddingText()` method concatenates searchable text fields for embedding
- [ ] UUID foreign keys serialized as strings

### Index Worker — `internal/outbox/<domain>/event_index.go`

- [ ] `IndexDependencies` struct includes `Search`, `Embedder`, and `Queries`
- [ ] `NewIndexWorker()` constructor takes `IndexDependencies`
- [ ] Worker re-fetches entity from DB on create/update (not from job args)
- [ ] Worker fetches parent entity for denormalization (when entity has parent FK)
- [ ] Worker calls `embedder.Embed()` with `doc.EmbeddingText()` before indexing
- [ ] Worker sets `doc.Embedding` with result before calling `search.Index()`
- [ ] Event type switch uses domain constants — no hardcoded strings
- [ ] Create/update events → embed + `search.Index()`
- [ ] Delete events → `search.Delete()` with parsed `uuid.UUID` (no embed, no DB fetch)
- [ ] Unknown event types logged as warnings (not errors)
- [ ] Worker accepts `ctx context.Context` (not `_`)

### Layer Rules — Imports

Scan all imports:

- [ ] `pkg/search/` imports ONLY standard library + opensearch-go + zerolog + uuid — no `gen/`, no `internal/`
- [ ] `pkg/embed/` imports ONLY standard library + opensearch-go (for OpenSearch provider) — no `gen/`, no `internal/`
- [ ] `internal/outbox/<domain>/mappings/` imports nothing — pure embedded data
- [ ] `internal/outbox/<domain>/` imports `pkg/search/` — ALLOWED
- [ ] `internal/outbox/<domain>/` imports `pkg/embed/` — ALLOWED
- [ ] `internal/outbox/<domain>/` imports `gen/db/<domain>` — ALLOWED
- [ ] `internal/outbox/<domain>/` imports `internal/domain/<domain>` — ALLOWED (for event constants only)
- [ ] NO imports of `pkg/search/` or `pkg/embed/` in `internal/domain/`
- [ ] NO imports of `pkg/search/` or `pkg/embed/` in `internal/api/`

### Wiring — `cmd/server/setup_connections.go`

- [ ] `Connections` struct includes `SearchClient search.Search` and `Embedder embed.Embedder`
- [ ] Search client created with `search.New(cfg.OpenSearchURL)`
- [ ] Embedder created with appropriate provider constructor
- [ ] `CreateIndexIfNotExists` called on startup using `<domain>events.IndexName` and `<domain>events.IndexMapping`
- [ ] Index workers created via `NewIndexWorker` with all three dependencies (Search, Embedder, Queries)
- [ ] No zero-value struct workers

### Consistency Checks

- [ ] Document struct JSON tags match mapping `.json` property names exactly
- [ ] Document struct fields are a subset of the mapping properties (plus `embedding`)
- [ ] Denormalized parent fields in document match parent mapping field types
- [ ] `IndexName` constant used consistently (worker, setup — same constant)
- [ ] `EmbeddingField` constant matches the mapping property name
- [ ] No hardcoded index names or mapping JSON outside `internal/outbox/<domain>/`
- [ ] No domain-specific types or imports leaked into `pkg/search/` or `pkg/embed/`

## Output format

```
## Search PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### Component Matrix
| Component | File | Status | Notes |
|-----------|------|--------|-------|
| Embedder interface | pkg/embed/embed.go | PASS | — |
| Search interface | pkg/search/search.go | PASS | — |
| Embedded mappings | internal/outbox/<domain>/mappings/<domain>.json | PASS | — |
| Index + document | internal/outbox/<domain>/index.go | PASS | — |
| Index worker | internal/outbox/<domain>/event_index.go | PASS | — |
| Wiring | cmd/server/setup_connections.go | PASS | — |

### Import Audit
| Package | Import | Allowed | Status |
|---------|--------|---------|--------|
| pkg/search/ | opensearch-go | yes | PASS |
| pkg/embed/ | opensearch-go | yes | PASS |
| pkg/search/ | gen/db/<domain> | NO | FAIL |
| internal/outbox/<domain>/ | pkg/search/ | yes | PASS |
| internal/outbox/<domain>/ | pkg/embed/ | yes | PASS |
| internal/domain/<domain>/ | pkg/search/ | NO | FAIL |
| ... | ... | ... | ... |

### Mapping Consistency
| Document Field | JSON Tag | Mapping Property | Mapping Type | SQL Type | Denormalized | Status |
|----------------|----------|------------------|--------------|----------|--------------|--------|
| SpaceKey | space_key | space_key | keyword | TEXT (from space) | yes | PASS |
| Title | title | title | text | TEXT | no | PASS |
| Embedding | embedding | embedding | knn_vector | — | no | PASS |
| ... | ... | ... | ... | ... | ... | ... |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```

## PR Context

- PR diff: !`gh pr diff $ARGUMENTS`
- PR info: !`gh pr view $ARGUMENTS --json number,title,body,state,baseRefName,headRefName,url`
