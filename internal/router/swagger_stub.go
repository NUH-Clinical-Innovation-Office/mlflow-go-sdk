//go:build !swagger

package router

import "github.com/go-chi/chi/v5"

// mountSwagger is a no-op in builds without the "swagger" build tag.
func mountSwagger(_ chi.Router, _ bool) {}
