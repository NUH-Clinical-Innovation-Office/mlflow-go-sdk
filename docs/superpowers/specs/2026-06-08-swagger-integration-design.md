# Swagger Integration Design

**Date:** 2026-06-08
**Tool:** swaggo/swag + swaggo/http-swagger
**Status:** Approved

## Overview

Add Swagger UI to the go-backend-template API. Disabled by default. Enabled only when `SWAGGER_ENABLED=true`. Safe for production deployments where the env var is unset or false.

## 1. Config

Add `SwaggerConfig` struct to `internal/config/config.go`:

```go
type SwaggerConfig struct {
    Enabled bool
}
```

Loaded from `SWAGGER_ENABLED` env var, default `false`. Added to `Config` struct as `Swagger SwaggerConfig`. No changes to `Validate()`.

`.env.example` addition:
```
# Swagger
SWAGGER_ENABLED=false
```

## 2. Dependencies

- `github.com/swaggo/swag` — annotation parser + codegen CLI
- `github.com/swaggo/http-swagger` — serves Swagger UI via `net/http`

Install via `go get`. A `make swagger` Makefile target runs:
```bash
swag init --parseInternal -g cmd/api/main.go -o docs/swagger
```

- `--parseInternal` required — handlers live in `internal/`
- `-g` points to main entry for top-level API annotations
- `-o docs/swagger` keeps generated files out of root

## 3. Router Integration

`RouterConfig` gets a new field `SwaggerEnabled bool`.

In `router.New()`:

```go
if cfg.SwaggerEnabled {
    r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
        http.Redirect(w, r, r.RequestURI+"/", http.StatusMovedPermanently)
    })
    r.Get("/swagger/*", httpSwagger.Handler(
        httpSwagger.URL("/swagger/doc.json"),
    ))
}
```

The redirect `/swagger` → `/swagger/` is required for relative asset paths in the UI.

`cmd/api/main.go` passes `cfg.Swagger.Enabled` into `RouterConfig`.

## 4. Swagger Annotations

Top-level API info in `cmd/api/main.go`:

```go
// @title           Go Backend Template API
// @version         1.0
// @description     REST API for go-backend-template
// @host            localhost:8080
// @BasePath        /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
```

Handler-level annotations on all handlers in `internal/auth/handler.go` and `internal/todo/handler.go`. Protected routes include `@Security BearerAuth`. Example:

```go
// @Summary      Login
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body LoginRequest true "Login credentials"
// @Success      200 {object} AuthResponse
// @Failure      400 {object} http2.ErrorResponse
// @Failure      401 {object} http2.ErrorResponse
// @Router       /auth/login [post]
```

## 5. Error Response Type

Add exported struct to `internal/http/response.go`:

```go
type ErrorResponse struct {
    Error string `json:"error"`
}
```

Required so swaggo can resolve `$ref` in `@Failure` annotations.

## 6. Makefile & gitignore

New Makefile target:
```makefile
.PHONY: swagger
swagger:
	swag init --parseInternal -g cmd/api/main.go -o docs/swagger
```

Add `docs/swagger/` to `.gitignore` — generated files are not committed. Developers run `make swagger` locally before starting the server in development.

## Files Changed

| File | Change |
|---|---|
| `internal/config/config.go` | Add `SwaggerConfig`, load `SWAGGER_ENABLED` |
| `internal/router/router.go` | Add `SwaggerEnabled` to `RouterConfig`, mount routes conditionally |
| `internal/http/response.go` | Add `ErrorResponse` struct |
| `cmd/api/main.go` | Add top-level swagger annotations, pass `SwaggerEnabled` to router |
| `internal/auth/handler.go` | Add per-handler swagger annotations |
| `internal/todo/handler.go` | Add per-handler swagger annotations |
| `go.mod` / `go.sum` | Add swaggo dependencies |
| `Makefile` | Add `swagger` target |
| `.env.example` | Add `SWAGGER_ENABLED=false` |
| `.gitignore` | Add `docs/swagger/` |
