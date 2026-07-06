.PHONY: help test test-coverage lint fmt vet tidy clean example

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

test: ## Run all tests
	@go test -race ./...

test-coverage: ## Run tests with coverage report
	@go test -race -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html

lint: ## Run golangci-lint
	@golangci-lint run ./...

fmt: ## Format code
	@gofmt -w .

vet: ## Run go vet
	@go vet ./...

tidy: ## Tidy go modules
	@go mod tidy

clean: ## Clean build artifacts
	@rm -f coverage.out coverage.html

example: ## Run the live MLflow smoke example (needs MLFLOW_TRACKING_URI)
	@go run ./example
