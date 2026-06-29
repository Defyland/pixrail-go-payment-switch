GO ?= go
GOFMT ?= gofmt
GOVULNCHECK_VERSION ?= v1.3.0
REDOCLY_VERSION ?= 2.31.5
REDOCLY ?= npx --yes @redocly/cli@$(REDOCLY_VERSION)
IMAGE ?= pixrail-go-payment-switch:local
PIXRAIL_POSTGRES_TEST_DSN ?= postgres://pixrail:pixrail@localhost:5432/pixrail?sslmode=disable

.PHONY: fmt fmt-check vet build test test-race bench security openapi-lint integration-postgres compose-config docker-build verify ci

fmt:
	$(GOFMT) -w cmd internal

fmt-check:
	test -z "$$($(GOFMT) -l cmd internal)"

vet:
	$(GO) vet ./...

build:
	$(GO) build ./cmd/pixrail-api ./cmd/pixrail-migrate ./cmd/pixrail-worker

test:
	$(GO) test ./...

test-race:
	$(GO) test -race -coverprofile=coverage.out ./...

bench:
	$(GO) test -bench=. -run '^$$' -benchtime=1x ./internal/api

security:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

openapi-lint:
	$(REDOCLY) lint openapi.yaml

integration-postgres:
	PIXRAIL_POSTGRES_TEST_DSN='$(PIXRAIL_POSTGRES_TEST_DSN)' $(GO) test -count=1 -run TestPostgresStoreIntegration -v ./internal/postgres

compose-config:
	docker compose -f compose.yaml config

docker-build:
	docker build -t $(IMAGE) .

verify: fmt-check vet test-race build bench security openapi-lint

ci: verify integration-postgres compose-config docker-build
