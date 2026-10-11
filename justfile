# Every dev command for this repo. `just` with no argument lists them.

set shell := ["bash", "-euo", "pipefail", "-c"]

export DATABASE_URL := env("DATABASE_URL", "postgres://graph:graph@localhost:5432/graph?sslmode=disable")

_default:
    @just --list

dev:
    go run ./cmd/example

build:
    go build ./...

test:
    go test ./...

format:
    go fmt ./...

db-up:
    docker compose up -d --wait

db-down:
    docker compose down

db-psql:
    docker compose exec db psql -U graph graph
