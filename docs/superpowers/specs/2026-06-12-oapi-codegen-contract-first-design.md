# Contract-First API Codegen (oapi-codegen) — Design

**Date:** 2026-06-12
**Status:** Approved, pending implementation plan

## Goal

Replace swaggo code-first Swagger generation with contract-first codegen using
`oapi-codegen`. `api/openapi.yaml` (OpenAPI 3.0) becomes the single source of
truth. From it we generate Go types, a chi server interface, request-validation
middleware, and a typed client. Swagger UI is kept but points at the authored
spec instead of swaggo's generated `doc.json`.

## Decisions (locked)

| Topic | Decision |
|-------|----------|
| Migration scope | Full replace — drop swaggo entirely |
| Spec source | Convert existing `docs/swagger/swagger.yaml` (2.0) → OpenAPI 3, then hand-clean |
| Spec layout | Single file: `api/openapi.yaml` |
| UI renderer | Swagger UI, serving the authored OpenAPI 3 spec |
| Routing | Generated registration (spec drives routes; build fails on missing operation) |
| Validation | Push max expressible rules into the spec; delete now-redundant validators |
| Request validation middleware | Yes (request-only, not responses) |
| Typed client | Yes (for integration tests / downstream consumers) |

## Direction Flip

```
NOW:  Go annotations --swag init--> swagger.yaml (2.0) --> swaggo UI
NEW:  api/openapi.yaml (3.0) --oapi-codegen--> {types, server iface, client, validation} --> Go
                              \--go:embed Swagger UI--> /swagger
```

swaggo's only role was code → spec generation plus a bundled UI helper. Both are
removed. The spec is authored and reviewed; code is the byproduct.

## Architecture

### Layout

```
api/
  openapi.yaml            # SOURCE OF TRUTH (converted from current 2.0, hand-cleaned)
  cfg/
    types.yaml            # oapi-codegen config: models only
    server.yaml           # chi-server interface + registration
    client.yaml           # typed client
internal/api/             # generated package (git-tracked, regenerated)
  types.gen.go
  server.gen.go           # ServerInterface + HandlerFromMux registration
  client.gen.go
```

### Components

1. **Spec (`api/openapi.yaml`)** — converted from `docs/swagger/swagger.yaml`
   via `swagger2openapi`, then hand-cleaned. Models all current endpoints:
   - auth: `POST /auth/register`, `POST /auth/login`, `GET /me`
   - todos: list / create / get / update / delete
   - admin: approved-users list / create / bulk / delete

   Adds an `operationId` per route, `BearerAuth` security scheme, per-operation
   `security` tags, validation constraints, and a reusable `ErrorResponse`
   schema that mirrors `internal/http.ErrorResponse`.

2. **Codegen** — `oapi-codegen` + `kin-openapi`. Three config files → three
   output files. `make openapi` runs all three. Replaces `make swagger`.

3. **Handler refactor** — each domain `Handler` implements the generated
   `ServerInterface`. The interface fixes one method per `operationId` and
   supplies typed path/query params already parsed. Existing service layer,
   `http2.Respond*` helpers, and auth context usage stay. Only handler
   signatures + body/param plumbing change.

4. **Validation middleware** — `OapiRequestValidator(swagger)` mounted on
   `/api/v1`, validating body/params/required/enums against the spec before
   handlers run. A custom error handler reshapes validator output into the
   existing `http.ErrorResponse` body. Rules expressible in the spec
   (title `minLength`/`maxLength`, `due_date` `format: date-time`, required
   fields, UUID path format) move into the spec; the corresponding manual
   validators are deleted.

5. **Router rewire (generated registration)** — routes come from the spec, not
   hand-written `r.Get/Post` calls. The generated `HandlerFromMux` registers
   each operation. The existing security grouping (public / `RequireAuth` /
   `RequireAdmin`) is preserved by registering operations onto the matching chi
   sub-router (multiple registration calls / per-operation middleware keyed by
   `operationId`). Missing an operation fails the build.

6. **Swagger UI** — drop `swaggo/http-swagger`. New `mountSwagger` uses
   `go:embed` to serve Swagger UI static assets that load `api/openapi.yaml` at
   `/swagger`. Same build-tag gating (`swagger.go` / `swagger_stub.go`), same
   route, same `swaggerEnabled` flag.

## Data Flow (todos/create example)

```
Request → root middleware (reqID, realIP, log, recover, security, CORS)
        → /api/v1 group (timeout, rateLimit)
        → OapiRequestValidator   (body/params/required/enums vs spec)
        → RequireAuth group middleware
        → generated wrapper (decodes typed request body / params)
        → Handler.CreateTodo(w, r)   (user from ctx, call svc, respond)
        → http2.RespondJSON
```

Spec owns structural validation; service layer owns business rules (ownership,
existence) the spec cannot express.

## Error Handling

- **Validation failures** → 400 via a custom OapiRequestValidator error handler
  that emits the existing `http.ErrorResponse` shape (no leaked kin-openapi
  default text).
- **Auth / not-found / internal** — unchanged, still handler-driven via
  `http2.RespondError`.
- The spec `ErrorResponse` schema mirrors `internal/http.ErrorResponse` so docs
  match runtime reality.

## Testing

- **Spec lint** in CI — spec must be valid OpenAPI 3 (`redocly lint` or `vacuum`).
- **Drift check** — run `make openapi`, then `git diff --exit-code internal/api`;
  stale generated code fails CI (mirrors the existing swagger-in-CI pattern).
- **Existing handler/integration tests** — kept, adapted to new signatures;
  they become the contract-conformance net.
- **Optional** — one test exercising the generated typed client against the test
  server, proving client and server agree on the contract.

## Migration Steps (high level)

1. Add tools (`oapi-codegen`, `kin-openapi`, `swagger2openapi`); pin in
   `install-tools`.
2. Convert spec → `api/openapi.yaml`; hand-clean; add `operationId`, validation
   constraints, `security` tags.
3. Add 3 oapi-codegen configs + `make openapi`; generate `internal/api/*.gen.go`.
4. Refactor handlers (auth, todo) to implement `ServerInterface`; wire generated
   registration per security sub-router.
5. Mount `OapiRequestValidator` + custom error handler; delete redundant
   validators.
6. Replace Swagger UI with `go:embed` assets serving `api/openapi.yaml`; drop
   `swaggo/http-swagger` + `swaggo/swag`.
7. CI: replace `make swagger` with `make openapi` + drift check + spec lint.
8. Remove swaggo deps from `go.mod`, delete `docs/swagger/`, strip `@Summary`
   annotations.

## Out of Scope (YAGNI)

- Per-module spec split (single file until it hurts).
- Response validation middleware (request-only).
- Multi-version API.
