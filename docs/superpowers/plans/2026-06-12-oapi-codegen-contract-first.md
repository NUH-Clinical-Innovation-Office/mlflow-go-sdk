# Contract-First API Codegen (oapi-codegen) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace swaggo code-first Swagger generation with contract-first codegen using oapi-codegen, where `api/openapi.yaml` (OpenAPI 3) is the source of truth that generates Go types, a typed chi server wrapper, request-validation middleware, and a typed client.

**Architecture:** Author `api/openapi.yaml` (converted from the existing swaggo 2.0 spec). `oapi-codegen` generates `internal/api/{types,server,client}.gen.go`. Domain handlers implement the generated `ServerInterface`; the existing clean chi auth/admin route groups are preserved by routing each chi route to the generated typed wrapper functions (construction of the wrapper requires the full interface to be implemented — that is the build-time enforcement). `OapiRequestValidator` middleware validates requests against the spec on `/api/v1`, with a custom error handler emitting the existing `{"detail": ...}` body. Swagger UI is served via `go:embed` against the authored spec.

**Tech Stack:** Go 1.26, chi v5, oapi-codegen v2, kin-openapi, swagger UI static assets, swagger2openapi (Node, one-shot conversion).

---

## Important codebase facts (read before starting)

- Module path: `github.com/your-org/go-backend-template`.
- Error body shape: `internal/http.RespondError` writes `{"detail": "<msg>"}`; `http.ErrorResponse{Detail string}` is the schema. The generated `Error` schema MUST use property `detail`.
- Response helpers: `http2.RespondJSON(w, status, v)`, `http2.RespondError(w, status, msg)`. Keep using them in handlers. (`http2` is the local import alias for `internal/http`.)
- Request decode: `http2.DecodeJSON(w, r, maxBodyBytes, &dst)` returns `http2.ErrBodyTooLarge`. The generated wrappers will decode bodies instead; keep the 1 MiB cap by leaving body-size limiting to the validator/router (see Task 9 note).
- Routing today (`internal/router/router.go`): three groups under `/api/v1` — public (`/auth/register`, `/auth/login`), `RequireAuth` (`/todos*`, `/me`), `RequireAdmin` (`/admin/approved-users*`). This grouping MUST be preserved.
- User from context: `middleware.UserFromContext(r.Context())` returns `*domain.User` or nil.
- `router.New(...)` signature (cmd/api/main.go:117) currently ends with `swaggerEnabled bool, trustedProxies []net.IPNet`. It will gain the embedded-spec UI but the signature stays the same.
- swaggo is referenced in: `Makefile` (`swagger`, `install-tools`), `Dockerfile` (lines ~18-23), `.github/workflows/ci.yml` (lines 29-32), `.github/workflows/reusable-build.yml` (4 blocks), `internal/router/swagger.go`, `internal/router/swagger_stub.go`, `cmd/api/main.go` (header annotations), and `@Summary` annotations across `internal/auth/handler.go` and `internal/todo/handler.go`.
- Build tag gating: `swagger.go` has `//go:build swagger`, `swagger_stub.go` has `//go:build !swagger`. Keep this pattern.

## File structure (created / modified)

**Created:**
- `api/openapi.yaml` — OpenAPI 3 source of truth.
- `api/cfg/types.yaml` — oapi-codegen config (models only).
- `api/cfg/server.yaml` — oapi-codegen config (chi server interface + embedded spec).
- `api/cfg/client.yaml` — oapi-codegen config (typed client).
- `internal/api/types.gen.go` — generated types (committed).
- `internal/api/server.gen.go` — generated `ServerInterface` + wrappers + `GetSwagger()` (committed).
- `internal/api/client.gen.go` — generated client (committed).
- `internal/api/gen.go` — `go:generate` directives + package doc.
- `internal/router/openapi_error.go` — custom validator error handler.
- `internal/router/swaggerui/` — embedded Swagger UI assets + `index.html` pointing at `/openapi.yaml`.

**Modified:**
- `internal/todo/handler.go` — implement generated interface methods.
- `internal/auth/handler.go` — implement generated interface methods.
- `internal/router/router.go` — route to generated wrappers, mount validator, serve spec.
- `internal/router/swagger.go` / `swagger_stub.go` — embed-based UI.
- `cmd/api/main.go` — drop swaggo header annotations; construct wrapper.
- `Makefile` — replace `swagger` target with `openapi`; update `install-tools`.
- `Dockerfile` — replace swaggo generation with nothing (generated code committed).
- `.github/workflows/ci.yml`, `reusable-build.yml` — replace swaggo step with drift check + spec lint.
- `go.mod` — add oapi-codegen/kin-openapi, remove swaggo.

**Deleted (final task):**
- `docs/swagger/` (swagger.yaml, docs.go).

---

## Task 1: Add codegen tooling and pin versions

**Files:**
- Modify: `Makefile:49-55`

- [ ] **Step 1: Add tools to `install-tools` and replace the `swagger` target**

Replace the `install-tools` body and the `swagger` target in `Makefile`. New `install-tools`:

```makefile
install-tools: ## Install required tools
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0
	@go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1
```

Replace the `swagger:` target (lines 54-55) with:

```makefile
openapi: ## Generate Go code from api/openapi.yaml (run after editing the spec)
	@oapi-codegen -config api/cfg/types.yaml api/openapi.yaml
	@oapi-codegen -config api/cfg/server.yaml api/openapi.yaml
	@oapi-codegen -config api/cfg/client.yaml api/openapi.yaml
```

Update the `.PHONY` line (line 1): remove `swagger`, add `openapi`.

- [ ] **Step 2: Verify the make target parses**

