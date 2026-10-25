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

python3 - "$version" <<'PY'
from __future__ import annotations

import json
import sys
from pathlib import Path

version = sys.argv[1]
path = Path("mygo.json")
config = json.loads(path.read_text(encoding="utf-8"))
current = str(config.get("version", ""))
if not current:
    raise SystemExit("mygo.json has no version")
config["version"] = version
path.write_text(json.dumps(config, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

main = Path("main.go")
text = main.read_text(encoding="utf-8")
needle = 'const developmentVersion = "'
start = text.find(needle)
if start < 0:
    raise SystemExit("main.go has no developmentVersion constant")
value_start = start + len(needle)
value_end = text.find('"', value_start)
if value_end < 0:
    raise SystemExit("main.go developmentVersion constant is malformed")
text = text[:value_start] + version + text[value_end:]
main.write_text(text, encoding="utf-8")
PY

catalog_before="$(git hash-object locales/catalog_gen.go)"
go run github.com/egoist/mygo/cmd/mygo i18n check
go run github.com/egoist/mygo/cmd/mygo generate
if [[ "$catalog_before" != "$(git hash-object locales/catalog_gen.go)" ]]; then
  echo "Catalog accessors changed during release preparation; commit the generated file" >&2
  exit 1
fi

scripts/update-changelog.sh "$version"
git diff --check
echo "Prepared agent-remote-desktop v${version}"
