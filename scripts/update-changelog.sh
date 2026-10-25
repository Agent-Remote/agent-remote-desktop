#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <version>" >&2
  exit 2
fi

version="${1#v}"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.+][0-9A-Za-z.-]+)?$ ]]; then
  echo "Invalid semantic version: $1" >&2
  exit 2
fi

date="${RELEASE_DATE:-$(date -u +%Y-%m-%d)}"
previous_tag="$(git tag --list 'v[0-9]*' --sort=-v:refname | grep -v "^v${version}$" | head -n 1 || true)"
if [[ -n "$previous_tag" ]]; then
  range="${previous_tag}..HEAD"
else
  range="HEAD"
fi

temporary="$(mktemp "${TMPDIR:-/tmp}/agent-remote-desktop-changelog.XXXXXX")"
trap 'rm -f "$temporary"' EXIT
{
  echo "# Changelog"
  echo
  echo "All notable changes to this repository are recorded here."
  echo
  echo "## v${version} - ${date}"
  echo
  notes="$(git log --no-merges --pretty=format:'- %s (%h)' "$range" || true)"
  if [[ -n "$notes" ]]; then
    printf '%s\n' "$notes"
  else
    echo "- chore: prepare v${version} from ${previous_tag:-the initial source} with repository-owned version metadata only."
  fi
  echo
  if [[ -f CHANGELOG.md ]]; then
    awk -v version="v${version}" '
      /^## / {
        keep = 1
        skip = ($2 == version)
      }
      keep && !skip { print }
    ' CHANGELOG.md
  fi
} > "$temporary"
mv -f "$temporary" CHANGELOG.md
trap - EXIT
