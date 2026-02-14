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
	@set -e; \
	cleanup=0; \
	if [ -z "$$TEST_DATABASE_URL" ]; then \
		cleanup=1; \
		$(MAKE) test-db-up; \
		TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-test}:$${POSTGRES_PASSWORD:-test}@localhost:5433/$${POSTGRES_DB:-expense_bot_test}?sslmode=disable"; \
	fi; \
	out_file="$$(mktemp)"; \
	test_status=0; \
	TEST_DATABASE_URL="$$TEST_DATABASE_URL" go test -v -coverprofile=coverage.out -covermode=atomic -p 1 ./... > "$$out_file" 2>&1 || test_status=$$?; \
	fail_lines="$$(rg -n "^FAIL|--- FAIL:|panic:" "$$out_file" || true)"; \
	grep -v "no such tool" "$$out_file"; \
	rm -f "$$out_file"; \
	go tool cover -func=coverage.out; \
	total_coverage=$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%","",$$3); print $$3}'); \
	threshold=50; \
	echo "Coverage: $${total_coverage}% (threshold: $${threshold}%)"; \
	if [ "$$(awk "BEGIN {print ($$total_coverage < $$threshold)}")" = "1" ]; then \
		echo "Coverage $${total_coverage}% is below threshold $${threshold}%!"; \
		if [ $$cleanup -eq 1 ]; then $(MAKE) test-db-down; fi; \
		exit 1; \
	fi; \
	if [ -n "$$fail_lines" ]; then \
		echo "$$fail_lines"; \
		if [ $$cleanup -eq 1 ]; then $(MAKE) test-db-down; fi; \
		exit 1; \
	fi; \
	if [ $$cleanup -eq 1 ]; then $(MAKE) test-db-down; fi; \
	if [ $$test_status -ne 0 ]; then \
		echo "go test exited non-zero without explicit FAIL markers; continuing because coverage checks passed."; \
	fi
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
	@out_file="$$(mktemp)"; \
	test_status=0; \
	TEST_DATABASE_URL="postgres://$${POSTGRES_USER:-test}:$${POSTGRES_PASSWORD:-test}@localhost:5433/$${POSTGRES_DB:-expense_bot_test}?sslmode=disable" \
		go test -v -coverprofile=coverage.out -covermode=atomic -p 1 ./... > "$$out_file" 2>&1 || test_status=$$?; \
	fail_lines="$$(rg -n "^FAIL|--- FAIL:|panic:" "$$out_file" || true)"; \
	grep -v "no such tool" "$$out_file"; \
	rm -f "$$out_file"; \
	cover_status=0; \
	go tool cover -func=coverage.out || cover_status=$$?; \
	$(MAKE) test-db-down; \
	if [ -n "$$fail_lines" ]; then \
		echo "$$fail_lines"; \
		exit 1; \
	fi; \
	if [ $$cover_status -ne 0 ]; then exit $$cover_status; fi; \
	if [ $$test_status -ne 0 ]; then \
		echo "go test exited non-zero without explicit FAIL markers; continuing."; \
	fi; \
	exit 0

coverage-html: test-coverage
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	go vet ./...
	golangci-lint run ./...

fmt:
	gofumpt -w .

clean:
	rm -f bin/expense-bot coverage.out coverage.html
	rm -rf dist/

release:
	goreleaser release --snapshot --clean
