---
description: Reviews proto PRs — verifies API contract correctness
tools: Read, Bash, Glob, Grep
---

# Review Proto Agent

Audit a proto PR. Answer the question: **"Is the API contract right?"**

## Project Root

The PR targets one project: `connect-rpc-backend/` or `grpc-backend/`.
Identify which project from the PR file paths.

## How to review

1. Fetch the PR diff:
   ```
   gh pr diff <number>
   ```

2. Identify the domain being added and read the full files (not just the diff).

3. Check every item below. For each, report **PASS** or **FAIL** with a brief explanation.

## Checklist

### Proto Files — `protos/<domain>/v1/`

Three files per domain:

#### `<domain>_model.proto` — Resource + Enums
- [ ] `syntax = "proto3";` declared
- [ ] `package` matches directory structure (`<domain>.v1`)
- [ ] `go_package` uses alias format: `<domain>/v1;<domain>v1`
- [ ] Imports `google/protobuf/timestamp.proto` for time fields
- [ ] Resource message has `id` (string), `created_at`, `updated_at` fields
- [ ] Time fields use `google.protobuf.Timestamp` (not strings)
- [ ] Enums have `_UNSPECIFIED = 0` as zero value
- [ ] No `buf/validate` import here (validation lives in service + refs)

#### `<domain>_refs.proto` — Typed ID References (cross-package use)
- [ ] `Ref` message exists with `id` field
- [ ] ID validated as UUID: `[(buf.validate.field).string.uuid = true]`
- [ ] Refs are for cross-package references only; within same package, requests use plain `string id`

#### `<domain>_service.proto` — Service + Request/Response
- [ ] Imports model proto and `google/protobuf/field_mask.proto`
- [ ] CRUD RPCs: `Create<Resource>`, `Get<Resource>`, `List<Resource>`, `Update<Resource>`, `Delete<Resource>`
- [ ] Create request: string fields have length validation (`min_len`, `max_len`)
- [ ] Create request: enum fields exclude unspecified (`{defined_only: true, not_in: [0]}`)
- [ ] Get/Delete request: ID validated as UUID
- [ ] List request: `page_size` with range validation (`{gte: 1, lte: 100}`) + `page_token`
- [ ] List response: `items` (repeated, always named `items`) + `next_page_token`
- [ ] Response messages wrap entity in a named field (e.g., `Content content = 1`)
- [ ] Within same package, requests use plain `string id` (not Ref types)
- [ ] Update request: uses `google.protobuf.FieldMask` with `[(buf.validate.field).required = true]`
- [ ] Update request: resource field is required
- [ ] No business logic or computed fields in request messages

### buf.yaml — `protos/buf.yaml`

- [ ] Lives at the protobuf root (not per-domain)
- [ ] `version: v2`
- [ ] `deps` includes `buf.build/bufbuild/protovalidate`

### Scope

- [ ] No SQL files in this PR (entity-store agent handles that)
- [ ] No Go source files modified (this PR is proto-only)

## Output format

```
## Proto PR Audit — <domain>

### Summary
<one sentence: pass or issues found>

### Results
| Check | Status | Notes |
|-------|--------|-------|
| proto path | PASS | |
| ... | FAIL | <explanation> |

### Issues
<numbered list of FAIL items with details and suggested fixes>
```
