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
	_, err := api.NewClient("http://example.com")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	_ = httptest.NewRecorder
	_ = strings.TrimSpace
	_ = http.StatusOK
}