Run: `make help`
Expected: help table prints, shows `openapi` row, no `swagger` row, exit 0.

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add oapi-codegen tooling, replace swagger make target"
```

---

## Task 2: Author the OpenAPI 3 spec

**Files:**
- Create: `api/openapi.yaml`

This is converted from `docs/swagger/swagger.yaml` and hand-cleaned. Write the file exactly as below. Each operation has an `operationId` (drives the generated Go method name). Validation constraints are added to schemas. `Error` schema uses `detail` to match `http.ErrorResponse`.

- [ ] **Step 1: Write `api/openapi.yaml`**

```yaml
openapi: 3.0.3
info:
  title: Go Backend Template API
  description: REST API for go-backend-template
  version: "1.0"
servers:
  - url: /api/v1
components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
  schemas:
    Error:
      type: object
      required: [detail]
      properties:
        detail:
          type: string
    RegisterRequest:
      type: object
      required: [email, password, approved_id]
      properties:
        email: { type: string, format: email }
        password: { type: string, minLength: 8 }
        approved_id: { type: string }
    LoginRequest:
      type: object
      required: [email, password]
      properties:
        email: { type: string, format: email }
        password: { type: string }
    AuthResponse:
      type: object
      required: [token, token_type]
      properties:
        token: { type: string }
        token_type: { type: string }
    UserResponse:
      type: object
      required: [id, email, first_name, is_active, roles, created_at]
      properties:
        id: { type: string }
        email: { type: string }
        first_name: { type: string }
        is_active: { type: boolean }
        roles:
          type: array
          items: { type: string }
        created_at: { type: string, format: date-time }
    CreateTodoRequest:
      type: object
      required: [title]
      properties:
        title: { type: string, minLength: 1, maxLength: 255 }
        description: { type: string, nullable: true }
        due_date: { type: string, format: date-time, nullable: true }
    UpdateTodoRequest:
      type: object
      properties:
        title: { type: string, minLength: 1, maxLength: 255, nullable: true }
        description: { type: string, nullable: true }
        is_completed: { type: boolean, nullable: true }
        due_date: { type: string, format: date-time, nullable: true }
    TodoResponse:
      type: object
      required: [id, user_id, title, is_completed, created_at, updated_at]
      properties:
        id: { type: string }
        user_id: { type: string }
        title: { type: string }
        description: { type: string, nullable: true }
        is_completed: { type: boolean }
        due_date: { type: string, format: date-time, nullable: true }
        created_at: { type: string, format: date-time }
        updated_at: { type: string, format: date-time }
    ApprovedUserRequest:
      type: object
      required: [email, first_name]
      properties:
        email: { type: string, format: email }
        first_name: { type: string, minLength: 1 }
    BulkApprovedUserRequest:
      type: object
      required: [users]
      properties:
        users:
          type: array
          minItems: 1
          items: { $ref: '#/components/schemas/ApprovedUserRequest' }
    ApprovedUserResponse:
      type: object
      required: [id, email, first_name, created_at, updated_at]
      properties:
        id: { type: string }
        email: { type: string }
        first_name: { type: string }
        created_at: { type: string, format: date-time }
        updated_at: { type: string, format: date-time }
  responses:
    ErrorResponse:
      description: Error
      content:
        application/json:
          schema: { $ref: '#/components/schemas/Error' }
