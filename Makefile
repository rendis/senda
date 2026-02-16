.PHONY: dev dev-down dev-clean build test test-integration lint migrate-up migrate-down clean help

COMPOSE := docker compose -f docker/docker-compose.yml
BINARY  := senda
DATABASE_URL ?= postgres://senda:senda@localhost:5432/senda?sslmode=disable

## Development
dev: ## Start full stack (senda + postgres)
	$(COMPOSE) up --build

dev-down: ## Stop the stack
	$(COMPOSE) down

dev-clean: ## Stop the stack and remove volumes
	$(COMPOSE) down -v

## Build
build: ## Build the binary
	go build -o bin/$(BINARY) ./cmd/senda

## Test
test: ## Run unit tests
	go test ./... -v -count=1 -race

test-integration: ## Run integration tests (requires running postgres)
	go test ./... -v -count=1 -race -tags=integration

## Migrations
migrate-up: ## Run all migrations up
	migrate -database "$(DATABASE_URL)" -path migrations up

migrate-down: ## Roll back last migration
	migrate -database "$(DATABASE_URL)" -path migrations down 1

## Lint
lint: ## Run linter
	golangci-lint run ./...

## Cleanup
clean: ## Remove build artifacts
	rm -rf bin/ tmp/

## Help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
