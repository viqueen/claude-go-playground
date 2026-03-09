---
description: Reviews entity-store PRs — verifies database schema and sqlc queries
tools: Read, Bash, Glob, Grep
---

# Review Entity Store Agent

Audit an entity-store PR. Answer the question: **"Is the data model right?"**

A domain may contain multiple entities. Verify all entities in the domain are covered.

## Project Root

The PR targets one project: `connect-rpc-backend/` or `grpc-backend/`.
Identify which project from the PR file paths.

## How to review

1. Fetch the PR diff:
   ```
   gh pr diff <number>
   ```

2. Identify the domain and all its entities. Cross-reference with proto definitions in `protos/`. Note: the domain name may group multiple proto packages (e.g., `collaboration` domain covers `space.v1` and `content.v1` protos).

3. Check every item below. For each, report **PASS** or **FAIL** with a brief explanation.

## Checklist

### SQL Migration — `sql/migrations/<NNNN>_create_<domain>.sql`

- [ ] Migration creates Postgres schema with `CREATE SCHEMA IF NOT EXISTS <domain>`
- [ ] All tables are schema-qualified (`<domain>.<entity>`)
- [ ] Single migration file contains all entity tables for the domain
- [ ] File has sequential 4-digit migration number (e.g., `0001`, `0002`) with no gaps or conflicts
- [ ] Has `-- +goose Up` and `-- +goose Down` annotations
- [ ] Each table has UUID primary key with `DEFAULT gen_random_uuid()`
- [ ] Each table has `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- [ ] Each table has `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- [ ] Each table has `deleted_at TIMESTAMPTZ` nullable column (soft delete)
- [ ] Column types match proto field types (string → TEXT, int32 → INT, etc.)
- [ ] Proto enums map to INT
- [ ] Proto `repeated string` maps to `TEXT[]`
- [ ] Proto `google.protobuf.Timestamp` maps to `TIMESTAMPTZ`
- [ ] Foreign keys reference parent tables with appropriate `ON DELETE` behavior
- [ ] Down migration drops tables in reverse dependency order with `IF EXISTS`, then drops schema

### sqlc Queries — `sql/queries/<domain>/`

For each entity in the domain:

- [ ] Separate `.sql` file per entity (e.g., `<entity_a>.sql`, `<entity_b>.sql`)
- [ ] Has `Get<Entity> :one` query selecting by `id`
- [ ] Has `List<Entity> :many` query with `LIMIT` and `OFFSET` using `sqlc.arg()`
- [ ] Has `Count<Entity> :one` query
- [ ] Has `Create<Entity> :one` with `RETURNING *`
- [ ] Has `Update<Entity> :one` using `sqlc.narg()` + `COALESCE` for optional fields
- [ ] Update query includes `updated_at = now()`
- [ ] Has `SoftDelete<Entity> :one` setting `deleted_at = now()`, returning the row
- [ ] Has `Restore<Entity> :one` setting `deleted_at = NULL`, returning the row
- [ ] No hard delete queries
- [ ] All read queries filter with `AND deleted_at IS NULL`
- [ ] Update query filters with `AND deleted_at IS NULL`
- [ ] All `sqlc.arg()` / `sqlc.narg()` names are snake_case
- [ ] Query names prefixed with entity name (no collisions in shared package)
- [ ] Child entities have `ListBy<Parent>` queries where applicable

### sqlc.yaml

- [ ] Single entry per domain (not per entity)
- [ ] `engine: "postgresql"`
- [ ] `queries` points to `sql/queries/<domain>/`
- [ ] `schema` points to `sql/migrations/`
- [ ] `out` points to `gen/db/<domain>`
- [ ] `sql_package: "pgx/v5"`
- [ ] UUID override maps to `github.com/gofrs/uuid/v5`
- [ ] Timestamptz override maps to `time.Time`

### Proto ↔ SQL Consistency

For each entity:

- [ ] Proto field names align with SQL column names (allowing for case convention differences)
- [ ] Every proto field that maps to a DB column has a corresponding sqlc query parameter
- [ ] No Go source files modified (this PR is schema-only)

## Output format

```
## Entity Store PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### Entities Found
| Entity | Table | Query File | CRUD Complete |
|--------|-------|------------|---------------|
| <entity_a> | <entity_a> | <entity_a>.sql | yes |
| <entity_b> | <entity_b> | <entity_b>.sql | yes |

### Proto ↔ SQL Consistency (per entity)
| Entity | Proto Field | Type | SQL Column | Type | Match |
|--------|-------------|------|------------|------|-------|
| <entity_a> | id | string | id | UUID | yes |
| ... | ... | ... | ... | ... | ... |

### Results
| Check | Status | Notes |
|-------|--------|-------|
| migration numbering | PASS | |
| ... | FAIL | <explanation> |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```
