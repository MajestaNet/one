.PHONY: test test-race test-ide test-ide-integration cover lint api worker migrate build boundary image-contents ide-artifacts ci

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

api:
	$(GO) run ./cmd/api

worker:
	$(GO) run ./cmd/worker

# Direct go.mod requires (including cmd/one's go-keyring) are fetched by `go run`.
migrate:
	$(GO) run ./cmd/migrate

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
