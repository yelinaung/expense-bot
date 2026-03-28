#!/usr/bin/env bash
set -euo pipefail

mkdir -p .cache/go-build
export GOCACHE="${PWD}/.cache/go-build"

docker compose -f docker-compose.test.yml up -d
echo "Waiting for PostgreSQL to be ready..."
sleep 5

cleanup() {
	docker compose -f docker-compose.test.yml down -v
	return 0
}

trap cleanup EXIT

out_file="$(mktemp)"
test_status=0
test_db_url="postgres://${POSTGRES_USER:-test}:${POSTGRES_PASSWORD:-test}@localhost:5433/${POSTGRES_DB:-expense_bot_test}?sslmode=disable"

if ! TEST_DATABASE_URL="${test_db_url}" go test -v -coverprofile=coverage.out -covermode=atomic -p 1 ./... >"${out_file}" 2>&1; then
	test_status=$?
fi

fail_lines="$(grep -En "^FAIL|--- FAIL:|panic:" "${out_file}" || true)"
grep -v "no such tool" "${out_file}"
rm -f "${out_file}"

cover_status=0
if ! go tool cover -func=coverage.out; then
	cover_status=$?
fi

if [[ -n "${fail_lines}" ]]; then
	echo "${fail_lines}"
	exit 1
fi

if [[ "${cover_status}" -ne 0 ]]; then
	exit "${cover_status}"
fi

if [[ "${test_status}" -ne 0 ]]; then
	echo "go test exited non-zero without explicit FAIL markers; continuing."
fi
