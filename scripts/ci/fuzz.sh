#!/bin/sh
# Run every native Go fuzz target for a fixed time budget each.
#
# Fuzzing is coverage-guided: inputs that reach new branches are saved to the
# corpus cache under $GOCACHE/fuzz, so every run resumes from what previous
# runs discovered. To persist that corpus across CI pipelines, the corpus is
# copied to/from CORPUS_DIR, which the CI job caches.
#
# A crashing input makes the target's `go test` run fail and is written to the
# package's testdata/fuzz/<Target>/ directory in the source tree; the CI job
# uploads it as an artifact so the failure is reproducible locally.
#
# Runs inside the alpine/busybox CI image (POSIX sh, not bash).
#   FUZZTIME    time budget per fuzz target (default 20s; go test -fuzztime)
#   CORPUS_DIR  where the persistent corpus cache lives (default .go-fuzz-corpus)
set -u

FUZZTIME="${FUZZTIME:-20s}"
CORPUS_DIR="${CORPUS_DIR:-.go-fuzz-corpus}"

GO_FUZZ_CACHE="$(go env GOCACHE)/fuzz"
mkdir -p "${GO_FUZZ_CACHE}" "${CORPUS_DIR}"
cp -r "${CORPUS_DIR}"/. "${GO_FUZZ_CACHE}"/ 2>/dev/null || true

# Find packages that define fuzz targets without compiling everything.
pkg_dirs=$(find . -path ./.go -prune -o -name '*_test.go' -print \
  | while read -r f; do
      grep -q '^func Fuzz' "$f" && dirname "$f"
    done | sort -u)

if [ -z "${pkg_dirs}" ]; then
  echo "No fuzz targets found."
  exit 0
fi

failed=0
for pkg in ${pkg_dirs}; do
  targets=$(go test "${pkg}" -list '^Fuzz' | grep '^Fuzz' || true)
  for target in ${targets}; do
    echo ">>> ${pkg} ${target} (${FUZZTIME})"
    if ! go test "${pkg}" -run '^$' -fuzz "^${target}$" -fuzztime "${FUZZTIME}"; then
      echo "FAIL: ${pkg} ${target}"
      failed=1
    fi
  done
done

# Persist the corpus even when a target failed, so the crash reproduces fast.
cp -r "${GO_FUZZ_CACHE}"/. "${CORPUS_DIR}"/ 2>/dev/null || true

exit ${failed}
