.PHONY: dev dev-down dev-clean build test vet test-integration test-e2e test-e2e-run test-e2e-full test-e2e-full-run test-e2e-core test-e2e-chaos test-e2e-up test-e2e-down test-e2e-core-run test-e2e-chaos-run test-e2e-ses system-validate-manifest system-matrix system-pr system-nightly system-down lint migrate-up migrate-down swagger swagger-check ci-backend-pr ci-backend-main ci-frontend ci-pr ci-main install-githooks clean help

COMPOSE     := docker compose -f docker/docker-compose.yml
BINARY      := senda
DATABASE_URL ?= postgres://senda:senda@localhost:5432/senda?sslmode=disable
SENDA_BASE_URL ?= http://localhost:8090
MAILPIT_URL ?= http://localhost:9025
SENDA_E2E_JWT_SECRET ?= e2e-test-jwt-secret-at-least-32-characters-long
SWAG_VERSION ?= v1.16.6
SWAGGER_DOCS_DIR := cmd/senda/docs
SWAGGER_V2 := $(SWAGGER_DOCS_DIR)/swagger.yaml
OPENAPI_V3 := $(SWAGGER_DOCS_DIR)/openapi.yaml
GO_PACKAGES := $(shell bash scripts/go-packages.sh)

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
	go test $(GO_PACKAGES) -v -count=1 -race

vet: ## Run go vet on repo packages only
	go vet $(GO_PACKAGES)

test-integration: ## Run integration tests (requires running postgres)
	go test $(GO_PACKAGES) -v -count=1 -race -tags=integration

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

test-e2e-ses: ## Run SES lifecycle E2E suite (aws-sim bridge + MiniStack + signed SNS replay)
	$(E2E_ENV) go test -tags=e2e -v -count=1 -timeout 900s ./test/e2e/ -run 'TestSESLifecycle0[1-4]_|TestSNSReplay01_'

## System test orchestration
system-validate-manifest: ## Validate full screen manifest coverage vs app routes
	go run ./cmd/systemtest validate-manifest --manifest test/system/screen-manifest.json --baseline-map test/system/visual-baseline-map.json --app-dir web/src/app

system-matrix: ## Generate system coverage matrix CSV into artifacts
	mkdir -p artifacts/system
	go run ./cmd/systemtest matrix --manifest test/system/screen-manifest.json --format csv --out artifacts/system/coverage-matrix.csv

system-pr: ## Run PR system gate light (infra + API contract smoke; UI flow opt-in)
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
	golangci-lint run $(GO_PACKAGES)

## OpenAPI / MCP
swagger: ## Generate swag-based Swagger 2 + OpenAPI 3 docs and validate route coverage
	go run ./cmd/openapi generate-docs --out cmd/senda/openapi_generated.go
	go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION) init -g cmd/senda/main.go -o $(SWAGGER_DOCS_DIR) --parseDependency --parseInternal
	go run ./cmd/openapi convert --swagger $(SWAGGER_V2) --out $(OPENAPI_V3)
	rm -f $(SWAGGER_DOCS_DIR)/swagger.json
	go run ./cmd/openapi validate --spec $(OPENAPI_V3)

swagger-check: ## Regenerate OpenAPI docs and fail if generated artifacts were not committed
	$(MAKE) swagger
	git diff --exit-code -- cmd/senda/openapi_generated.go $(SWAGGER_V2) $(OPENAPI_V3)

## GitHub-aligned validation gates
ci-backend-pr: ## Run the fast backend validation used by GitHub/local push gates (no Docker)
	bash scripts/run-github-gates.sh backend-pr

ci-backend-main: ## Run the fast backend validation used by pushes to main (no Docker)
	bash scripts/run-github-gates.sh backend-main

ci-frontend: ## Run the same frontend validation used by GitHub
	bash scripts/run-github-gates.sh frontend

ci-pr: ## Run the same validation expected before opening/updating a PR
	bash scripts/run-github-gates.sh pr

ci-main: ## Run the same fast validation expected before pushing main/systemic changes
	bash scripts/run-github-gates.sh main

install-githooks: ## Enforce the repo pre-push validation hook locally
	git config core.hooksPath .githooks

## Cleanup
clean: ## Remove build artifacts
	rm -rf bin/ tmp/

## Help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
