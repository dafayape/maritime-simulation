.DEFAULT_GOAL := help

## ---------- local development ----------

.PHONY: dev
dev: ## Run the server locally (needs Postgres + Redis, e.g. `make up-deps`)
	go run ./cmd/server

.PHONY: build
build: ## Compile server + nodesim into ./bin
	CGO_ENABLED=0 go build -trimpath -o bin/server ./cmd/server
	CGO_ENABLED=0 go build -trimpath -o bin/nodesim ./cmd/nodesim

.PHONY: test
test: ## Run unit tests with race detector
	go test -race ./...

.PHONY: vet
vet: ## go vet + gofmt check
	go vet ./...
	@fmt_out=$$(gofmt -l .); if [ -n "$$fmt_out" ]; then echo "needs gofmt:"; echo "$$fmt_out"; exit 1; fi

.PHONY: tidy
tidy: ## Sync go.mod/go.sum
	go mod tidy

## ---------- docker ----------

.PHONY: up
up: ## Dev stack (base + override: pprof, db ports on localhost)
	docker compose up -d --build

.PHONY: up-deps
up-deps: ## Only Postgres + Redis (for `make dev` against local Go)
	docker compose up -d postgres redis

.PHONY: down
down: ## Stop the stack (keeps the Postgres volume)
	docker compose down

.PHONY: logs
logs: ## Follow backend logs
	docker compose logs -f backend

.PHONY: ps
ps: ## Stack status
	docker compose ps

## ---------- production (VPS) ----------

.PHONY: prod-up
prod-up: ## Production stack: localhost-bound API behind a reverse proxy
	docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build

.PHONY: prod-down
prod-down:
	docker compose -f docker-compose.yml -f docker-compose.prod.yml down

## ---------- verification ----------

.PHONY: smoke
smoke: ## 50-ship fleet against localhost:8080 (multi-hop via low TX power)
	go run ./cmd/nodesim -api http://localhost:8080 -nodes 50 -tx 10 -spread-km 8 -duration 60s

.PHONY: goroutines
goroutines: ## Goroutine count via pprof (dev stack only)
	@wget -qO- "http://127.0.0.1:6060/debug/pprof/goroutine?debug=0" | head -c 200; echo

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
