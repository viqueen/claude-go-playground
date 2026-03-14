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

### Search Package — `pkg/search/`

- [ ] `search.go` defines `Search` interface with `Index`, `Delete`, `CreateIndexIfNotExists` methods
- [ ] Implementation struct is private (lowercase)
- [ ] Constructor `New()` returns `(Search, error)` — interface, not struct
- [ ] Uses `opensearch-go/v4` client (`opensearchapi`) — not legacy v2
- [ ] Error handling logs via `zerolog` and returns meaningful errors

### Index Mapping — `pkg/search/index_<domain>.go`

- [ ] Index name constant is plural lowercase (e.g., `SpaceIndex = "spaces"`)
- [ ] Mapping JSON is valid and well-formed
- [ ] Field types align with SQL schema:
  - UUID / keyword fields → `keyword`
  - Full-text fields → `text` with appropriate analyzer
  - Enums / integers → `integer`
  - Arrays → `keyword` (OpenSearch handles arrays natively)
  - Timestamps → `date`
  - Booleans → `boolean`
- [ ] All searchable fields from the entity are included in the mapping
- [ ] No unnecessary fields indexed (e.g., `deleted_at` should not be indexed)

### Document Struct — `pkg/search/document_<domain>.go`

- [ ] Document struct fields match mapping field names via JSON tags
- [ ] `New<Domain>Document()` mapper correctly converts sqlc model to document
- [ ] UUIDs serialized as strings
- [ ] Timestamps use `time.Time` (ISO 8601 compatible with OpenSearch `date` type)
- [ ] No fields included that aren't in the mapping

### Index Worker — `internal/outbox/<domain>/event_index.go`

- [ ] `IndexDependencies` struct is exported with `Search` and `Queries` fields
- [ ] `NewIndexWorker()` constructor takes `IndexDependencies`
- [ ] Worker re-fetches entity from DB on create/update (not from job args)
- [ ] Event type switch uses domain constants (e.g., `<domain>domain.EventCreated`) — no hardcoded strings
- [ ] Create and update events call `search.Index()`
- [ ] Delete events call `search.Delete()` with entity ID only (no DB fetch)
- [ ] Unknown event types logged as warnings (not errors)
- [ ] Worker accepts `ctx context.Context` (not `_`)

### Layer Rules — Imports

Scan all imports:

- [ ] `pkg/search/` imports `gen/db/<domain>` — ALLOWED (for document mapping)
- [ ] `pkg/search/` imports opensearch-go — ALLOWED
- [ ] `internal/outbox/<domain>/` imports `pkg/search/` — ALLOWED
- [ ] `internal/outbox/<domain>/` imports `gen/db/<domain>` — ALLOWED
- [ ] `internal/outbox/<domain>/` imports `internal/domain/<domain>` — ALLOWED (for event constants only)
- [ ] NO imports of `pkg/search/` in `internal/domain/`
- [ ] NO imports of `pkg/search/` in `internal/api/`

### Wiring — `cmd/server/setup_connections.go`

- [ ] `Connections` struct includes `SearchClient search.Search`
- [ ] Search client created with `search.New(cfg.OpenSearchURL)`
- [ ] `CreateIndexIfNotExists` called on startup for each domain index
- [ ] Index workers created via `NewIndexWorker` with dependencies (not zero-value struct)
- [ ] Search client and queries both passed to index worker

### Consistency Checks

- [ ] Document struct fields are a subset of the index mapping fields
- [ ] JSON tags on document struct match mapping property names exactly
- [ ] Index name constant used consistently (worker, setup, mapping — same constant)
- [ ] No hardcoded index names or mapping JSON outside `pkg/search/`

## Output format

```
## Search PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### Component Matrix
| Component | File | Status | Notes |
|-----------|------|--------|-------|
| Search interface | pkg/search/search.go | PASS | — |
| Index mapping | pkg/search/index_<domain>.go | PASS | — |
| Document struct | pkg/search/document_<domain>.go | PASS | — |
| Index worker | internal/outbox/<domain>/event_index.go | PASS | — |
| Wiring | cmd/server/setup_connections.go | PASS | — |

### Import Audit
| Package | Import | Allowed | Status |
|---------|--------|---------|--------|
| pkg/search/ | gen/db/<domain> | yes | PASS |
| internal/outbox/<domain>/ | pkg/search/ | yes | PASS |
| internal/domain/<domain>/ | pkg/search/ | NO | FAIL |
| ... | ... | ... | ... |

### Mapping Consistency
| Document Field | JSON Tag | Mapping Property | Mapping Type | SQL Type | Status |
|----------------|----------|------------------|--------------|----------|--------|
| ID | id | id | keyword | UUID | PASS |
| Name | name | name | text | TEXT | PASS |
| ... | ... | ... | ... | ... | ... |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```

## PR Context

- PR diff: !`gh pr diff $ARGUMENTS`
- PR info: !`gh pr view $ARGUMENTS --json number,title,body,state,baseRefName,headRefName,url`
