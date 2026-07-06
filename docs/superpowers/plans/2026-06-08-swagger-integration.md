# Swagger Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Swagger UI to the API, served at `/swagger/`, enabled only when `SWAGGER_ENABLED=true` (default false).

**Architecture:** swaggo/swag generates an OpenAPI spec from Go comment annotations. swaggo/http-swagger serves the Swagger UI from the generated `docs/swagger` package. The router mounts the UI conditionally based on a config flag passed through `RouterConfig`.

**Tech Stack:** `github.com/swaggo/swag` (codegen CLI + runtime), `github.com/swaggo/http-swagger` (UI handler for net/http)

---

### Task 1: Add dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Install swaggo packages**

```bash
cd /path/to/project
go get github.com/swaggo/swag@latest
go get github.com/swaggo/http-swagger@latest
go mod tidy
```

Expected output: `go.mod` and `go.sum` updated with two new direct dependencies.

- [ ] **Step 2: Install swag CLI**

```bash
go install github.com/swaggo/swag/cmd/swag@latest
```

Verify:
```bash
swag --version
```

Expected: prints a version string like `swag version v1.x.x`

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add swaggo dependencies"
```

---

### Task 2: Add ErrorResponse type and update .gitignore + .env.example

**Files:**
- Modify: `internal/http/response.go`
- Modify: `.gitignore`
- Modify: `.env.example`

- [ ] **Step 1: Add ErrorResponse struct to response.go**

The existing `RespondError` uses `{"detail": message}` — match that key exactly.

Open `internal/http/response.go` and add after the `RespondError` function:

```go
// ErrorResponse is the standard error body returned by all endpoints.
// Used as the response type in Swagger @Failure annotations.
type ErrorResponse struct {
	Detail string `json:"detail"`
}
```

Full file after change:

```go
// Package http provides HTTP response helpers.
package http

import (
	"encoding/json"
	"net/http"
)

// RespondJSON writes a JSON response
func RespondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// RespondError writes a JSON error response
func RespondError(w http.ResponseWriter, status int, message string) {
	RespondJSON(w, status, map[string]string{"detail": message})
}

// ErrorResponse is the standard error body returned by all endpoints.
// Used as the response type in Swagger @Failure annotations.
type ErrorResponse struct {
	Detail string `json:"detail"`
}
```

- [ ] **Step 2: Add docs/swagger/ to .gitignore**

Add at the bottom of `.gitignore`:

```
# Swagger generated docs
docs/swagger/
```

- [ ] **Step 3: Add SWAGGER_ENABLED to .env.example**

Add at the bottom of `.env.example`:

```
# Swagger (set to true in development only)
SWAGGER_ENABLED=false
```

- [ ] **Step 4: Run tests to confirm nothing broken**

```bash
go test ./internal/http/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/http/response.go .gitignore .env.example
git commit -m "feat: add ErrorResponse type, gitignore swagger output"
```

---

### Task 3: Add SwaggerConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

Open `internal/config/config_test.go` and add a test that `SWAGGER_ENABLED=true` is parsed correctly and defaults to false:

```go
func TestSwaggerConfig(t *testing.T) {
	t.Run("defaults to disabled", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://x:x@localhost/x")
		t.Setenv("JWT_SECRET_KEY", "secret")
		cfg, err := Load()
		require.NoError(t, err)
		assert.False(t, cfg.Swagger.Enabled)
	})

	t.Run("enabled when SWAGGER_ENABLED=true", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://x:x@localhost/x")
		t.Setenv("JWT_SECRET_KEY", "secret")
		t.Setenv("SWAGGER_ENABLED", "true")
		cfg, err := Load()
		require.NoError(t, err)
		assert.True(t, cfg.Swagger.Enabled)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/... -run TestSwaggerConfig -v
```

Expected: FAIL — `cfg.Swagger` field does not exist yet.

- [ ] **Step 3: Add SwaggerConfig to config.go**

In `internal/config/config.go`, add the struct after `CORSConfig`:

```go
// SwaggerConfig contains swagger UI settings
type SwaggerConfig struct {
	Enabled bool
}
```

Add the field to the `Config` struct (after `CORS CORSConfig`):

```go
Swagger SwaggerConfig
```

Add loading in the `Load()` function (after the CORS block):

```go
Swagger: SwaggerConfig{
    Enabled: p.bool("SWAGGER_ENABLED", false),
},
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/config/... -run TestSwaggerConfig -v
```

Expected: PASS

- [ ] **Step 5: Run full config tests**

```bash
go test ./internal/config/... -v
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat: add SwaggerConfig with SWAGGER_ENABLED env var"
```

---

### Task 4: Add Makefile swagger target

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add swagger target to Makefile**

Add the following to `Makefile`. Insert it after the `install-tools` target. Also add `swagger` to the `.PHONY` line at the top.

Updated `.PHONY` line:
```makefile
.PHONY: help build build-migrate run test test-unit test-integration test-coverage lint fmt vet tidy clean install-tools swagger sqlc-gen sqlc-compile migrate-up migrate-down migrate-reset migrate-version migrate-force docker-build verify ci
```

New target (insert after `install-tools`):
```makefile
swagger: ## Generate Swagger docs from annotations
	@swag init --parseInternal -g cmd/api/main.go -o docs/swagger
```

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "chore: add make swagger target"
```

---

### Task 5: Add top-level Swagger annotations to main.go

**Files:**
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add package-level swagger annotations**

In `cmd/api/main.go`, add the annotations as a doc comment block directly above the `package main` line:

```go
// @title           Go Backend Template API
// @version         1.0
// @description     REST API for go-backend-template
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization

// Command api is the main entry point for the API server.
package main
```

Note: the swagger annotations must appear before the `package` declaration and before any other doc comment. Keep the existing "Command api..." comment as a regular doc comment on the line just above `package main`.

- [ ] **Step 2: Commit**

```bash
git add cmd/api/main.go
git commit -m "feat: add top-level swagger API annotations"
```

---

### Task 6: Add Swagger annotations to auth handlers

**Files:**
- Modify: `internal/auth/handler.go`

- [ ] **Step 1: Annotate RegisterHandler**

Add directly above `func (h *Handler) RegisterHandler(...)`:

```go
// RegisterHandler handles user registration
//
// @Summary      Register a new user
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body RegisterRequest true "Registration details"
// @Success      201 {object} AuthResponse
// @Failure      400 {object} http2.ErrorResponse
// @Failure      404 {object} http2.ErrorResponse
// @Failure      409 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /auth/register [post]
func (h *Handler) RegisterHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 2: Annotate LoginHandler**

Add directly above `func (h *Handler) LoginHandler(...)`:

```go
// LoginHandler handles user login
//
// @Summary      Login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "Login credentials"
// @Success      200 {object} AuthResponse
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /auth/login [post]
func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 3: Annotate GetMeHandler**

```go
// GetMeHandler handles getting current user info
//
// @Summary      Get current user
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} UserResponse
// @Failure      401 {object} http2.ErrorResponse
// @Router       /me [get]
func (h *Handler) GetMeHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 4: Annotate ListApprovedUsersHandler**

```go
// ListApprovedUsersHandler handles GET /admin/approved-users
//
// @Summary      List approved users
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array}  ApprovedUserResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /admin/approved-users [get]
func (h *Handler) ListApprovedUsersHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 5: Annotate CreateApprovedUserHandler**

