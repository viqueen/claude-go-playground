# Proto Agent

Add the API contract and data model for a domain. No Go business logic — just the proto definition,
SQL migration, and sqlc queries. This PR is auditable as: **"Is the data model and API contract right?"**

## Inputs

The user will specify:
- The **domain name** (e.g., `content`, `workspace`, `user`)
- The **resource fields** and **operations** needed
- Any specific constraints or relationships

## What to generate

### 1. Proto Definition — `api/<domain>/v1/<domain>.proto`

```protobuf
syntax = "proto3";
package <domain>.v1;

import "buf/validate/validate.proto";

// Resource message
// Create/Get/List/Update/Delete request/response messages
// Service definition with RPCs
```

- Use `buf/validate/validate.proto` for field validation (e.g., `[(buf.validate.field).required = true]`)
- Follow standard CRUD naming: `Create<Resource>`, `Get<Resource>`, `List<Resource>`, `Update<Resource>`, `Delete<Resource>`
- List RPCs should support `page_size` + `page_token` pagination
- Include `buf.yaml` in the proto directory if not already present

### 2. SQL Migration — `sql/migrations/<NNN>_create_<domain>.sql`

```sql
-- +goose Up
CREATE TABLE <domain> ( ... );

-- +goose Down
DROP TABLE IF EXISTS <domain>;
```

- Use `UUID PRIMARY KEY DEFAULT gen_random_uuid()` for IDs
- Include `created_at TIMESTAMPTZ NOT NULL DEFAULT now()` and `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- Number the migration sequentially after existing ones

### 3. sqlc Queries — `sql/queries/<domain>/<domain>.sql`

```sql
-- name: Get<Resource> :one
-- name: List<Resource> :many
-- name: Count<Resource> :one
-- name: Create<Resource> :one
-- name: Update<Resource> :one  (using sqlc.narg for nullable fields)
-- name: Delete<Resource> :execrows
```

- Use `sqlc.arg('name')` for required params
- Use `sqlc.narg('name')` with `COALESCE` for optional update fields
- Update queries must include `updated_at = now()`

### 4. sqlc.yaml Update

Add a new entry to the `sql:` list in `sqlc.yaml`:

```yaml
- engine: "postgresql"
  queries: "sql/queries/<domain>/"
  schema: "sql/migrations/"
  gen:
    go:
      package: "<domain>"
      out: "gen/sqlc/<domain>"
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

### 5. buf.yaml (if not present) — `api/<domain>/v1/buf.yaml`

```yaml
version: v2
deps:
  - buf.build/bufbuild/protovalidate
```

## Post-Generation

1. Run `make codegen` to generate Go code from proto + sqlc
2. Run `make vet` — should pass (no new Go source files reference gen/ yet)

## Checklist

- [ ] Proto file with all RPCs, proper validation annotations
- [ ] Migration with correct schema, goose annotations, sequential numbering
- [ ] sqlc queries cover all CRUD operations
- [ ] sqlc.yaml updated with new domain entry
- [ ] buf.yaml present with protovalidate dependency
- [ ] `make codegen` succeeds
