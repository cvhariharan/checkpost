# Watcher developer tasks. Run `make` (or `make help`) to list targets.

BINARY      := watcher
COMPOSE_DEV := docker compose -f dev/docker-compose.yml

# Inputs that, when changed, should trigger a rebuild.
WEB_DIST    := web/dist/index.html
WEB_STAMP   := web/node_modules/.install-stamp
WEB_SRC     := $(shell find web/src -type f) web/package.json web/svelte.config.js web/vite.config.js web/tsconfig.json
GO_SRC      := $(shell find . -type f -name '*.go' -not -path './web/node_modules/*')

.PHONY: build build-go web dev dev-down up down clean

build: $(BINARY) ## Build the frontend + watcher binary (needs CGO toolchain; see dev/README.md)

web: $(WEB_DIST) ## Build the frontend bundle (embedded into the binary)

build-go: ## Force-build only the Go binary (assumes web/dist already built)
	CGO_ENABLED=1 go build -o $(BINARY) ./cmd/server

# Install node deps only when the manifest/lockfile change.
$(WEB_STAMP): web/package.json web/package-lock.json
	cd web && npm install
	@touch $@

# Rebuild the frontend bundle only when its sources or deps change.
$(WEB_DIST): $(WEB_SRC) $(WEB_STAMP)
	cd web && npm run build

# Rebuild the binary only when Go sources, modules, or the embedded bundle change.
$(BINARY): $(GO_SRC) go.mod go.sum $(WEB_DIST)
	CGO_ENABLED=1 go build -o $@ ./cmd/server

dev: ## Start the full dev stack (watcher, postgres, dex, mailhog, osquery agents)
	$(COMPOSE_DEV) up --build -d

dev-down: ## Stop the dev stack and remove its volumes
	$(COMPOSE_DEV) down -v

up: ## Quick start: watcher + postgres over HTTPS
	docker compose up --build

down: ## Stop the quick-start stack
	docker compose down

clean: ## Remove the built binary and frontend bundle
	rm -rf $(BINARY) web/dist
