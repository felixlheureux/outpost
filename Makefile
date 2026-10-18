GO ?= go
GOFMT ?= gofmt
SQLC ?= sqlc
VERSION ?= development

.PHONY: setup dev build release check check-format generate smoke format

setup:
	$(GO) mod download
	npm --prefix web ci

dev:
	GO=$(GO) ./scripts/dev.sh

build:
	npm --prefix web run build
	CGO_ENABLED=0 $(GO) build -buildvcs=false -trimpath -ldflags="-s -w -X github.com/felixlheureux/outpost/internal/cli.Version=$(VERSION)" -o bin/outpost ./cmd/outpost

release:
	@test "$(VERSION)" != development || (echo 'Set VERSION to the release tag' >&2; exit 1)
	npm --prefix web run build
	mkdir -p dist
	@for arch in amd64 arm64; do \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$arch $(GO) build -buildvcs=false -trimpath -ldflags="-s -w -X github.com/felixlheureux/outpost/internal/cli.Version=$(VERSION)" -o dist/outpost-linux-$$arch ./cmd/outpost || exit; \
		(cd dist && sha256sum outpost-linux-$$arch > outpost-linux-$$arch.sha256) || exit; \
	done
	cp scripts/backup.py dist/outpost-backup.py
	(cd dist && sha256sum outpost-backup.py > outpost-backup.py.sha256)

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
