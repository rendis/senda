.PHONY: dev dev-down dev-clean build test test-integration test-e2e test-e2e-run test-e2e-full test-e2e-full-run test-e2e-core test-e2e-chaos test-e2e-up test-e2e-down test-e2e-core-run test-e2e-chaos-run system-validate-manifest system-matrix system-pr system-nightly system-down lint migrate-up migrate-down clean help

COMPOSE     := docker compose -f docker/docker-compose.yml
BINARY      := senda
DATABASE_URL ?= postgres://senda:senda@localhost:5432/senda?sslmode=disable
SENDA_BASE_URL ?= http://localhost:8090
MAILPIT_URL ?= http://localhost:9025
SENDA_E2E_JWT_SECRET ?= e2e-test-jwt-secret-at-least-32-characters-long

E2E_ENV := SENDA_BASE_URL=$(SENDA_BASE_URL) \
           MAILPIT_URL=$(MAILPIT_URL) \
           SENDA_E2E_JWT_SECRET=$(SENDA_E2E_JWT_SECRET)
E2E_DETERMINISTIC_PATTERN := '^(TestCore|TestCRUD|TestE|TestF|TestS)'

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
test-e2e-up: ## E2E harness is managed inside go test via Testcontainers
	@echo "test-e2e-up: no-op (Testcontainers harness is self-managed by ./test/e2e)"

test-e2e-down: ## E2E harness is managed inside go test via Testcontainers
	@echo "test-e2e-down: no-op (Testcontainers harness is self-managed by ./test/e2e)"

test-e2e: ## Run deterministic E2E gate (no chaos)
	$(E2E_ENV) go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/ -run $(E2E_DETERMINISTIC_PATTERN)

test-e2e-run: ## Run deterministic E2E gate (assumes stack already running)
	$(E2E_ENV) SENDA_E2E_EXTERNAL_STACK=1 go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/ -run $(E2E_DETERMINISTIC_PATTERN)

test-e2e-full: ## Run full E2E suite including AWS-sim + chaos tests
	$(E2E_ENV) SENDA_E2E_AWS=1 go test -tags=e2e -v -count=1 -timeout 900s ./test/e2e/

test-e2e-full-run: ## Run full E2E suite including AWS-sim + chaos tests
	$(E2E_ENV) SENDA_E2E_AWS=1 go test -tags=e2e -v -count=1 -timeout 900s ./test/e2e/

test-e2e-core: ## Run deterministic E2E core gate (no chaos)
	$(E2E_ENV) go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/ -run '^TestCore'

test-e2e-core-run: ## Run deterministic E2E core gate (assumes stack already running)
	$(E2E_ENV) go test -tags=e2e -v -count=1 -timeout 600s ./test/e2e/ -run '^TestCore'

test-e2e-chaos-run: ## Run chaos E2E suite (non-blocking)
	$(E2E_ENV) go test -tags=e2e -v -count=1 -timeout 900s ./test/e2e/ -run '^TestC[0-9]'

test-e2e-chaos: ## Run chaos E2E suite (self-managed Testcontainers harness)
	$(E2E_ENV) go test -tags=e2e -v -count=1 -timeout 900s ./test/e2e/ -run '^TestC[0-9]'

## System test orchestration
system-validate-manifest: ## Validate full screen manifest coverage vs app routes
	go run ./cmd/systemtest validate-manifest --manifest test/system/screen-manifest.json --baseline-map test/system/visual-baseline-map.json --app-dir web/src/app

system-matrix: ## Generate system coverage matrix CSV into artifacts
	mkdir -p artifacts/system
	go run ./cmd/systemtest matrix --manifest test/system/screen-manifest.json --format csv --out artifacts/system/coverage-matrix.csv

system-pr: ## Run PR system gate (functional + UI flow; visual opt-in)
	bash test/system/system-runner.sh pr

system-nightly: ## Run nightly full system gate (functional + security/chaos + a11y; visual opt-in)
	bash test/system/system-runner.sh nightly

system-down: ## Force-stop system E2E stack
	bash test/system/subagents/infra-orchestrator.sh down

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
