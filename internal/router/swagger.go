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
