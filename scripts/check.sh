#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
for script in scripts/*.sh; do
  bash -n "$script"
done
# Keep test sources in the repository-root tests/ tree. Go's package-wide
# `go test ./...` would still discover a misplaced _test.go file, so enforce
# the project layout explicitly before running the suite.
misplaced_tests="$(rg --files -g '*_test.go' -g '!tests/**' || true)"
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
go test ./...
git diff --check
