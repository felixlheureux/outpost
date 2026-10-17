GO ?= go
GOFMT ?= gofmt
SQLC ?= sqlc
VERSION ?= development

.PHONY: setup dev build check check-format generate smoke format

setup:
	$(GO) mod download
	npm --prefix web ci

dev:
	GO=$(GO) ./scripts/dev.sh

build:
	npm --prefix web run build
	CGO_ENABLED=0 $(GO) build -buildvcs=false -trimpath -ldflags="-s -w -X github.com/felixlheureux/outpost/internal/cli.Version=$(VERSION)" -o bin/outpost ./cmd/outpost

generate:
	@test "$$($(SQLC) version)" = "v1.31.1" || (echo 'Use sqlc v1.31.1 (see docs/development.md)' >&2; exit 1)
	$(SQLC) generate

format:
	$(GOFMT) -w cmd internal migrations web/*.go
	npm --prefix web run format

check-format:
	@format_files="$$($(GOFMT) -l cmd internal migrations web/*.go)" && test -z "$$format_files" || (echo 'Go formatting check failed; verify gofmt is installed and run make format' >&2; exit 1)

check: check-format build
	$(GO) vet -tags dev ./...
	$(GO) test -tags dev ./...
	npm --prefix web run check

smoke:
	npm --prefix web run smoke
