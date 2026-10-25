#!/usr/bin/env bash
# Run every top-level Go test in one package exactly once across parallel
# processes. The package is compiled once, then each shard executes the same
# test binary with a disjoint -test.run expression.
set -euo pipefail

if (($# != 2)); then
  printf 'usage: %s <package> <shard-count>\n' "$0" >&2
  exit 2
fi

package="$1"
shards="$2"
if [[ ! "$shards" =~ ^[1-9][0-9]*$ ]]; then
  printf 'test-package-shards: shard count must be a positive integer: %s\n' "$shards" >&2
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

printf 'test-package-shards: %s tests across %s shards (%s)\n' "$test_count" "$shards" "$package"
pids=()
active_shards=()
for ((shard = 0; shard < shards; shard++)); do
  pattern=''
  while IFS= read -r test_name; do
    if [[ -n "$pattern" ]]; then
      pattern+='|'
    fi
    pattern+="$test_name"
  done < "$work_dir/tests.$shard"
  if [[ -z "$pattern" ]]; then
    continue
  fi
  active_shards+=("$shard")
  (
    cd "$package_dir"
    "$binary" -test.timeout 35m -test.run "^(${pattern})$"
  ) > "$work_dir/output.$shard" 2>&1 &
  pids+=("$!")
done

status=0
for index in "${!pids[@]}"; do
  if ! wait "${pids[$index]}"; then
    status=1
  fi
done

for shard in "${active_shards[@]}"; do
  if [[ -s "$work_dir/output.$shard" ]]; then
    cat "$work_dir/output.$shard"
  fi
done
if ((status != 0)); then
  printf 'test-package-shards: one or more shards failed\n' >&2
  exit "$status"
fi
