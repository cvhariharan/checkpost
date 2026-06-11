BINARY      := checkpost
COMPOSE_DEV := docker compose -f dev/docker-compose.yml

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

WEB_DIST    := web/dist/index.html
WEB_STAMP   := web/node_modules/.install-stamp
WEB_SRC     := $(shell find web/src -type f) web/package.json web/svelte.config.js web/vite.config.js web/tsconfig.json
GO_SRC      := $(shell find . -type f -name '*.go' -not -path './web/node_modules/*')

.PHONY: build build-go web dev dev-down up down clean

build: $(BINARY)

web: $(WEB_DIST)

build-go:
	CGO_ENABLED=1 go build -ldflags '$(LDFLAGS)' -o $(BINARY) ./cmd/checkpost

$(WEB_STAMP): web/package.json web/package-lock.json
	cd web && npm install
	@touch $@

$(WEB_DIST): $(WEB_SRC) $(WEB_STAMP)
	cd web && npm run build

$(BINARY): $(GO_SRC) go.mod go.sum $(WEB_DIST)
	CGO_ENABLED=1 go build -ldflags '$(LDFLAGS)' -o $@ ./cmd/checkpost

dev:
	$(COMPOSE_DEV) up --build -d

dev-down:
	$(COMPOSE_DEV) down -v

up:
	docker compose up --build

down:
	docker compose down

clean:
	rm -rf $(BINARY) web/dist
