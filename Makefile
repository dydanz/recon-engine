.PHONY: help build run test clean migrate-up migrate-down docker-up docker-down swag deps gen-mock-data test-api

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	go build -o bin/recon-engine cmd/api/main.go

run: ## Run the application
	go run cmd/api/main.go

test: ## Run tests
	go test -v ./... -cover

clean: ## Clean build artifacts
	rm -rf bin/

migrate-up: ## Run database migrations
	psql $(DB_URL) -f migrations/001_init_schema.sql

migrate-down: ## Rollback database migrations
	psql $(DB_URL) -c "DROP TABLE IF EXISTS reconciliation_results CASCADE; DROP TABLE IF EXISTS reconciliation_jobs CASCADE; DROP TABLE IF EXISTS transactions CASCADE;"

docker-up: ## Start Docker containers
	docker-compose up -d

docker-down: ## Stop Docker containers
	docker-compose down

swag: ## Generate Swagger documentation
	swag init -g cmd/api/main.go -o docs --parseDependency --parseInternal

deps: ## Download dependencies
	go mod download
	go mod tidy

gen-mock-data: ## Generate mock data using Python script (requires Docker)
	./scripts/run_mock_generator.sh

test-api: ## Run API integration tests
	./scripts/test_api.sh

.DEFAULT_GOAL := help
