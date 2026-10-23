#!/usr/bin/env bash
# Exercise a packaged release on the host that will run it.
#
# This is separate from native-acceptance.sh: it consumes an already-built
# release directory, verifies its manifest, installs it into a temporary
# user-writable directory, checks installer status, and launches the packaged
# binary with --version. It never writes to a system install path, downloads,
# elevates, or changes GOOS/GOARCH.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RELEASE_DIR="${1:-}"
if (($# != 1)) || [[ -z "$RELEASE_DIR" || "$RELEASE_DIR" == -* ]]; then
  echo "Usage: scripts/release-native-smoke.sh RELEASE_DIR" >&2
  exit 2
fi
RELEASE_DIR="$(cd "$RELEASE_DIR" && pwd)"

command -v go >/dev/null 2>&1 || { echo "release-native-smoke: go is required" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "release-native-smoke: python3 is required" >&2; exit 2; }

verify_args=("$RELEASE_DIR")
if [[ "${DESKTOP_ALLOW_ADHOC:-0}" == 1 ]]; then
  verify_args+=(--allow-adhoc)
fi
"$ROOT_DIR/scripts/verify-release.sh" "${verify_args[@]}"

# Use GOHOST* so an inherited cross-build selector cannot turn this into
# target-only evidence. The selected target must be present in the manifest.
goos="$(go env GOHOSTOS)"
goarch="$(go env GOHOSTARCH)"
target="$goos/$goarch"
target_key="${target//\//-}"

paths=()
while IFS= read -r path; do
  [[ -n "$path" ]] && paths+=("$path")
done < <(python3 - "$RELEASE_DIR" "$target" <<'PY'
import json
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
target = sys.argv[2]
manifest = json.loads((root / "metadata" / "release-manifest.json").read_text(encoding="utf-8"))
if target not in manifest.get("targets", []):
    raise SystemExit(f"release does not contain host target {target}")
prefix = f"artifacts/{target.replace('/', '-')}/"
for entry in manifest.get("artifacts", []):
    path = entry.get("path", "")
    if path.startswith(prefix):
        print(path)
PY
)

installer_rel=""
app_rel=""
for rel in "${paths[@]}"; do
  base="${rel##*/}"
  if [[ "$base" == "agent-remote-desktop-installer" || "$base" == "agent-remote-desktop-installer.exe" ]]; then
    installer_rel="$rel"
  fi
  case "$goos" in
    windows)
      [[ "$base" == "agent-remote-desktop.exe" ]] && app_rel="$rel"
      ;;
    linux)
      [[ "$base" == "agent-remote-desktop" ]] && app_rel="$rel"
      ;;
    darwin)
      if [[ "$rel" == *.app/Contents/MacOS/* ]]; then app_rel="$rel"; fi
      ;;
  esac
done
[[ -n "$installer_rel" ]] || { echo "release-native-smoke: installer artifact is missing for $target" >&2; exit 1; }
[[ -n "$app_rel" ]] || { echo "release-native-smoke: desktop executable is missing for $target" >&2; exit 1; }

tmp_root="$(mktemp -d "${TMPDIR:-/tmp}/agent-remote-desktop-release-native.XXXXXX")"
trap 'rm -rf -- "$tmp_root"' EXIT
destination="$tmp_root/installed"
installer="$RELEASE_DIR/$installer_rel"

"$installer" --release "$RELEASE_DIR" --destination "$destination" --platform "$goos" --arch "$goarch"
status_json="$tmp_root/status.json"
"$installer" --status --destination "$destination" > "$status_json"
release_version="$(python3 - "$RELEASE_DIR" <<'PY'
import json
import pathlib
import sys
manifest = json.loads((pathlib.Path(sys.argv[1]) / "metadata" / "release-manifest.json").read_text(encoding="utf-8"))
print(manifest["app"]["version"])
PY
)"
python3 - "$status_json" "$target" "$release_version" <<'PY'
import json
import pathlib
import sys
status = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
current = status.get("current")
if (
    status.get("installed") is not True
    or not isinstance(current, dict)
    or current.get("target") != sys.argv[2]
    or current.get("version") != sys.argv[3]
):
    raise SystemExit(f"unexpected installer status: {status}")
PY

app="$destination/${app_rel#artifacts/"$target_key"/}"
[[ -f "$app" ]] || { echo "release-native-smoke: installed executable is missing: $app" >&2; exit 1; }
if [[ "$goos" == darwin ]]; then
  command -v codesign >/dev/null 2>&1 || {
    echo "release-native-smoke: codesign is required for an installed macOS bundle" >&2
    exit 2
  }
  # The installer copies the complete .app tree into a temporary user
  # directory. Walk from the actual executable selected from the manifest to
  # that installed bundle and verify the copied bundle, rather than relying
  # only on the source release directory's signature check.
  app_bundle="$app"
  while [[ "$app_bundle" != "$destination" && "$app_bundle" != *.app ]]; do
    app_bundle="$(dirname "$app_bundle")"
  done
  [[ "$app_bundle" == *.app && -d "$app_bundle" ]] || {
    echo "release-native-smoke: installed macOS app bundle is missing: $app_bundle" >&2
    exit 1
  }
  codesign --verify --deep --strict --verbose=2 "$app_bundle" >/dev/null || {
    echo "release-native-smoke: installed macOS app bundle failed code-signature verification: $app_bundle" >&2
    exit 1
  }
fi
version_output="$(env -u GOOS -u GOARCH "$app" --version)"
expected_version="agent-remote-desktop $release_version"
if [[ "$version_output" != "$expected_version" ]]; then
  echo "release-native-smoke: installed binary version mismatch: got '$version_output', want '$expected_version'" >&2
  exit 1
fi
printf '%s\n' "$version_output"
printf 'release native smoke passed: %s (%s)\n' "$target" "$destination"
