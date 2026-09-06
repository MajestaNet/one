.PHONY: test test-race test-ide test-ide-integration cover lint postgres api worker migrate dev build boundary image-contents ide-artifacts ci

COMPOSE_FILE := deploy/docker-compose.yml

GO ?= go
export PATH := /usr/local/go/bin:$(HOME)/go/bin:$(PATH)
CONTROL_IDE := tools/control-ide
# npm ci under tools/control-ide can drop Go files in node_modules (e.g. flatted/golang).
GO_PACKAGES := $(shell $(GO) list ./... | grep -v /node_modules/)

test:
	$(GO) test -p 1 $(GO_PACKAGES)

test-race:
	$(GO) test -p 1 -race -coverprofile=coverage.out $(GO_PACKAGES)

test-ide:
	npm --prefix $(CONTROL_IDE) test

test-ide-integration:
	npm --prefix $(CONTROL_IDE) run test:integration

cover:
	$(GO) test -p 1 -coverprofile=coverage.out $(GO_PACKAGES)
	$(GO) tool cover -func=coverage.out | tail -20
	$(GO) run ./scripts/check-go-coverage coverage.out 35

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "install golangci-lint: https://golangci-lint.run/"; exit 1; }
	golangci-lint run ./...

boundary:
	bash ./scripts/assert-product-boundary.sh

image-contents:
	@test -n "$(IMAGE)" || (echo "usage: make image-contents IMAGE=one-api:tag"; exit 1)
	bash ./scripts/assert-image-contents.sh "$(IMAGE)"

ide-artifacts:
	bash ./scripts/assert-ide-artifacts.sh

# Local Postgres for DATABASE_URL=postgres://one:one@localhost:5432/one
# (requires Docker Desktop / docker compose). make api does not start it.
postgres:
	@command -v docker >/dev/null 2>&1 || { echo >&2 "Docker is required for local Postgres. Start Docker Desktop, or set DATABASE_URL to an existing Postgres 16+ instance."; exit 1; }
	docker compose -f $(COMPOSE_FILE) up -d postgres
	@echo "waiting for postgres on localhost:5432..."
	@i=0; \
	until docker compose -f $(COMPOSE_FILE) exec -T postgres pg_isready -U one -d one >/dev/null 2>&1; do \
		i=$$((i+1)); \
		if [ $$i -ge 30 ]; then echo >&2 "postgres did not become ready on localhost:5432"; exit 1; fi; \
		sleep 1; \
	done
	@echo "postgres is ready (postgres://one:one@localhost:5432/one)"

api:
	$(GO) run ./cmd/api

worker:
	$(GO) run ./cmd/worker

# Direct go.mod requires (including cmd/one's go-keyring) are fetched by `go run`.
migrate:
	$(GO) run ./cmd/migrate

# One-shot local API: Compose Postgres, then go run (kernel migrate + seed on boot).
dev: postgres
	$(GO) run ./cmd/api

build:
	CGO_ENABLED=0 $(GO) build -o bin/one-api ./cmd/api
	CGO_ENABLED=0 $(GO) build -o bin/one-worker ./cmd/worker
	CGO_ENABLED=0 $(GO) build -o bin/one-migrate ./cmd/migrate
	CGO_ENABLED=0 $(GO) build -o bin/one ./cmd/one

# One test pass: race + coverage.out, then report + build (do not re-run cover).
ci: boundary lint test-race
	$(GO) tool cover -func=coverage.out | tail -20
	$(GO) run ./scripts/check-go-coverage coverage.out 35
	$(MAKE) build
