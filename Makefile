.PHONY: build test test-coverage test-integration lint fmt clean test-db-up test-db-down release

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  = -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/expense-bot .

test:
	go test -v ./...

test-coverage:
	@go test -v -coverprofile=coverage.out -covermode=atomic -p 1 ./... 2>&1 | grep -v "no such tool"
	@go tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML report, run: make coverage-html"

test-race:
	CGO_ENABLED=1 go test -v -race ./...

test-db-up:
	docker compose -f docker-compose.test.yml up -d
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5

test-db-down:
	docker compose -f docker-compose.test.yml down -v

test-integration: test-db-up
	@TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-test}:$${POSTGRES_PASSWORD:-test}@localhost:5433/$${POSTGRES_DB:-expense_bot_test}?sslmode=disable" \
		go test -v -coverprofile=coverage.out -covermode=atomic -p 1 ./... 2>&1 | grep -v "no such tool" || true
	@go tool cover -func=coverage.out
	@$(MAKE) test-db-down

coverage-html: test-coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	golangci-lint run ./...

fmt:
	gofumpt -w .

clean:
	rm -f bin/expense-bot coverage.out coverage.html
	rm -rf dist/

release:
	@if [ -z "$(TAG)" ]; then echo "Usage: make release TAG=v0.3.0"; exit 1; fi
	@echo "Building release $(TAG)..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/expense-bot-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/expense-bot-linux-arm64 .
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/expense-bot-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/expense-bot-darwin-arm64 .
	@cd dist && sha256sum expense-bot-* > checksums.txt
	@echo "Release binaries in dist/"
