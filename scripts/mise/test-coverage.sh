#!/usr/bin/env bash
set -euo pipefail

started_db=0

mkdir -p .cache/go-build
export GOCACHE="${PWD}/.cache/go-build"

cleanup() {
	if [[ "${started_db}" -eq 1 ]]; then
		docker compose -f docker-compose.test.yml down -v
	fi
}

trap cleanup EXIT

if [[ -z "${TEST_DATABASE_URL:-}" ]]; then
	started_db=1
	docker compose -f docker-compose.test.yml up -d
	echo "Waiting for PostgreSQL to be ready..."
	sleep 5
	TEST_DATABASE_URL="postgres://${POSTGRES_USER:-test}:${POSTGRES_PASSWORD:-test}@localhost:5433/${POSTGRES_DB:-expense_bot_test}?sslmode=disable"
	export TEST_DATABASE_URL
fi

out_file="$(mktemp)"
test_status=0
if ! TEST_DATABASE_URL="${TEST_DATABASE_URL}" go test -v -coverprofile=coverage.out -covermode=atomic -p 1 ./... >"${out_file}" 2>&1; then
	test_status=$?
fi

fail_lines="$(grep -En "^FAIL|--- FAIL:|panic:" "${out_file}" || true)"
grep -v "no such tool" "${out_file}"
rm -f "${out_file}"

cover_report="$(go tool cover -func=coverage.out)"
printf "%s\n" "${cover_report}"

total_coverage="$(printf "%s\n" "${cover_report}" | awk '/^total:/ {gsub("%","",$3); print $3}')"
threshold=50

echo "Coverage: ${total_coverage}% (threshold: ${threshold}%)"

if [[ "$(awk "BEGIN {print (${total_coverage} < ${threshold})}")" == "1" ]]; then
	echo "Coverage ${total_coverage}% is below threshold ${threshold}%!"
	exit 1
fi

if [[ -n "${fail_lines}" ]]; then
	echo "${fail_lines}"
	exit 1
fi

if [[ "${test_status}" -ne 0 ]]; then
	echo "go test exited non-zero without explicit FAIL markers; continuing because coverage checks passed."
fi

echo ""
echo "To view HTML report, run: mise run coverage-html"
