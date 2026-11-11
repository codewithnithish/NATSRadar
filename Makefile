.DEFAULT_GOAL := help
VERSION ?= dev

.PHONY: help nats seed dev-api dev-web web build test vet fmt clean docker-build docker-up docker-down helm-lint helm-package

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

nats: ## Run a local NATS server with JetStream (data in .data/nats)
	nats-server -js -sd .data/nats -m 8222

seed: ## Create demo streams, consumers, KV stores and Object stores on local NATS
	go run ./cmd/natscope-seed

dev-api: ## Run the Go server on :8080 (serves the last built UI)
	go run ./cmd/natscope

dev-web: ## Run the React dev server on :5173 (proxies /api to :8080)
	cd web && npm install && npm run dev

web: ## Build the React app and copy it into the Go embed directory
	cd web && npm ci && npm run build
	find internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
	cp -R web/dist/. internal/webui/dist/

build: web ## Build the single binary at bin/natscope
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o bin/natscope ./cmd/natscope

docker-build: ## Build the Natscope Docker image
	docker build -t natscope -f docker/Dockerfile .

docker-up: ## Start the full NATS + Natscope stack with Docker Compose
	docker compose -f docker/docker-compose.yml up -d

docker-down: ## Stop the Docker Compose stack
	docker compose -f docker/docker-compose.yml down

helm-lint: ## Lint the Natscope Helm Chart
	helm lint k8s/charts/natscope

helm-package: ## Package the Natscope Helm Chart
	helm package k8s/charts/natscope -d k8s/charts/

test: ## Run Go tests (starts an in-process NATS server)
	go test ./...

vet: ## Static checks for Go and TypeScript
	go vet ./...
	cd web && npm run typecheck

fmt: ## Format Go code
	gofmt -w cmd internal

clean: ## Remove build output and local NATS data
	rm -rf bin .data web/dist
	find internal/webui/dist -mindepth 1 ! -name .gitkeep -delete
