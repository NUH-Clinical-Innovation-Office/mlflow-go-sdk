package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/your-org/go-backend-template/internal/api"
	"gopkg.in/yaml.v3"
)

// mountOpenAPISpec serves the embedded authored spec for Swagger UI.
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
		if _, err := w.Write(out); err != nil {
			return
		}
	})
}
