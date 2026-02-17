.PHONY: dev dev-down dev-clean build test test-integration test-e2e test-e2e-up test-e2e-down test-e2e-run lint migrate-up migrate-down clean help

COMPOSE     := docker compose -f docker/docker-compose.yml
COMPOSE_E2E := docker compose -f docker/docker-compose.e2e.yml
BINARY      := senda
DATABASE_URL ?= postgres://senda:senda@localhost:5432/senda?sslmode=disable

E2E_ENV := SENDA_BASE_URL=http://localhost:8090 \
           MAILPIT_URL=http://localhost:9025 \
           SENDA_E2E_JWT_SECRET=e2e-test-jwt-secret-at-least-32-characters-long

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

## E2E Tests
test-e2e-up: ## Start E2E stack (postgres + mailpit + senda)
	$(COMPOSE_E2E) up -d --build --wait

test-e2e-down: ## Stop E2E stack and remove volumes
	$(COMPOSE_E2E) down -v

test-e2e: test-e2e-up ## Run E2E tests (starts stack, runs tests, stops stack)
	$(E2E_ENV) go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/ || ($(MAKE) test-e2e-down && exit 1)
	$(MAKE) test-e2e-down

test-e2e-run: ## Run E2E tests (assumes stack already running)
	$(E2E_ENV) go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/

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
