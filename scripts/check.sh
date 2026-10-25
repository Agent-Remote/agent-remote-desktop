#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
for script in scripts/*.sh; do
  bash -n "$script"
done
# Keep test sources in the repository-root tests/ tree. Go's package-wide
# `go test ./...` would still discover a misplaced _test.go file, so enforce
# the project layout explicitly before running the suite.
if command -v rg >/dev/null 2>&1; then
  misplaced_tests="$(rg --files -g '*_test.go' -g '!tests/**' || true)"
else
  # GitHub's Windows image does not guarantee ripgrep. Keep this repository
  # layout check independent of optional developer tooling.
  misplaced_tests="$(find . -type f -name '*_test.go' ! -path './tests/*' -print | sed 's#^./##' || true)"
fi
if [[ -n "$misplaced_tests" ]]; then
  echo "Test files must live under repository-root tests/:" >&2
  printf '%s\n' "$misplaced_tests" >&2
  exit 1
fi
if [[ -n "$(gofmt -l main.go internal cmd locales tests)" ]]; then
  echo "Go sources need gofmt" >&2
  exit 1
fi
catalog_before=$(git hash-object locales/catalog_gen.go)
go run github.com/egoist/mygo/cmd/mygo i18n check
go run github.com/egoist/mygo/cmd/mygo generate
if [[ "$catalog_before" != "$(git hash-object locales/catalog_gen.go)" ]]; then
  echo "Catalog accessors were stale; commit the regenerated locales/catalog_gen.go" >&2
  exit 1
fi
go vet ./...
# The headless UI package intentionally exercises every locale and lifecycle
# path. It is safe to execute its top-level tests in independent processes,
# because each test owns its temporary home and fixture state. CI sets this to
# four to avoid making the three native jobs wait on one serial package. Keep
# the default at one so the ordinary local command remains exactly `go test
# ./...` unless the caller opts into parallel package shards.
app_shards="${DESKTOP_TEST_APP_SHARDS:-1}"
if [[ ! "$app_shards" =~ ^[1-9][0-9]*$ ]]; then
  echo "DESKTOP_TEST_APP_SHARDS must be a positive integer" >&2
  exit 2
fi
if ((app_shards == 1)); then
  go test -timeout 35m ./...
else
  app_package="$(go list ./tests/app)"
  other_packages="$(go list ./... | awk -v app="$app_package" '$0 != app')"
  shard_log="$(mktemp "${TMPDIR:-/tmp}/agent-remote-test-shards.XXXXXX")"
  cleanup_shard_log() {
    rm -f "$shard_log"
  }
  trap cleanup_shard_log EXIT

  bash scripts/test-package-shards.sh "$app_package" "$app_shards" > "$shard_log" 2>&1 &
  app_status_pid=$!
  other_status=0
  if [[ -n "$other_packages" ]]; then
    go test -timeout 35m $other_packages || other_status=$?
  fi
  app_status=0
  wait "$app_status_pid" || app_status=$?
  cat "$shard_log"
  if ((other_status != 0 || app_status != 0)); then
    exit 1
  fi
fi
git diff --check