```go
// CreateApprovedUserHandler handles POST /admin/approved-users
//
// @Summary      Create approved user
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body ApprovedUserRequest true "Approved user details"
// @Success      201 {object} ApprovedUserResponse
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      409 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /admin/approved-users [post]
func (h *Handler) CreateApprovedUserHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 6: Annotate BulkCreateApprovedUsersHandler**

```go
// BulkCreateApprovedUsersHandler handles POST /admin/approved-users/bulk
//
// @Summary      Bulk create approved users
// @Tags         admin
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body BulkApprovedUserRequest true "List of approved users"
// @Success      201 {array}  ApprovedUserResponse
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /admin/approved-users/bulk [post]
func (h *Handler) BulkCreateApprovedUsersHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 7: Annotate DeleteApprovedUserHandler**

```go
// DeleteApprovedUserHandler handles DELETE /admin/approved-users/{id}
//
// @Summary      Delete approved user
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Approved user UUID"
// @Success      204
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      404 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /admin/approved-users/{id} [delete]
func (h *Handler) DeleteApprovedUserHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 8: Verify build**

```bash
go build ./internal/auth/...
```

Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add internal/auth/handler.go
git commit -m "feat: add swagger annotations to auth handlers"
```

---

### Task 7: Add Swagger annotations to todo handlers

**Files:**
- Modify: `internal/todo/handler.go`

- [ ] **Step 1: Annotate ListHandler**

Add directly above `func (h *Handler) ListHandler(...)`:

```go
// ListHandler handles listing todos for the current user
//
// @Summary      List todos
// @Tags         todos
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array}  TodoResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /todos [get]
func (h *Handler) ListHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 2: Annotate CreateHandler**

```go
// CreateHandler handles creating a new todo
//
// @Summary      Create todo
// @Tags         todos
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body CreateTodoRequest true "Todo details"
// @Success      201 {object} TodoResponse
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /todos [post]
func (h *Handler) CreateHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 3: Annotate GetHandler**

