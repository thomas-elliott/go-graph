# Every dev command for this repo. `just` with no argument lists them.

set shell := ["bash", "-euo", "pipefail", "-c"]

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
