---
description: Define proto files for a domain
argument-hint: <domain> <project>
allowed-tools: Read, Write, Edit, Bash, Glob, Grep
disable-model-invocation: true
context: fork
---

Domain: $0
Project: $1

# Proto Agent

Add the API contract for a domain. No Go business logic, no SQL — just the proto definition.
This PR is auditable as: **"Is the API contract right?"**

## Project Root

All file paths are relative to the chosen project: `connect-rpc-backend/` or `grpc-backend/`.
The user will specify which project. All `make` commands must be run from the project root.

## Inputs

The user will specify:
- The **domain name** (e.g., `content`, `workspace`, `user`)
- The **resource fields** and **operations** needed
- Any specific constraints or relationships

## What to generate

### 1. Proto Files — `protos/<domain>/v1/`

Split into three files per domain:

#### `protos/<domain>/v1/<domain>_model.proto` — Resource + Enums

```protobuf
syntax = "proto3";

package <domain>.v1;

import "buf/validate/validate.proto";
import "google/protobuf/timestamp.proto";

option go_package = "<domain>/v1;<domain>v1";

enum <Resource>Status {
  <RESOURCE>_STATUS_UNSPECIFIED = 0;
  <RESOURCE>_STATUS_DRAFT = 1;
  <RESOURCE>_STATUS_PUBLISHED = 2;
  <RESOURCE>_STATUS_ARCHIVED = 3;
}

message <Resource> {
  string id = 1;
  string title = 2 [(buf.validate.field).string = {min_len: 1, max_len: 255}];
  string body = 3 [(buf.validate.field).string.min_len = 1];
  <Resource>Status status = 4 [(buf.validate.field).enum = {defined_only: true}];
  repeated string tags = 5;
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp updated_at = 7;
}
```

Conventions:
- Enum zero value is always `_UNSPECIFIED = 0`
- Use `google.protobuf.Timestamp` for time fields (not strings)
- Resource message holds the full representation returned in responses
- `go_package` uses the alias format: `<domain>/v1;<domain>v1`
- Model messages MUST have `buf/validate` annotations — these are enforced when the model is embedded in Update requests with FieldMask
- Cross-domain relationships use Ref types (e.g., `space.v1.SpaceRef`) in the model, not plain `string` IDs

#### `protos/<domain>/v1/<domain>_refs.proto` — Typed ID References

```protobuf
syntax = "proto3";

package <domain>.v1;

import "buf/validate/validate.proto";

option go_package = "<domain>/v1;<domain>v1";

message <Resource>Ref {
  string id = 1 [(buf.validate.field).string.uuid = true];
}
```

Conventions:
- Refs are for **cross-package** use — when another package needs to reference this entity
- Within the same package, request messages use plain `string id` fields directly
- Ref messages enforce UUID validation at the proto level

#### `protos/<domain>/v1/<domain>_service.proto` — Service + Request/Response

```protobuf
syntax = "proto3";

package <domain>.v1;

import "buf/validate/validate.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/field_mask.proto";
import "<domain>/v1/<domain>_model.proto";

option go_package = "<domain>/v1;<domain>v1";

service <Resource>Service {
  rpc Create<Resource>(Create<Resource>Request) returns (Create<Resource>Response);
  rpc Get<Resource>(Get<Resource>Request) returns (Get<Resource>Response);
  rpc List<Resource>(List<Resource>Request) returns (List<Resource>Response);
  rpc Update<Resource>(Update<Resource>Request) returns (Update<Resource>Response);
  rpc Delete<Resource>(Delete<Resource>Request) returns (google.protobuf.Empty);
}

// Create

message Create<Resource>Request {
  string title = 1 [(buf.validate.field).string = {min_len: 1, max_len: 255}];
  string body = 2 [(buf.validate.field).string.min_len = 1];
  <Resource>Status status = 3 [(buf.validate.field).enum = {defined_only: true, not_in: [0]}];
  repeated string tags = 4;
}

message Create<Resource>Response {
  <Resource> <resource> = 1;
}

// Get

message Get<Resource>Request {
  string id = 1 [(buf.validate.field).string.uuid = true];
}

message Get<Resource>Response {
  <Resource> <resource> = 1;
}

// List

message List<Resource>Request {
  int32 page_size = 1 [(buf.validate.field).int32 = {gte: 0, lte: 100}];
  string page_token = 2;
}

message List<Resource>Response {
  repeated <Resource> items = 1;
  string next_page_token = 2;
}

// Update

message Update<Resource>Request {
  string id = 1 [(buf.validate.field).string.uuid = true];
  <Resource> <resource> = 2 [(buf.validate.field).required = true];
  google.protobuf.FieldMask update_mask = 3 [(buf.validate.field).required = true];
}

message Update<Resource>Response {
  <Resource> <resource> = 1;
}

// Delete

message Delete<Resource>Request {
  string id = 1 [(buf.validate.field).string.uuid = true];
}
```

