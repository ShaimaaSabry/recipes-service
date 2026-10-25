SWAG=github.com/swaggo/swag/cmd/swag@v1.16.6
GOLANGCI_LINT=github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
TEST_DATABASE_URL ?= postgres://postgres:postgres@localhost:4444/recipes?sslmode=disable

.PHONY: up down run test test-integration lint swag

# Start Postgres in Docker and wait until it accepts connections.
up:
	docker compose up -d --wait

down:
	docker compose down

# Run the server with .env.local (copy it from .env.example first).
run: up
	go run ./cmd

# Unit tests only; the database tests skip themselves.
test:
	go test ./...

# Every test, including the ones against Postgres in Docker.
test-integration: up
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -count=1 ./...

lint:
	go run $(GOLANGCI_LINT) run

swag:
	go run $(SWAG) init -g cmd/main.go -o ./docs --parseInternal
