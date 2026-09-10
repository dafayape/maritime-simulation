# Pintasan operasional dasbor frontend. Selaras dengan Makefile backend.

.PHONY: dev build start lint test docker-build up down logs prod-up prod-down

dev: ## Dev server Vite-like Next.js di :3000
	npm run dev

build: ## Build produksi (type-check + lint + compile)
	npm run build

start: ## Jalankan hasil build produksi secara lokal
	npm run start

lint: ## ESLint seluruh src/
	npm run lint

test: ## Unit test vitest
	npm test

docker-build: ## Build image produksi
	docker compose build

up: ## Jalankan container dasbor (dev lokal, backend di :8080)
	docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f frontend

prod-up: ## Deploy VPS: bind 127.0.0.1 + restart always (di balik nginx)
	docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build

prod-down:
	docker compose -f docker-compose.yml -f docker-compose.prod.yml down
