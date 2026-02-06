.PHONY: build test test-coverage test-integration lint fmt clean test-db-up test-db-down

build:
	go build -o bin/expense-bot .

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