```go
// GetHandler handles getting a single todo by ID
//
// @Summary      Get todo
// @Tags         todos
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Todo UUID"
// @Success      200 {object} TodoResponse
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      404 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /todos/{id} [get]
func (h *Handler) GetHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 4: Annotate UpdateHandler**

```go
// UpdateHandler handles updating a todo
//
// @Summary      Update todo
// @Tags         todos
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id   path string          true "Todo UUID"
// @Param        body body UpdateTodoRequest true "Updated todo"
// @Success      200 {object} TodoResponse
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      404 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /todos/{id} [put]
func (h *Handler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 5: Annotate DeleteHandler**

```go
// DeleteHandler handles deleting a todo
//
// @Summary      Delete todo
// @Tags         todos
// @Produce      json
// @Security     BearerAuth
// @Param        id path string true "Todo UUID"
// @Success      204
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Failure      404 {object} http2.ErrorResponse
// @Failure      500 {object} http2.ErrorResponse
// @Router       /todos/{id} [delete]
func (h *Handler) DeleteHandler(w http.ResponseWriter, r *http.Request) {
```

- [ ] **Step 6: Verify build**

```bash
go build ./internal/todo/...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add internal/todo/handler.go
git commit -m "feat: add swagger annotations to todo handlers"
```

---

### Task 8: Wire SwaggerEnabled into RouterConfig and mount routes

**Files:**
- Modify: `internal/router/router.go`
- Modify: `cmd/api/main.go`

- [ ] **Step 1: Add SwaggerEnabled to RouterConfig**

In `internal/router/router.go`, add the field to `RouterConfig`:

```go
// RouterConfig holds dependencies for router setup
type RouterConfig struct {
	Logger          *zap.Logger
	Tracer          trace.Tracer
	AuthSvc         appmiddleware.AuthProvider
	TodoService     todo.TodoService
	AuthHandler     *auth.Handler
	TodoHandler     *todo.Handler
	CORS            config.CORSConfig
	RateLimit       config.RateLimitConfig
	CheckDBHealth   func() error
	SwaggerEnabled  bool
}
```

- [ ] **Step 2: Add swagger import and conditional route mount**

Add the import for http-swagger and the generated docs package. The generated package path will be `github.com/your-org/go-backend-template/docs/swagger`.

Add to the imports in `router.go`:

```go
_ "github.com/your-org/go-backend-template/docs/swagger" // swagger docs (side-effect import)
httpSwagger "github.com/swaggo/http-swagger"
```

In `router.New()`, after the public routes block (`r.Get("/health", ...)`), add:

```go
// Swagger UI — development only
if cfg.SwaggerEnabled {
    r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, r.RequestURI+"/", http.StatusMovedPermanently)
    })
    r.Get("/swagger/*", httpSwagger.Handler(
        httpSwagger.URL("/swagger/doc.json"),
    ))
}
```

- [ ] **Step 3: Pass SwaggerEnabled from main.go**

In `cmd/api/main.go`, update the `RouterConfig` construction:

```go
routerConfig := router.RouterConfig{
    Logger:         logger,
    Tracer:         tracer,
    AuthSvc:        authService,
    TodoService:    todoService,
    AuthHandler:    authHandler,
    TodoHandler:    todoHandler,
    CORS:           cfg.CORS,
    RateLimit:      cfg.RateLimit,
    CheckDBHealth:  func() error { return pool.Ping(context.Background()) },
    SwaggerEnabled: cfg.Swagger.Enabled,
}
```

- [ ] **Step 4: Generate swagger docs**

```bash
make swagger
```

Expected: `docs/swagger/` directory created with `docs.go`, `swagger.json`, `swagger.yaml`.

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 6: Commit**

```bash
git add internal/router/router.go cmd/api/main.go docs/swagger/
git commit -m "feat: mount swagger UI behind SwaggerEnabled config flag"
```

Wait — `docs/swagger/` is in `.gitignore`. Do NOT commit the generated files. They are regenerated locally with `make swagger`. Only commit the router and main changes:

```bash
git add internal/router/router.go cmd/api/main.go
git commit -m "feat: mount swagger UI behind SwaggerEnabled config flag"
```

---

### Task 9: Smoke test the swagger UI

**Files:** none changed

- [ ] **Step 1: Set env and generate docs**

```bash
make swagger
```

- [ ] **Step 2: Start the server with swagger enabled**

```bash
SWAGGER_ENABLED=true DATABASE_URL=postgres://postgres:postgres@localhost:5432/go_backend_template?sslmode=disable JWT_SECRET_KEY=dev-secret go run ./cmd/api
```

- [ ] **Step 3: Open Swagger UI in browser**

Navigate to: `http://localhost:8080/swagger/`

Expected: Swagger UI loads showing "Go Backend Template API", with auth and todo endpoints listed.

- [ ] **Step 4: Verify swagger is disabled by default**

Stop the server. Start without `SWAGGER_ENABLED`:

```bash
DATABASE_URL=postgres://postgres:postgres@localhost:5432/go_backend_template?sslmode=disable JWT_SECRET_KEY=dev-secret go run ./cmd/api
```

Navigate to `http://localhost:8080/swagger/` — expected: 404 Not Found.

- [ ] **Step 5: Run full test suite**

```bash
go test ./... -short
```

Expected: all PASS.

- [ ] **Step 6: Final commit if any cleanup needed**

```bash
git add -p
git commit -m "chore: swagger integration complete"
```

---

## install-tools update

Add `swag` to the `install-tools` Makefile target so new developers get the CLI automatically:

```makefile
install-tools: ## Install required tools
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0
	@go install github.com/swaggo/swag/cmd/swag@latest
```

Commit:
```bash
git add Makefile
git commit -m "chore: add swag CLI to install-tools"
```
