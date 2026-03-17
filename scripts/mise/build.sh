#!/usr/bin/env bash
set -euo pipefail

version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "none")}"
date_utc="${DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
ldflags="-s -w -X main.version=${version} -X main.commit=${commit} -X main.date=${date_utc}"

mkdir -p .cache/go-build bin
export GOCACHE="${PWD}/.cache/go-build"

go build -buildvcs=false -ldflags "${ldflags}" -o bin/expense-bot .
