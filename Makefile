.PHONY: dev up down logs migrate-up migrate-down sqlc test lint fmt web api worker

up:
    docker compose up -d postgres minio createbucket mailpit

down:
    docker compose down

logs:
    docker compose logs -f --tail=100

dev:
    $(MAKE) up
    $(MAKE) migrate-up
    pnpm --parallel -r dev

web:
    pnpm --filter web dev

api:
    cd apps/api && go run ./cmd/server

worker:
    cd apps/api && go run ./cmd/worker

migrate-up:
    cd apps/api && goose -dir db/migrations postgres "$${DATABASE_URL}" up

migrate-down:
    cd apps/api && goose -dir db/migrations postgres "$${DATABASE_URL}" down

sqlc:
    cd apps/api && sqlc generate

test:
    cd apps/api && go test ./...
    pnpm -r test

lint:
    cd apps/api && golangci-lint run
    pnpm -r lint

fmt:
    cd apps/api && gofmt -w .
    pnpm -r format