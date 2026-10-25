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
# path and must remain serial within a runner because native renderer/event
# resources are shared. CI runs that package in isolated shard jobs; native
# jobs set DESKTOP_TEST_APP_SKIP=1 and cover every other package here.
if [[ "${DESKTOP_TEST_APP_SKIP:-0}" == 1 ]]; then
  app_package="$(go list ./tests/app)"
  other_packages="$(go list ./... | awk -v app="$app_package" '$0 != app')"
  go test -timeout 35m $other_packages
else
  go test -timeout 35m ./...
fi
git diff --check
