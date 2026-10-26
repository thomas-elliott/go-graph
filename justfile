# Every dev command for this repo. `just` with no argument lists them.

set shell := ["bash", "-euo", "pipefail", "-c"]

set dotenv-load

export DATABASE_URL := env("DATABASE_URL", "postgres://graph:graph@localhost:5432/graph?sslmode=disable")
export TEST_DATABASE_URL := env("TEST_DATABASE_URL", "postgres://graph:graph@localhost:5432/graph_test?sslmode=disable")

_default:
    @just --list

dev:
    go run ./cmd/example

# Runs dev, but using a full dataset
dev-full:
    go run ./cmd/example -data ./local/data

# Loads the sample data into postgres
load:
    go run ./cmd/load

# Loads the full dataset into postgres
load-full:
    go run ./cmd/load -data ./local/data

embed:
    go run ./cmd/embed

build:
    go build ./...

test:
    go test ./...

format:
    go fmt ./...

db-up:
    docker compose up -d --wait
    docker compose exec db psql -U graph -tAc "SELECT 1 FROM pg_database WHERE datname = 'graph_test'" | grep -q 1 \
        || docker compose exec db createdb -U graph graph_test

db-down:
    docker compose down

db-psql:
    docker compose exec db psql -U graph graph

eval method="text" *args:
    go run ./cmd/eval -method {{method}} {{args}}