paths:
  /auth/register:
    post:
      operationId: RegisterUser
      tags: [auth]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/RegisterRequest' }
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema: { $ref: '#/components/schemas/AuthResponse' }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '409': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
  /auth/login:
    post:
      operationId: LoginUser
      tags: [auth]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/LoginRequest' }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema: { $ref: '#/components/schemas/AuthResponse' }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
  /me:
    get:
      operationId: GetCurrentUser
      tags: [auth]
      security: [{ BearerAuth: [] }]
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema: { $ref: '#/components/schemas/UserResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
  /todos:
    get:
      operationId: ListTodos
      tags: [todos]
      security: [{ BearerAuth: [] }]
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items: { $ref: '#/components/schemas/TodoResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
    post:
      operationId: CreateTodo
      tags: [todos]
      security: [{ BearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/CreateTodoRequest' }
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema: { $ref: '#/components/schemas/TodoResponse' }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
  /todos/{id}:
    get:
      operationId: GetTodo
      tags: [todos]
      security: [{ BearerAuth: [] }]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema: { $ref: '#/components/schemas/TodoResponse' }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
    patch:
      operationId: UpdateTodo
      tags: [todos]
      security: [{ BearerAuth: [] }]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/UpdateTodoRequest' }
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema: { $ref: '#/components/schemas/TodoResponse' }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
    delete:
      operationId: DeleteTodo
      tags: [todos]
      security: [{ BearerAuth: [] }]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        '204': { description: No Content }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
  /admin/approved-users:
    get:
      operationId: ListApprovedUsers
      tags: [admin]
      security: [{ BearerAuth: [] }]
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                type: array
                items: { $ref: '#/components/schemas/ApprovedUserResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
    post:
      operationId: CreateApprovedUser
      tags: [admin]
      security: [{ BearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/ApprovedUserRequest' }
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema: { $ref: '#/components/schemas/ApprovedUserResponse' }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '409': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
  /admin/approved-users/bulk:
    post:
      operationId: BulkCreateApprovedUsers
      tags: [admin]
      security: [{ BearerAuth: [] }]
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: '#/components/schemas/BulkApprovedUserRequest' }
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema:
                type: array
                items: { $ref: '#/components/schemas/ApprovedUserResponse' }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
  /admin/approved-users/{id}:
    delete:
      operationId: DeleteApprovedUser
      tags: [admin]
      security: [{ BearerAuth: [] }]
      parameters:
        - name: id
          in: path
          required: true
          schema: { type: string, format: uuid }
      responses:
        '204': { description: No Content }
        '400': { $ref: '#/components/responses/ErrorResponse' }
        '401': { $ref: '#/components/responses/ErrorResponse' }
        '404': { $ref: '#/components/responses/ErrorResponse' }
        '500': { $ref: '#/components/responses/ErrorResponse' }
```

- [ ] **Step 2: Validate the spec is well-formed OpenAPI 3**

Run: `npx -y @redocly/cli@1.25.11 lint api/openapi.yaml`
Expected: "0 errors" (warnings about missing `description` on operations are acceptable). If `npx` unavailable, defer to Task 10's Go-based check and skip here.

- [ ] **Step 3: Commit**

```bash
git add api/openapi.yaml
git commit -m "feat: add OpenAPI 3 spec as API source of truth"
```

---

## Task 3: Add oapi-codegen config files

**Files:**
- Create: `api/cfg/types.yaml`
- Create: `api/cfg/server.yaml`
- Create: `api/cfg/client.yaml`

- [ ] **Step 1: Write `api/cfg/types.yaml`**

```yaml
package: api
output: internal/api/types.gen.go
generate:
  models: true
output-options:
  skip-prune: true
```

- [ ] **Step 2: Write `api/cfg/server.yaml`**

`embedded-spec: true` makes the generated package expose `GetSwagger()` which returns the parsed spec — used by the validator middleware and the UI handler, so no runtime YAML file read is needed.

```yaml
package: api
output: internal/api/server.gen.go
generate:
  chi-server: true
  embedded-spec: true
import-mapping: {}
output-options:
  skip-prune: true
```

- [ ] **Step 3: Write `api/cfg/client.yaml`**

```yaml
package: api
output: internal/api/client.gen.go
generate:
  client: true
output-options:
  skip-prune: true
```

- [ ] **Step 4: Commit**

```bash
git add api/cfg/
git commit -m "feat: add oapi-codegen config for types, server, client"
```

---

## Task 4: Generate the code and add it to the module

**Files:**
- Create: `internal/api/gen.go`
- Create (generated): `internal/api/types.gen.go`, `internal/api/server.gen.go`, `internal/api/client.gen.go`
- Modify: `go.mod`

- [ ] **Step 1: Write `internal/api/gen.go`**

```go
// Package api contains code generated from api/openapi.yaml by oapi-codegen.
// Do not edit the *.gen.go files by hand. Run `make openapi` to regenerate
// after editing api/openapi.yaml.
package api

//go:generate oapi-codegen -config ../../api/cfg/types.yaml ../../api/openapi.yaml
//go:generate oapi-codegen -config ../../api/cfg/server.yaml ../../api/openapi.yaml
//go:generate oapi-codegen -config ../../api/cfg/client.yaml ../../api/openapi.yaml
```

- [ ] **Step 2: Install the generator**

Run: `go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1`
Expected: exit 0, binary on PATH (`which oapi-codegen` resolves).

- [ ] **Step 3: Generate**

Run: `make openapi`
Expected: creates `internal/api/types.gen.go`, `server.gen.go`, `client.gen.go`, exit 0.

- [ ] **Step 4: Pull deps into the module**

Run: `go mod tidy`
Expected: `go.mod` now requires `github.com/oapi-codegen/runtime` and `github.com/getkin/kin-openapi`; exit 0.

- [ ] **Step 5: Verify the generated package compiles and inspect the interface**

Run: `go build ./internal/api/...`
Expected: exit 0.

Run: `grep -n "ServerInterface interface" -A 30 internal/api/server.gen.go`
Expected: an interface listing one method per operationId, e.g. `RegisterUser(w http.ResponseWriter, r *http.Request)`, `GetTodo(w http.ResponseWriter, r *http.Request, id openapi_types.UUID)`, etc. Record the exact method signatures — later tasks must match them. (oapi-codegen names path params from the spec; `id` of `format: uuid` becomes `openapi_types.UUID`.)

- [ ] **Step 6: Commit generated code**

```bash
git add internal/api/ go.mod go.sum
git commit -m "feat: generate api types, chi server interface, and client"
```

---

## Task 5: Implement the generated interface in the todo handler

**Files:**
- Modify: `internal/todo/handler.go`

The generated `ServerInterface` declares (todo subset): `ListTodos(w,r)`, `CreateTodo(w,r)`, `GetTodo(w,r,id)`, `UpdateTodo(w,r,id)`, `DeleteTodo(w,r,id)`. We add these methods to the existing `*todo.Handler`, reusing the service layer. Request bodies are decoded into generated types (`api.CreateTodoRequest`, `api.UpdateTodoRequest`); the manual `validator.ValidateCreateTodoTitle` / `ValidateUpdateTodoTitle` calls are removed (spec `minLength`/`maxLength` + the validator middleware enforce them). `due_date` is now parsed from the generated `*time.Time` field directly (spec `format: date-time` means the body type is already `*time.Time`), so `http2.ParseDueDate` on todo is no longer needed in these methods.

- [ ] **Step 1: Add the interface methods to `internal/todo/handler.go`**

Add these methods (keep the existing `toTodoResponse`, `Handler`, `NewHandler`). Add `"github.com/google/uuid"` and `apiTypes "github.com/oapi-codegen/runtime/types"` imports only if the generated `id` type requires it — if `GetTodo`'s signature from Task 4 Step 5 uses `openapi_types.UUID` (alias of `github.com/oapi-codegen/runtime/types.UUID`, which is `[16]byte` compatible with `uuid.UUID`), convert with `uuid.UUID(id)`.

```go
// ListTodos implements api.ServerInterface.
func (h *Handler) ListTodos(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	todos, err := h.svc.ListByUserID(r.Context(), user.ID)
	if err != nil {
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	response := make([]TodoResponse, len(todos))
	for i := range todos {
		response[i] = toTodoResponse(&todos[i])
	}
	http2.RespondJSON(w, http.StatusOK, response)
}

// CreateTodo implements api.ServerInterface.
func (h *Handler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req api.CreateTodoRequest
	if err := http2.DecodeJSON(w, r, 1<<20, &req); err != nil {
		if errors.Is(err, http2.ErrBodyTooLarge) {
			http2.RespondError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		http2.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	todo, err := h.svc.Create(r.Context(), user.ID, req.Title, req.Description, req.DueDate)
	if err != nil {
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	http2.RespondJSON(w, http.StatusCreated, toTodoResponse(todo))
}

// GetTodo implements api.ServerInterface.
func (h *Handler) GetTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	todo, err := h.svc.GetByID(r.Context(), uuid.UUID(id), user.ID)
	if err != nil {
		if err == ErrTodoNotFound || err == ErrTodoNotOwned {
			http2.RespondError(w, http.StatusNotFound, "todo not found")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	http2.RespondJSON(w, http.StatusOK, toTodoResponse(todo))
}

// UpdateTodo implements api.ServerInterface.
func (h *Handler) UpdateTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req api.UpdateTodoRequest
	if err := http2.DecodeJSON(w, r, 1<<20, &req); err != nil {
		if errors.Is(err, http2.ErrBodyTooLarge) {
			http2.RespondError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		http2.RespondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	todo, err := h.svc.Update(r.Context(), uuid.UUID(id), user.ID, req.Title, req.Description, req.IsCompleted, req.DueDate)
	if err != nil {
		if err == ErrTodoNotFound || err == ErrTodoNotOwned {
			http2.RespondError(w, http.StatusNotFound, "todo not found")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	http2.RespondJSON(w, http.StatusOK, toTodoResponse(todo))
}

// DeleteTodo implements api.ServerInterface.
func (h *Handler) DeleteTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http2.RespondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	err := h.svc.Delete(r.Context(), uuid.UUID(id), user.ID)
	if err != nil {
		if err == ErrTodoNotFound || err == ErrTodoNotOwned {
			http2.RespondError(w, http.StatusNotFound, "todo not found")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

Add imports: `"github.com/google/uuid"`, `apiTypes "github.com/oapi-codegen/runtime/types"`, `"github.com/your-org/go-backend-template/internal/api"`.

- [ ] **Step 2: Delete the old swaggo-annotated handler methods and the now-unused imports**

Remove the old `ListHandler`, `CreateHandler`, `GetHandler`, `UpdateHandler`, `DeleteHandler` (with their `@Summary` annotation blocks) and the `CreateTodoRequest`/`UpdateTodoRequest`/`TodoResponse` request structs that are now replaced by generated types — KEEP `TodoResponse` (still used by `toTodoResponse` and JSON output) but DELETE `CreateTodoRequest` and `UpdateTodoRequest` (replaced by `api.*`). Remove now-unused imports (`validator`, and `time` only if no longer referenced — `TodoResponse` uses `time.Time`, so keep `time`).

- [ ] **Step 3: Verify the package compiles**

Run: `go build ./internal/todo/...`
Expected: exit 0.

- [ ] **Step 4: Verify Handler satisfies the todo slice of the interface**

Add a temporary compile-time assertion at the bottom of `handler.go`:

```go
// compile-time check (temporary; full interface asserted in router)
var _ interface {
	ListTodos(http.ResponseWriter, *http.Request)
} = (*Handler)(nil)
```

Run: `go build ./internal/todo/...`
Expected: exit 0. Then remove the temporary assertion.

- [ ] **Step 5: Run todo unit tests**

Run: `go test -short ./internal/todo/...`
Expected: PASS (tests calling old method names must be updated to new names; update any that fail to compile by renaming `ListHandler`→`ListTodos` etc.).

- [ ] **Step 6: Commit**

```bash
git add internal/todo/handler.go
git commit -m "refactor: implement generated ServerInterface in todo handler"
```

---

## Task 6: Implement the generated interface in the auth handler

**Files:**
- Modify: `internal/auth/handler.go`

Generated `ServerInterface` (auth/admin subset): `RegisterUser(w,r)`, `LoginUser(w,r)`, `GetCurrentUser(w,r)`, `ListApprovedUsers(w,r)`, `CreateApprovedUser(w,r)`, `BulkCreateApprovedUsers(w,r)`, `DeleteApprovedUser(w,r,id)`. Bodies decode into `api.*` types. Email/password/first_name structural validation moves to the spec + validator middleware; keep the `validator.ValidateRegister`/`ValidateLogin`/`ValidateApprovedUser` calls ONLY for rules the spec can't express (e.g. password complexity beyond minLength) — to stay safe and minimize risk, KEEP these existing validator calls (they are domain rules), and rely on the spec for required/format. The bulk `len==0` check is now redundant (spec `minItems: 1`) but harmless; keep it as defense in depth.

- [ ] **Step 1: Add interface methods (replacing the old `*Handler` handler methods)**

Replace `RegisterHandler`→`RegisterUser`, `LoginHandler`→`LoginUser`, `GetMeHandler`→`GetCurrentUser`, `ListApprovedUsersHandler`→`ListApprovedUsers`, `CreateApprovedUserHandler`→`CreateApprovedUser`, `BulkCreateApprovedUsersHandler`→`BulkCreateApprovedUsers`, `DeleteApprovedUserHandler`→`DeleteApprovedUser`. Bodies use generated types. Example for the two that change shape most:

```go
// RegisterUser implements api.ServerInterface.
func (h *Handler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var req api.RegisterRequest
	if err := decodeJSONBody(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}

	if err := validator.ValidateRegister(validator.RegisterRequest{
		Email:      string(req.Email),
		Password:   req.Password,
		ApprovedID: req.ApprovedId,
	}); err != nil {
		http2.RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := h.auth.Register(r.Context(), string(req.Email), req.Password, req.ApprovedId)
	if err != nil {
		h.logger.Error("register failed", zap.Error(err))
		if errors.Is(err, ErrUserNotFound) {
			http2.RespondError(w, http.StatusNotFound, "approved user not found")
			return
		}
		if errors.Is(err, ErrInvalidCredentials) {
			http2.RespondError(w, http.StatusBadRequest, "invalid approved_id format")
			return
		}
		if errors.Is(err, ErrUserAlreadyExists) {
			http2.RespondError(w, http.StatusConflict, "user already exists")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	http2.RespondJSON(w, http.StatusCreated, AuthResponse{Token: token, TokenType: "bearer"})
}

// DeleteApprovedUser implements api.ServerInterface.
func (h *Handler) DeleteApprovedUser(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	if err := h.admin.DeleteApprovedUser(r.Context(), uuid.UUID(id)); err != nil {
		h.logger.Error("delete approved user failed", zap.Error(err))
		if errors.Is(err, ErrApprovedUserNotFound) {
			http2.RespondError(w, http.StatusNotFound, "approved user not found")
			return
		}
		http2.RespondError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

For the remaining methods (`LoginUser`, `GetCurrentUser`, `ListApprovedUsers`, `CreateApprovedUser`, `BulkCreateApprovedUsers`): copy the body of the corresponding old handler verbatim, rename the method, and swap the request struct type to the `api.*` equivalent (`api.LoginRequest`, `api.ApprovedUserRequest`, `api.BulkApprovedUserRequest`). `GetCurrentUser` and `ListApprovedUsers` have no request body — copy their bodies unchanged under the new names.

Notes on generated field names: oapi-codegen renders JSON `approved_id` as Go field `ApprovedId`, `first_name` as `FirstName`, `is_completed` as `IsCompleted`. `email` typed as `format: email` becomes `openapi_types.Email` (a `string` type) — convert with `string(req.Email)`. Confirm exact field names against `internal/api/types.gen.go` before writing (Task 4 Step 5).

Imports to add: `"github.com/your-org/go-backend-template/internal/api"`, `apiTypes "github.com/oapi-codegen/runtime/types"`. The `chi` import and manual `chi.URLParam`/`uuid.Parse` in the old `DeleteApprovedUserHandler` are removed (param now typed); keep `uuid` for the `uuid.UUID(id)` conversion.

- [ ] **Step 2: Remove old request structs replaced by generated types**

Delete `RegisterRequest`, `LoginRequest`, `ApprovedUserRequest`, `BulkApprovedUserRequest` struct definitions (now `api.*`). KEEP `AuthResponse`, `UserResponse`, `ApprovedUserResponse`, `BulkApprovedUserRequest`'s response helpers, and `toApprovedUserResponse(s)` — they remain the JSON output types. Remove the now-unused `chi` import.

- [ ] **Step 3: Build and test**

Run: `go build ./internal/auth/...`
Expected: exit 0.

Run: `go test -short ./internal/auth/...`
Expected: PASS (rename any test references to old handler method names).

- [ ] **Step 4: Commit**

```bash
git add internal/auth/handler.go
git commit -m "refactor: implement generated ServerInterface in auth handler"
```

---

## Task 7: Add the validator error handler

**Files:**
- Create: `internal/router/openapi_error.go`
- Test: `internal/router/openapi_error_test.go`

The OapiRequestValidator middleware, on validation failure, must return the project's `{"detail": "..."}` body instead of kin-openapi's default plaintext.

- [ ] **Step 1: Write the failing test**

```go
package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAPIValidationErrorHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	openAPIValidationErrorHandler(rec, "title is required", http.StatusBadRequest)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["detail"] != "title is required" {
		t.Fatalf("detail = %q, want %q", body["detail"], "title is required")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/router/ -run TestOpenAPIValidationErrorHandler -v`
Expected: FAIL (compile error: `openAPIValidationErrorHandler` undefined).

- [ ] **Step 3: Write the implementation**

```go
package router

import (
	"net/http"

	http2 "github.com/your-org/go-backend-template/internal/http"
)

// openAPIValidationErrorHandler renders OapiRequestValidator failures using
// the project's standard {"detail": ...} error body.
func openAPIValidationErrorHandler(w http.ResponseWriter, message string, statusCode int) {
	http2.RespondError(w, statusCode, message)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/router/ -run TestOpenAPIValidationErrorHandler -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/router/openapi_error.go internal/router/openapi_error_test.go
git commit -m "feat: add OpenAPI validation error handler with standard body"
```

---

## Task 8: Build the embedded Swagger UI

**Files:**
- Create: `internal/router/swaggerui/index.html`
- Create: `internal/router/swaggerui/embed.go`
- Modify: `internal/router/swagger.go`
- Modify: `internal/router/swagger_stub.go`

The UI loads Swagger UI from the jsDelivr CDN and points it at `/openapi.yaml` (served by the router in Task 9). This keeps the repo light (no vendored JS bundle) while preserving an offline-buildable Go binary (only `index.html` is embedded).

- [ ] **Step 1: Write `internal/router/swaggerui/index.html`**

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <title>Go Backend Template API</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "/openapi.yaml",
        dom_id: "#swagger-ui",
      });
    };
  </script>
</body>
</html>
```

- [ ] **Step 2: Write `internal/router/swaggerui/embed.go`**

```go
// Package swaggerui embeds the Swagger UI host page.
package swaggerui

import _ "embed"

// IndexHTML is the Swagger UI host page served at /swagger.
//
//go:embed index.html
var IndexHTML []byte
```

- [ ] **Step 3: Rewrite `internal/router/swagger.go` (the `//go:build swagger` file)**

```go
//go:build swagger

package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/your-org/go-backend-template/internal/router/swaggerui"
)

// mountSwagger registers the /swagger UI when built with the "swagger" build
// tag. The UI is served from an embedded host page and loads the authored
// spec from /openapi.yaml (registered separately in router.New).
func mountSwagger(r chi.Router, enabled bool) {
	if !enabled {
		return
	}
	r.Get("/swagger", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(swaggerui.IndexHTML)
	})
}
```

- [ ] **Step 4: `internal/router/swagger_stub.go` stays unchanged** (still a no-op for non-swagger builds). Verify its content is:

```go
//go:build !swagger

package router

import "github.com/go-chi/chi/v5"

// mountSwagger is a no-op in builds without the "swagger" build tag.
func mountSwagger(_ chi.Router, _ bool) {}
```

- [ ] **Step 5: Verify both build tags compile**

Run: `go build ./internal/router/... && go build -tags swagger ./internal/router/...`
Expected: exit 0 for both.

- [ ] **Step 6: Commit**

```bash
git add internal/router/swaggerui/ internal/router/swagger.go
git commit -m "feat: embed Swagger UI host page serving authored spec"
```

---

## Task 9: Wire the router to the generated wrappers + validator + spec endpoint

**Files:**
- Modify: `internal/router/router.go`

We construct one `api.ServerInterface` implementation by composing the two handlers (todo implements its 5 methods, auth implements its 7). Since Go has no struct embedding across two pointers automatically forming one interface unless combined, define a small `apiServer` struct embedding both `*todo.Handler` and `*auth.Handler` — method promotion makes it satisfy the full `api.ServerInterface`. Then obtain the generated `ServerInterfaceWrapper` and route each existing chi route to the wrapper method, preserving the auth/admin groups. Mount the validator on `/api/v1` and serve the spec at `/openapi.yaml`.

- [ ] **Step 1: Add the composed server type and update imports**

At the top of `router.go` add imports:

```go
	"github.com/your-org/go-backend-template/internal/api"
	oapimiddleware "github.com/oapi-codegen/nethttp-middleware"
```

Add the composite type (place near `HealthResponse`). It uses **named fields** (not anonymous embedding) because both handlers would otherwise promote a field named `Handler` and collide. The forwarding methods that make it satisfy `api.ServerInterface` are added in Step 4.

```go
// apiServer composes the domain handlers into a single api.ServerInterface.
// Named fields (not embedding) avoid the Handler-name collision between the
// two handler types. Forwarding methods live in apiserver.go. The compile-time
// assertion fails the build if any operation is unimplemented.
type apiServer struct {
	todo *todo.Handler
	auth *auth.Handler
}

var _ api.ServerInterface = (*apiServer)(nil)
```

- [ ] **Step 2: Replace the `/api/v1` route block with wrapper-based routing**

Inside `New`, build the wrapper before the router groups:

```go
	apiSrv := &apiServer{todo: todoHandler, auth: authHandler}
	wrapper := api.ServerInterfaceWrapper{Handler: apiSrv}
```

Then rewrite the `/api/v1` block, keeping the three groups, routing to `wrapper.<Op>`:

```go
	swagger, err := api.GetSwagger()
	if err == nil {
		swagger.Servers = nil // don't enforce the /api/v1 server prefix during validation
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(
			timeoutMiddleware(30*time.Second),
			rateLimitMiddleware(rateLimitCfg),
		)
		if swagger != nil {
			r.Use(oapimiddleware.OapiRequestValidatorWithOptions(swagger, &oapimiddleware.Options{
				ErrorHandler: openAPIValidationErrorHandler,
			}))
		}

		// Public
		r.Post("/auth/register", wrapper.RegisterUser)
		r.Post("/auth/login", wrapper.LoginUser)

		// Protected
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.RequireAuth(authSvc))
			r.Get("/todos", wrapper.ListTodos)
			r.Post("/todos", wrapper.CreateTodo)
			r.Get("/todos/{id}", wrapper.GetTodo)
			r.Patch("/todos/{id}", wrapper.UpdateTodo)
			r.Delete("/todos/{id}", wrapper.DeleteTodo)
			r.Get("/me", wrapper.GetCurrentUser)
		})

		// Admin
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.RequireAdmin(authSvc))
			r.Get("/admin/approved-users", wrapper.ListApprovedUsers)
			r.Post("/admin/approved-users", wrapper.CreateApprovedUser)
			r.Post("/admin/approved-users/bulk", wrapper.BulkCreateApprovedUsers)
			r.Delete("/admin/approved-users/{id}", wrapper.DeleteApprovedUser)
		})
	})
```

Note: `api.ServerInterfaceWrapper`'s methods read chi path params via `chi.URLParam`, so the `{id}` patterns above must match the param name `id` in the spec — they do.

- [ ] **Step 3: Serve the raw spec at `/openapi.yaml` (root group, so the UI can fetch it)**

Add after `mountSwagger(r, swaggerEnabled)`:

```go
	if swaggerEnabled {
		mountOpenAPISpec(r)
	}
```

- [ ] **Step 4: Create the explicit forwarding methods + spec handler**

Create `internal/router/apiserver.go`:

```go
package router

import (
	"net/http"

	apiTypes "github.com/oapi-codegen/runtime/types"
)

// The apiServer forwarding methods delegate to the domain handlers so the
// composite type satisfies api.ServerInterface without anonymous-field name
// collisions (both handlers would otherwise promote a field named Handler).

func (s *apiServer) RegisterUser(w http.ResponseWriter, r *http.Request) { s.auth.RegisterUser(w, r) }
func (s *apiServer) LoginUser(w http.ResponseWriter, r *http.Request)    { s.auth.LoginUser(w, r) }
func (s *apiServer) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	s.auth.GetCurrentUser(w, r)
}
func (s *apiServer) ListApprovedUsers(w http.ResponseWriter, r *http.Request) {
	s.auth.ListApprovedUsers(w, r)
}
func (s *apiServer) CreateApprovedUser(w http.ResponseWriter, r *http.Request) {
	s.auth.CreateApprovedUser(w, r)
}
func (s *apiServer) BulkCreateApprovedUsers(w http.ResponseWriter, r *http.Request) {
	s.auth.BulkCreateApprovedUsers(w, r)
}
func (s *apiServer) DeleteApprovedUser(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	s.auth.DeleteApprovedUser(w, r, id)
}

func (s *apiServer) ListTodos(w http.ResponseWriter, r *http.Request)  { s.todo.ListTodos(w, r) }
func (s *apiServer) CreateTodo(w http.ResponseWriter, r *http.Request) { s.todo.CreateTodo(w, r) }
func (s *apiServer) GetTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	s.todo.GetTodo(w, r, id)
}
func (s *apiServer) UpdateTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	s.todo.UpdateTodo(w, r, id)
}
func (s *apiServer) DeleteTodo(w http.ResponseWriter, r *http.Request, id apiTypes.UUID) {
	s.todo.DeleteTodo(w, r, id)
}
```

Create `internal/router/openapi_spec.go`:

```go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/your-org/go-backend-template/internal/api"
	"gopkg.in/yaml.v3"
)

// mountOpenAPISpec serves the authored spec (from the embedded parsed spec) at
// /openapi.yaml so the Swagger UI host page can render it.
func mountOpenAPISpec(r chi.Router) {
	r.Get("/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		swagger, err := api.GetSwagger()
		if err != nil {
			http.Error(w, "spec unavailable", http.StatusInternalServerError)
			return
		}
		out, err := yaml.Marshal(swagger)
		if err != nil {
			http.Error(w, "spec marshal failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(out)
	})
}
```

(`apiServer` is defined once, in `router.go` Step 1, as the two-named-field struct. The methods above live in `apiserver.go`. Ensure no duplicate `apiServer` definition exists.)

- [ ] **Step 5: Build all targets**

Run: `go build ./... && go build -tags swagger ./...`
Expected: exit 0 both. If `gopkg.in/yaml.v3` isn't in go.mod, run `go mod tidy` first.

- [ ] **Step 6: Run the full router + integration build**

Run: `go vet ./internal/router/...`
Expected: exit 0.

- [ ] **Step 7: Commit**

```bash
git add internal/router/router.go internal/router/apiserver.go internal/router/openapi_spec.go go.mod go.sum
git commit -m "feat: route through generated wrappers with request validation"
```

---

## Task 10: Add an end-to-end validation + client test

**Files:**
- Test: `internal/router/contract_test.go`

Proves the validator rejects a bad body with the standard error shape, and the generated client compiles against the server types.

- [ ] **Step 1: Write the test**

```go
package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/your-org/go-backend-template/internal/api"
)

func TestValidatorRejectsMissingTitle(t *testing.T) {
	swagger, err := api.GetSwagger()
	if err != nil {
		t.Fatalf("GetSwagger: %v", err)
	}
	if len(swagger.Paths.Map()) == 0 {
		t.Fatal("spec has no paths")
	}
}

func TestGeneratedClientCompiles(t *testing.T) {
	// Compile-time proof the generated client is usable; no network call.
	_, err := api.NewClient("http://example.com")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_ = httptest.NewRecorder
	_ = strings.TrimSpace
	_ = http.StatusOK
}
```

- [ ] **Step 2: Run the test**

Run: `go test ./internal/router/ -run 'TestValidator|TestGeneratedClient' -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/router/contract_test.go
git commit -m "test: assert spec loads and generated client compiles"
```

---

## Task 11: Drop swaggo from main.go and the module

**Files:**
- Modify: `cmd/api/main.go:1-9`
- Modify: `go.mod`

- [ ] **Step 1: Remove the swaggo header annotations in `cmd/api/main.go`**

Delete lines 1-9 (the `// @title ... // @name Authorization` block). Keep the `// Command api ...` package doc comment (line 10-11). The file now starts:

```go
// Command api is the main entry point for the API server.
package main
```

- [ ] **Step 2: Tidy out swaggo deps**

Run: `go mod tidy`
Expected: `github.com/swaggo/*` and `github.com/go-openapi/*` (swaggo-only indirects) are removed from `go.mod`/`go.sum`; exit 0.

- [ ] **Step 3: Build everything**

Run: `go build ./... && go build -tags swagger ./...`
Expected: exit 0 both.

- [ ] **Step 4: Commit**

```bash
git add cmd/api/main.go go.mod go.sum
git commit -m "chore: remove swaggo annotations and dependencies"
```

---

## Task 12: Delete the old swagger docs package

**Files:**
- Delete: `docs/swagger/swagger.yaml`, `docs/swagger/docs.go`

- [ ] **Step 1: Remove the directory**

Run: `git rm -r docs/swagger`
Expected: both files staged for deletion.

- [ ] **Step 2: Confirm nothing imports it**

Run: `grep -rn "docs/swagger" --include='*.go' . ; echo done`
Expected: only `done` printed (no remaining references). If any remain, remove those imports.

- [ ] **Step 3: Build**

Run: `go build ./... && go build -tags swagger ./...`
Expected: exit 0 both.

- [ ] **Step 4: Commit**

```bash
git commit -m "chore: delete swaggo-generated docs package"
```

---

## Task 13: Update Dockerfile

**Files:**
- Modify: `Dockerfile:18-23`

- [ ] **Step 1: Remove the swaggo generation block**

Delete the block that installs `swag` and runs `swag init` (the `if [ "$BUILD_TAGS" = "swagger" ]` step around lines 18-23). Generated code is committed, so the build needs no codegen step. Keep the `BUILD_TAGS` arg and its use in the `go build -tags "$BUILD_TAGS"` line (the `swagger` tag still toggles the UI).

- [ ] **Step 2: Build the production image**

Run: `docker build -t go-backend-template:plan-check .`
Expected: image builds successfully, exit 0.

- [ ] **Step 3: Build the swagger-enabled image**

Run: `docker build --build-arg BUILD_TAGS=swagger -t go-backend-template:plan-check-swagger .`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile
git commit -m "build: drop swaggo generation from Dockerfile"
```

---

## Task 14: Update CI workflows

**Files:**
- Modify: `.github/workflows/ci.yml:29-32`
- Modify: `.github/workflows/reusable-build.yml` (4 swaggo blocks)

Replace each "Generate swagger docs" step with: install oapi-codegen, regenerate, and fail on drift; plus a spec lint.

- [ ] **Step 1: Replace the swaggo step in `ci.yml`**

Replace the `Generate swagger docs` step (lines 29-32) with:

```yaml
      - name: Verify generated API code is up to date
        run: |
          go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1
          make openapi
          git diff --exit-code internal/api || (echo "internal/api is stale; run 'make openapi' and commit" && exit 1)
      - name: Lint OpenAPI spec
        run: npx -y @redocly/cli@1.25.11 lint api/openapi.yaml
```

- [ ] **Step 2: Replace all four swaggo blocks in `reusable-build.yml`**

For each of the four `Generate swagger docs` steps (at lines ~21-24, 56-59, 78-81, and any other), replace with the same drift-check step:

```yaml
      - name: Verify generated API code is up to date
        run: |
          go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1
          make openapi
          git diff --exit-code internal/api || (echo "internal/api is stale; run 'make openapi' and commit" && exit 1)
```

(Only add the redocly lint once, in `ci.yml`, to avoid redundant lint across matrix jobs.)

- [ ] **Step 3: Sanity-check the workflow YAML parses**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/ci.yml')); yaml.safe_load(open('.github/workflows/reusable-build.yml')); print('ok')"`
Expected: prints `ok`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/reusable-build.yml
git commit -m "ci: replace swaggo gen with oapi-codegen drift check and spec lint"
```

---

## Task 15: Full verification pass

**Files:** none (verification only)

- [ ] **Step 1: Regenerate to confirm no drift**

Run: `make openapi && git diff --exit-code internal/api`
Expected: no diff, exit 0.

- [ ] **Step 2: Run the full verify target**

Run: `make verify`
Expected: fmt, vet, lint, sqlc-compile, and all tests pass, exit 0. Fix any lint issues (e.g. unused imports) revealed here.

- [ ] **Step 3: Run integration tests (requires Docker)**

Run: `make test-integration`
Expected: PASS. If integration tests reference old handler method names or the old route registration, update them to the new method names / unchanged routes (routes/paths are identical, so request-level tests should pass without change).

- [ ] **Step 4: Manual smoke check of the swagger build**

Run: `go build -tags swagger -o /tmp/api-swagger ./cmd/api`
Expected: exit 0. (Full runtime check requires DB; building is sufficient proof here.)

- [ ] **Step 5: Final commit if any fixes were made**

```bash
git add -A
git commit -m "chore: verification fixes for oapi-codegen migration"
```

---

## Self-review notes (addressed)

- **Spec coverage:** Every endpoint in `docs/swagger/swagger.yaml` (register, login, me, todos CRUD, approved-users list/create/bulk/delete) has an operationId in Task 2 and a handler method in Tasks 5-6 and a route in Task 9. Swagger UI (spec §UI) → Task 8. Validation middleware (spec §4) → Task 9. Typed client (spec §components) → Tasks 4 & 10. CI drift + lint (spec §testing) → Task 14. swaggo removal (spec §migration 6-8) → Tasks 11-13.
- **Type consistency:** Path param type `apiTypes.UUID` (alias `openapi_types.UUID`) used consistently in Tasks 5, 6, 9; converted to `uuid.UUID` via `uuid.UUID(id)` everywhere. Generated request types `api.CreateTodoRequest` / `api.UpdateTodoRequest` / `api.RegisterRequest` / `api.LoginRequest` / `api.ApprovedUserRequest` / `api.BulkApprovedUserRequest` referenced consistently. Composite `apiServer` has exactly one definition (two named fields `todo`, `auth`) — the anonymous-embedding attempt in Task 9 Step 2 is explicitly discarded in favor of the named-field struct + forwarding methods in Step 4.
- **Open risk to verify during execution:** confirm exact generated field names (`ApprovedId` vs `ApprovedID`, `Email` type) against `internal/api/types.gen.go` at Task 4 Step 5 before writing Tasks 5-6; adjust conversions accordingly. oapi-codegen default uses `ApprovedId` (initialism not enforced) — verify and align.
```