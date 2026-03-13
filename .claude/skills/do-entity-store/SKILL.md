---
description: Create SQL migration and sqlc queries for a domain
argument-hint: <domain> <project>
allowed-tools: Read, Write, Edit, Bash, Glob, Grep
disable-model-invocation: true
context: fork
---

Domain: $0
Project: $1

# Entity Store Agent

Add the database schema and queries for a domain's entity store. A domain may have multiple
entities — each gets its own table but they share a single migration file, a single sqlc
query directory, and a single generated Go package. This PR is auditable as: **"Is the data model right?"**

## Project Root

All file paths are relative to the chosen project: `connect-rpc-backend/` or `grpc-backend/`.
The user will specify which project. All `make` commands must be run from the project root.

## Inputs

The user will specify:
- The **domain name** (e.g., `collaboration`, `billing`) — a domain may group multiple proto packages (e.g., `collaboration` covers `space.v1` and `content.v1` protos)
- The **entities** in that domain with their fields and types
- Any indexes, constraints, or relationships between entities

Cross-reference with the proto definitions in `protos/` to ensure the schema
aligns with the API contract. The domain name does not need to match any single proto package.

## What to generate

### 1. SQL Migration — `sql/migrations/<NNNN>_create_<domain>.sql`

A single migration file per domain. Creates a dedicated Postgres schema for the domain
and all entity tables within it.

```sql
-- +goose Up
CREATE SCHEMA IF NOT EXISTS <domain>;

CREATE TABLE <domain>.<entity_a> (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    status     INT NOT NULL DEFAULT 1,
    tags       TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE <domain>.<entity_b> (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    <entity_a>_id UUID NOT NULL REFERENCES <domain>.<entity_a>(id) ON DELETE CASCADE,
    content      TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);

-- +goose Down
DROP TABLE IF EXISTS <domain>.<entity_b>;
DROP TABLE IF EXISTS <domain>.<entity_a>;
DROP SCHEMA IF EXISTS <domain>;
```

Conventions:
- Each domain gets its own Postgres schema (`CREATE SCHEMA IF NOT EXISTS <domain>`)
- All entity tables are created under `<domain>.<table>` (schema-qualified)
- Primary key is `UUID DEFAULT gen_random_uuid()`
- `created_at` and `updated_at` are `TIMESTAMPTZ NOT NULL DEFAULT now()`
- `deleted_at` is `TIMESTAMPTZ` nullable — `NULL` means active, non-NULL means soft-deleted
- Proto enums map to `INT` in SQL
- Proto `repeated string` maps to `TEXT[]` in SQL
- Proto `google.protobuf.Timestamp` maps to `TIMESTAMPTZ`
- Foreign keys use `REFERENCES <table>(id)` with appropriate `ON DELETE` behavior
- Down migration drops tables in reverse order (dependents first)
- Number the migration sequentially with 4-digit format (e.g., `0001`, `0002`)
- If a placeholder stub migration exists, replace it with the real migration

### 2. sqlc Queries — `sql/queries/<domain>/`

One query file per entity within the domain directory:

```
sql/queries/<domain>/
├── <entity_a>.sql
└── <entity_b>.sql
```

Each file contains the full CRUD surface for that entity:

```sql
-- name: Get<EntityA> :one
SELECT * FROM <domain>.<entity_a> WHERE id = sqlc.arg('id') AND deleted_at IS NULL;

-- name: List<EntityA> :many
SELECT * FROM <domain>.<entity_a>
WHERE deleted_at IS NULL
ORDER BY created_at LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: Count<EntityA> :one
SELECT count(*) FROM <domain>.<entity_a> WHERE deleted_at IS NULL;

-- name: Create<EntityA> :one
INSERT INTO <domain>.<entity_a> (title, body, status, tags)
VALUES (sqlc.arg('title'), sqlc.arg('body'), sqlc.arg('status'), sqlc.arg('tags'))
RETURNING *;

-- name: Update<EntityA> :one
UPDATE <domain>.<entity_a>
SET title = COALESCE(sqlc.narg('title'), title),
    body = COALESCE(sqlc.narg('body'), body),
    status = COALESCE(sqlc.narg('status'), status),
    tags = COALESCE(sqlc.narg('tags'), tags),
    updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDelete<EntityA> :one
UPDATE <domain>.<entity_a>
SET deleted_at = now(), updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NULL
RETURNING *;

-- name: Restore<EntityA> :one
UPDATE <domain>.<entity_a>
SET deleted_at = NULL, updated_at = now()
WHERE id = sqlc.arg('id') AND deleted_at IS NOT NULL
RETURNING *;
```

