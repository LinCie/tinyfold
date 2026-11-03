.PHONY: dev start build migrate-up migrate-down migrate-status migrate-version migrate-reset migration

-include .env

DATABASE_URL ?= ./data/app.db
MIGRATIONS_DIR ?= migrations

# Pass everything from .env (JWT_SECRET, PORT, ...) on to the commands below.
.EXPORT_ALL_VARIABLES:
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.27.3

dev:
	@command -v air >/dev/null || { echo "air not found"; exit 1; }
	air

start:
	go run ./cmd/api

build:
	go build -o ./bin/api ./cmd/api

migrate-up:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate up

migrate-down:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate down

migrate-status:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate status

migrate-version:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate version

migrate-reset:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate reset

migration:
	@test -n "$(name)" || (echo "usage: make migration name=create_users" && exit 1)
	$(GOOSE) -dir "$(MIGRATIONS_DIR)" postgres "$(DATABASE_URL)" create "$(name)" sql
