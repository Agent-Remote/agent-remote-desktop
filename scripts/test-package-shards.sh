#!/usr/bin/env bash
# Run one deterministic shard of a Go test package. The package is compiled
# once on the runner, then the selected shard executes the test binary with a
# disjoint -test.run expression. CI places different shards on separate hosts
# so native UI resources remain serial within each runner.
set -euo pipefail

if (($# != 3)); then
  printf 'usage: %s <package> <shard-count> <shard-index>\n' "$0" >&2
  exit 2
fi

package="$1"
shards="$2"
shard_index="$3"
if [[ ! "$shards" =~ ^[1-9][0-9]*$ ]]; then
  printf 'test-package-shards: shard count must be a positive integer: %s\n' "$shards" >&2
  exit 2
fi
if [[ ! "$shard_index" =~ ^[0-9]+$ ]] || ((shard_index >= shards)); then
  printf 'test-package-shards: shard index must be between 0 and %s: %s\n' "$((shards - 1))" "$shard_index" >&2
  exit 2
fi

package_dir="$(go list -f '{{.Dir}}' "$package")"
work_dir="$(mktemp -d "${TMPDIR:-/tmp}/agent-remote-test-shards.XXXXXX")"
cleanup() {
  rm -rf "$work_dir"
}
trap cleanup EXIT

binary="$work_dir/test"
if [[ "$(go env GOHOSTOS)" == windows ]]; then
  binary+='.exe'
fi
go test -c -o "$binary" "$package"

"$binary" -test.timeout 35m -test.list '^Test' | awk '/^Test[[:alnum:]_]*/ { print }' > "$work_dir/tests.txt"
test_count="$(wc -l < "$work_dir/tests.txt" | tr -d '[:space:]')"
if [[ "$test_count" == 0 ]]; then
  printf 'test-package-shards: package has no top-level tests: %s\n' "$package" >&2
  exit 1
fi

for ((shard = 0; shard < shards; shard++)); do
  : > "$work_dir/tests.$shard"
done
index=0
while IFS= read -r test_name; do
  shard=$((index % shards))
  printf '%s\n' "$test_name" >> "$work_dir/tests.$shard"
  index=$((index + 1))
done < "$work_dir/tests.txt"

shard_test_count="$(wc -l < "$work_dir/tests.$shard_index" | tr -d '[:space:]')"
pattern=''
while IFS= read -r test_name; do
  if [[ -n "$pattern" ]]; then
    pattern+='|'
  fi
  pattern+="$test_name"
done < "$work_dir/tests.$shard_index"
printf 'test-package-shards: shard %s/%s, %s of %s tests (%s)\n' "$((shard_index + 1))" "$shards" "$shard_test_count" "$test_count" "$package"
cd "$package_dir"
"$binary" -test.timeout 35m -test.run "^(${pattern})$"
