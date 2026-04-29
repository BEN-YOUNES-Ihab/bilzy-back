.PHONY: dev build run tidy migrate-up migrate-down migrate-status migrate-reset test fmt vet

# ---- run ---------------------------------------------------------------------

dev:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

run: build
	./bin/server

# ---- deps --------------------------------------------------------------------

tidy:
	go mod tidy

# ---- db ----------------------------------------------------------------------
# Requires goose CLI: `go install github.com/pressly/goose/v3/cmd/goose@latest`
# Reads DATABASE_URL from .env (override on the command line if needed).

GOOSE_DIR := internal/db/migrations
GOOSE_DRIVER := postgres
GOOSE = goose -dir $(GOOSE_DIR) $(GOOSE_DRIVER) "$$DATABASE_URL"

migrate-up:
	@set -a; . ./.env; set +a; $(GOOSE) up

migrate-down:
	@set -a; . ./.env; set +a; $(GOOSE) down

migrate-status:
	@set -a; . ./.env; set +a; $(GOOSE) status

migrate-reset:
	@set -a; . ./.env; set +a; $(GOOSE) reset

# ---- quality -----------------------------------------------------------------

test:
	go test ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...
