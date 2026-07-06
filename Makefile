.PHONY: help build build-migrate run test test-unit test-integration test-coverage lint fmt vet tidy update-modules clean install-tools openapi sqlc-gen sqlc-compile migrate-up migrate-down migrate-reset migrate-version migrate-force docker-build verify ci rename-org

# Variables
BINARY_NAME=go-backend-template
MAIN_PATH=./cmd/api
MIGRATE_PATH=./cmd/migrate
DOCKER_IMAGE=go-backend-template:latest

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the API binary
	@go build -o bin/$(BINARY_NAME) $(MAIN_PATH)

build-migrate: ## Build the migration binary
	@go build -o bin/migrate $(MIGRATE_PATH)

run: ## Run the API server
	@go run $(MAIN_PATH)

test: ## Run all tests (unit + integration via testcontainers)
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

test-unit: ## Run unit tests only (excludes integration tests)
	@go test -v -short -race -coverprofile=coverage.out -covermode=atomic ./...

test-integration: ## Run integration tests only (requires Docker - uses testcontainers)
	@go test -v -tags integration -race -timeout 120s ./internal/integration/...

test-coverage: test ## Run tests with coverage report
	@go tool cover -html=coverage.out -o coverage.html

lint: ## Run golangci-lint
	@golangci-lint run ./...

fmt: ## Format code
	@gofmt -w .

vet: ## Run go vet
	@go vet ./...

tidy: ## Tidy go modules
	@go mod tidy

update-modules: ## Update Go modules to their latest compatible versions
	@go get -u ./...
	@go mod tidy

clean: ## Clean build artifacts
	@rm -rf bin/
	@rm -f coverage.out coverage.html

install-tools: ## Install required tools
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.28.0
	@go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1

openapi: ## Generate Go code from api/openapi.yaml (run after editing the spec)
	@oapi-codegen -config api/cfg/types.yaml api/openapi.yaml
	@oapi-codegen -config api/cfg/server.yaml api/openapi.yaml
	@oapi-codegen -config api/cfg/client.yaml api/openapi.yaml

sqlc-gen: ## Generate sqlc code (re-run after any new migration is added)
	@sqlc generate

sqlc-compile: ## Validate sqlc schema/queries without generating
	@sqlc compile

migrate-up: ## Run all pending migrations
	@go run $(MIGRATE_PATH) up

migrate-down: ## Rollback last migration
	@go run $(MIGRATE_PATH) down

migrate-reset: ## Reset database (down then up)
	@go run $(MIGRATE_PATH) reset

migrate-version: ## Show current migration version
	@go run $(MIGRATE_PATH) version

migrate-force: ## Force migration to specific version (requires VERSION=number)
	@go run $(MIGRATE_PATH) force -version $(VERSION)

docker-build: ## Build Docker image
	@docker build -t $(DOCKER_IMAGE) .

verify: fmt vet lint sqlc-compile test ## Run all verification steps
	@echo "All verification steps completed successfully!"

ci: verify ## Run CI pipeline
	@echo "CI pipeline completed successfully!"

rename-org: ## Rename the module path (requires NEW_ORG=github.com/your-org)
	@test -n "$(NEW_ORG)" || (echo "NEW_ORG is required, e.g. make rename-org NEW_ORG=github.com/acme"; exit 1)
	@grep -rl --include='*.go' --include='*.mod' --include='*.sum' 'github.com/your-org/go-backend-template' . | xargs sed -i '' "s|github.com/your-org/go-backend-template|$(NEW_ORG)/go-backend-template|g"
	@go mod tidy
	@echo "Renamed module path to $(NEW_ORG)/go-backend-template"