For child entities, include queries that filter by parent:

```sql
-- name: List<EntityB>By<EntityA> :many
SELECT * FROM <domain>.<entity_b>
WHERE <entity_a>_id = sqlc.arg('<entity_a>_id') AND deleted_at IS NULL
ORDER BY created_at LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');
```

Conventions:
- One `.sql` file per entity, all under `sql/queries/<domain>/`
- `sqlc.arg('name')` for required params
- `sqlc.narg('name')` with `COALESCE` for optional update fields (generates `pgtype.Text`, `pgtype.Int4`, etc. — NOT pointer types)
- Update queries always set `updated_at = now()`
- All read queries filter with `AND deleted_at IS NULL` (exclude soft-deleted rows)
- Update queries filter with `AND deleted_at IS NULL` (cannot update soft-deleted rows)
- `SoftDelete<Entity>` sets `deleted_at = now()` (returns the row for outbox events)
- `Restore<Entity>` sets `deleted_at = NULL` (un-deletes a soft-deleted row)
- No hard delete queries — all deletes are soft
- All param names are snake_case
- Child entities have `ListBy<Parent>` queries
- Query names are prefixed with the entity name to avoid collisions within the shared package

### 3. sqlc.yaml Update

Add a single entry per domain (not per entity) to the `sql:` list in `sqlc.yaml`.
All entities in the domain share one generated Go package:

```yaml
- engine: "postgresql"
  queries: "sql/queries/<domain>/"
  schema: "sql/migrations/"
  gen:
    go:
      package: "<domain>"
      out: "gen/db/<domain>"
      sql_package: "pgx/v5"
      overrides:
        - db_type: "uuid"
          go_type:
            import: "github.com/gofrs/uuid/v5"
            type: "UUID"
        - db_type: "timestamptz"
          go_type:
            import: "time"
            type: "Time"
```

Conventions:
- One sqlc entry per domain (covers all entities in that domain)
- Generated code goes to `gen/db/<domain>` as a single Go package
- UUID type override: `github.com/gofrs/uuid/v5`
- Timestamptz type override: `time.Time`
- SQL package: `pgx/v5`

## Post-Generation

1. Run `make codegen` to generate sqlc Go code
2. Run `make vet` — should pass (no new Go source files reference gen/ yet)

## Checklist

- [ ] Migration creates Postgres schema with `CREATE SCHEMA IF NOT EXISTS <domain>`
- [ ] All tables are schema-qualified (`<domain>.<entity>`)
- [ ] Single migration file per domain with all entity tables
- [ ] Migration has sequential numbering, `-- +goose Up` / `-- +goose Down`
- [ ] Down migration drops tables in reverse dependency order, then drops schema
- [ ] Each table has UUID primary key with `DEFAULT gen_random_uuid()`
- [ ] Each table has `created_at` and `updated_at` TIMESTAMPTZ NOT NULL columns
- [ ] Each table has `deleted_at` TIMESTAMPTZ nullable column (soft delete)
- [ ] Foreign keys reference parent tables with appropriate `ON DELETE` behavior
- [ ] Column types align with proto fields (enums → INT, repeated string → TEXT[], timestamps → TIMESTAMPTZ)
- [ ] One `.sql` query file per entity under `sql/queries/<domain>/`
- [ ] Each entity has full CRUD queries: Get, List, Count, Create, Update, SoftDelete, Restore
- [ ] Child entities have `ListBy<Parent>` queries
- [ ] Query names prefixed with entity name (no collisions in shared package)
- [ ] Update queries use `sqlc.narg()` + `COALESCE` for optional fields
- [ ] Update queries set `updated_at = now()`
- [ ] All read/update queries filter with `AND deleted_at IS NULL`
- [ ] SoftDelete query sets `deleted_at = now()` and returns the row (`:one`)
- [ ] Restore query sets `deleted_at = NULL` and returns the row (`:one`)
- [ ] No hard delete queries
- [ ] Single sqlc.yaml entry per domain, `out` points to `gen/db/<domain>`
- [ ] sqlc.yaml has UUID and timestamptz type overrides
- [ ] No Go source files in this PR (domain agent handles that)
- [ ] `make codegen` succeeds