Conventions:
- CRUD naming: `Create<Resource>`, `Get<Resource>`, `List<Resource>`, `Update<Resource>`, `Delete<Resource>`
- Every RPC has a `<RpcName>Request` and `<RpcName>Response` message pair — except Delete, which returns `google.protobuf.Empty` (errors use gRPC status codes)
- Response messages that return an entity wrap it in a named field (e.g., `Create<Resource>Response { <Resource> <resource> = 1; }`)
- Within the same package, requests use plain `string id` fields (not Ref types)
- List response items field is always named `items` for consistency across all entities
- IDs validated as UUID: `[(buf.validate.field).string.uuid = true]`
- Strings validated with length bounds: `{min_len: 1, max_len: 255}`
- Enums validated to exclude unspecified: `{defined_only: true, not_in: [0]}`
- Pagination: `page_size` with range validation `{gte: 0, lte: 100}` + `page_token` — `0` means "use server default" (AIP-158)
- Update uses `google.protobuf.FieldMask` to specify which fields to update
- Update embeds the full model message (not a separate payload) — FieldMask gates which fields are applied
- Update `<resource>` and `update_mask` are both required
- Cross-domain references: Create requests use Ref types (e.g., `SpaceRef`) to express ownership; Get/List/Update/Delete operate by ID alone — the API does not assume access scoping by parent

### 2. buf.yaml — `protos/buf.yaml`

Lives at the protobuf root (not per-domain). Create only if it doesn't already exist.

```yaml
version: v2
deps:
  - buf.build/bufbuild/protovalidate
```

## Post-Generation

1. Run `make codegen` to generate Go code from proto
2. Run `make vet` — should pass (no new Go source files reference gen/ yet)

## Checklist

- [ ] Three proto files: `_model.proto`, `_refs.proto`, `_service.proto`
- [ ] Proto package matches directory: `<domain>.v1` under `protos/<domain>/v1/`
- [ ] `go_package` uses alias format: `<domain>/v1;<domain>v1`
- [ ] Enums have `_UNSPECIFIED = 0` zero value
- [ ] Time fields use `google.protobuf.Timestamp`
- [ ] Update RPC uses `google.protobuf.FieldMask`
- [ ] All ID fields validated as UUID: `[(buf.validate.field).string.uuid = true]`
- [ ] String fields have length validation
- [ ] Enum fields exclude unspecified: `{defined_only: true, not_in: [0]}`
- [ ] Model message has `buf/validate` annotations (enforced via FieldMask updates)
- [ ] Model uses Ref types for cross-domain relationships (not plain string IDs)
- [ ] Pagination: `page_size` with `{gte: 0, lte: 100}` + `page_token` / `next_page_token`
- [ ] Ref message defined for cross-package ID references (within same package, use plain `string id`)
- [ ] List response items field named `items`
- [ ] Response messages wrap entity in a named field
- [ ] Delete RPCs return `google.protobuf.Empty` (not a response with `bool success`)
- [ ] Cross-domain refs only on Create requests (Get/List/Update/Delete operate by ID alone)
- [ ] `protos/buf.yaml` present at protobuf root with protovalidate dependency
- [ ] No SQL files in this PR (entity-store agent handles that)
- [ ] `make codegen` succeeds
