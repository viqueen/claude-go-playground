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
- [ ] `CreateIndexIfNotExists` accepts `[]byte` (embedded JSON), not `string`
- [ ] Implementation struct is private (lowercase)
- [ ] Constructor `New()` returns `(Search, error)` — interface, not struct
- [ ] Uses `opensearch-go/v4` client (`opensearchapi`) — not legacy v2
- [ ] Error handling logs via `zerolog` and returns meaningful errors
- [ ] **No domain imports**: `pkg/search/` must NOT import `gen/`, `internal/`, or any domain-specific code

### Embedded Mappings — `internal/outbox/<domain>/mappings/`

- [ ] `mappings.go` exists with `//go:embed *.json` and exported `FS embed.FS`
- [ ] One `<domain>.json` file per domain index
- [ ] No inline JSON mapping strings anywhere in Go code
- [ ] Each `.json` file is valid JSON (parseable by `jq`)
- [ ] Mappings live under `internal/outbox/<domain>/` (not under `pkg/search/`)

### Index Definition & Document — `internal/outbox/<domain>/index.go`

- [ ] `IndexName` constant is plural lowercase (e.g., `"spaces"`)
- [ ] `IndexMapping` loaded from embedded FS via `mappings.FS.ReadFile("<domain>.json")`
- [ ] Mapping is a `var` (not `const`) since it's `[]byte`
- [ ] Document struct defined in same package with JSON tags
- [ ] `New<Domain>Document()` mapper converts sqlc model to document struct
- [ ] UUIDs serialized as strings
- [ ] Timestamps use `time.Time` (ISO 8601 compatible with OpenSearch `date` type)
- [ ] Field types in `.json` align with SQL schema:
  - UUID / keyword fields → `keyword`
  - Full-text fields → `text` with appropriate analyzer
  - Enums / integers → `integer`
  - Arrays → `keyword` (OpenSearch handles arrays natively)
  - Timestamps → `date`
  - Booleans → `boolean`
- [ ] All searchable fields from the entity are included in the mapping
- [ ] No unnecessary fields indexed (e.g., `deleted_at` should not be indexed)

### Index Worker — `internal/outbox/<domain>/event_index.go`

- [ ] `IndexDependencies` struct is exported with `Search` and `Queries` fields
- [ ] `NewIndexWorker()` constructor takes `IndexDependencies`
- [ ] Worker re-fetches entity from DB on create/update (not from job args)
- [ ] Event type switch uses domain constants (e.g., `<domain>domain.EventCreated`) — no hardcoded strings
- [ ] Create and update events call `search.Index()` with `IndexName` from same package
- [ ] Delete events call `search.Delete()` with entity ID only (no DB fetch)
- [ ] References `IndexName` and `New<Domain>Document` from same package (not from `pkg/search/`)
- [ ] Unknown event types logged as warnings (not errors)
- [ ] Worker accepts `ctx context.Context` (not `_`)

### Layer Rules — Imports

Scan all imports:

- [ ] `pkg/search/` imports ONLY standard library + opensearch-go + zerolog — no `gen/`, no `internal/`
- [ ] `internal/outbox/<domain>/mappings/` imports nothing — pure embedded data
- [ ] `internal/outbox/<domain>/` imports `pkg/search/` — ALLOWED
- [ ] `internal/outbox/<domain>/` imports `gen/db/<domain>` — ALLOWED
- [ ] `internal/outbox/<domain>/` imports `internal/domain/<domain>` — ALLOWED (for event constants only)
- [ ] NO imports of `pkg/search/` in `internal/domain/`
- [ ] NO imports of `pkg/search/` in `internal/api/`

### Wiring — `cmd/server/setup_connections.go`

- [ ] `Connections` struct includes `SearchClient search.Search`
- [ ] Search client created with `search.New(cfg.OpenSearchURL)`
- [ ] `CreateIndexIfNotExists` called on startup using `<domain>events.IndexName` and `<domain>events.IndexMapping`
- [ ] Index workers created via `NewIndexWorker` with dependencies (not zero-value struct)
- [ ] Search client and queries both passed to index worker

### Consistency Checks

- [ ] Document struct JSON tags match mapping `.json` property names exactly
- [ ] Document struct fields are a subset of the mapping properties
- [ ] `IndexName` constant used consistently (worker, setup — same constant from same package)
- [ ] No hardcoded index names or mapping JSON outside `internal/outbox/<domain>/`
- [ ] Mapping `.json` file name matches the domain name used in `index.go`
- [ ] No domain-specific types or imports leaked into `pkg/search/`

## Output format

```
## Search PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### Component Matrix
| Component | File | Status | Notes |
|-----------|------|--------|-------|
| Search interface | pkg/search/search.go | PASS | — |
| Embedded mappings | internal/outbox/<domain>/mappings/<domain>.json | PASS | — |
| Index + document | internal/outbox/<domain>/index.go | PASS | — |
| Index worker | internal/outbox/<domain>/event_index.go | PASS | — |
| Wiring | cmd/server/setup_connections.go | PASS | — |

### Import Audit
| Package | Import | Allowed | Status |
|---------|--------|---------|--------|
| pkg/search/ | opensearch-go | yes | PASS |
| pkg/search/ | gen/db/<domain> | NO | FAIL |
| internal/outbox/<domain>/ | pkg/search/ | yes | PASS |
| internal/outbox/<domain>/ | gen/db/<domain> | yes | PASS |
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
