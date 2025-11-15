.PHONY: help build run test docker-up docker-down migrate-up migrate-down lint clean

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the application
	go build -o bin/pr-service ./cmd/pr-service

run: ## Run the application
	go run ./cmd/pr-service/main.go

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

docker-up: ## Start all services with docker-compose
	docker compose up --build -d

docker-down: ## Stop all services
	docker compose down -v

docker-logs: ## View logs from containers
	docker compose logs -f

migrate-up: ## Run database migrations up (local)
	goose -dir ./migrations postgres "postgresql://postgres:postgres@localhost:5433/pr_service?sslmode=disable" up

migrate-down: ## Run database migrations down (local)
	goose -dir ./migrations postgres "postgresql://postgres:postgres@localhost:5433/pr_service?sslmode=disable" down

lint: ## Run linter
	golangci-lint run

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out
