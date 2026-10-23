#!/usr/bin/env bash
# Build reproducible community artifacts for agent-remote-desktop.
#
# Packaging is delegated to the mygo version pinned in go.mod. This script
# does not upload artifacts, request Apple notarization, or sign Windows/Linux
# artifacts. It writes a dependency inventory and SHA-256 checksums alongside
# the packages.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEFAULT_PLATFORMS="darwin/arm64,windows/amd64,linux/amd64"
OUT_DIR="${DESKTOP_RELEASE_OUT:-${ROOT_DIR}/dist/release}"
PLATFORMS="${DESKTOP_RELEASE_PLATFORMS:-${DEFAULT_PLATFORMS}}"
CLEAN=0
SKIP_DMG=0
MAC_SIGNING_IDENTITY="${DESKTOP_MACOS_SIGNING_IDENTITY:-${COMMUNITY_SIGNING_IDENTITY:-${SIGNING_IDENTITY:-}}}"
MAC_CERTIFICATE_SHA1="${DESKTOP_MACOS_SIGNER_CERTIFICATE_SHA1:-${COMMUNITY_SIGNER_CERTIFICATE_SHA1:-${SIGNER_CERTIFICATE_SHA1:-}}}"
ALLOW_ADHOC="${DESKTOP_ALLOW_ADHOC:-0}"

usage() {
  cat <<'EOF'
Usage: scripts/build-release.sh [options]

Build the community desktop artifacts with the mygo version pinned in go.mod.

Options:
  --out DIR             output directory (default: dist/release)
  --platforms LIST      comma-separated GOOS/GOARCH targets
                        (default: darwin/arm64,windows/amd64,linux/amd64)
  --clean               remove the selected output directory before building
  --skip-dmg            skip the macOS disk image (useful for a smoke build)
  -h, --help            show this help

Formal macOS builds require DESKTOP_MACOS_SIGNING_IDENTITY and
DESKTOP_MACOS_SIGNER_CERTIFICATE_SHA1 for the persistent project self-signed
identity. Developer ID, notarization, Windows Authenticode and Linux publisher
signing are not used. Set DESKTOP_ALLOW_ADHOC=1 only for an explicit
development smoke build. Set DESKTOP_CLI_*_BIN to record an explicitly
selected CLI binary in the dependency manifest; otherwise the script probes
PATH without bundling CLIs.
EOF
}

die() {
  printf 'build-release: %s\n' "$*" >&2
  exit 2
}

while (($# > 0)); do
  case "$1" in
    --out)
      (($# >= 2)) || die "--out needs a directory"
      OUT_DIR="$2"
      shift 2
      ;;
    --platforms|--platform)
      (($# >= 2)) || die "$1 needs a comma-separated target list"
      PLATFORMS="$2"
      shift 2
      ;;
    --clean)
      CLEAN=1
      shift
      ;;
    --skip-dmg)
      SKIP_DMG=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      die "unknown option: $1 (use --help)"
      ;;
  esac
done

case "$OUT_DIR" in
  /*) ;;
  *) OUT_DIR="${ROOT_DIR}/${OUT_DIR}" ;;
esac

trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

TARGET_ARGS=()
HAS_DARWIN=0
DARWIN_LABELS=()
SEEN_TARGETS=()
IFS=',' read -r -a raw_targets <<< "$PLATFORMS"
for raw in "${raw_targets[@]}"; do
  target="$(trim "$raw")"
  [[ "$target" =~ ^(darwin|windows|linux)/(amd64|arm64)$ ]] ||
    die "unsupported target '$target'; supported targets are darwin/windows/linux amd64/arm64"
  for seen in "${SEEN_TARGETS[@]}"; do
    [[ "$seen" != "$target" ]] || die "duplicate target '$target'"
  done
  SEEN_TARGETS+=("$target")
  TARGET_ARGS+=("$target")
  if [[ "$target" == darwin/* ]]; then
    HAS_DARWIN=1
    DARWIN_LABELS+=("${target//\//-}")
  fi
done
((${#TARGET_ARGS[@]} > 0)) || die "at least one target is required"

if [[ "$CLEAN" == 1 ]]; then
  # Only remove the caller-selected output directory, never the repository.
  if [[ -n "$OUT_DIR" && "$OUT_DIR" != "/" && "$OUT_DIR" != "$ROOT_DIR" ]]; then
    rm -rf -- "$OUT_DIR"
  else
    die "refusing to clean unsafe output directory '$OUT_DIR'"
  fi
elif [[ -e "$OUT_DIR" ]] && [[ -n "$(find "$OUT_DIR" -mindepth 1 -maxdepth 1 -print -quit 2>/dev/null)" ]]; then
  die "output directory '$OUT_DIR' is not empty; use --clean or choose --out"
fi

command -v go >/dev/null 2>&1 || die "Go is required"
command -v python3 >/dev/null 2>&1 || die "python3 is required to write the dependency manifest"
[[ -f "$ROOT_DIR/go.mod" && -f "$ROOT_DIR/mygo.json" ]] || die "run from an agent-remote-desktop checkout"

TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agent-remote-desktop-release.XXXXXX")"
trap 'rm -rf -- "$TEMP_DIR"' EXIT

# Keep a later mygo.json edit from silently turning the community pipeline into
# a commercial-signing or notarization pipeline. The explicit -sign - and
# -skip-notarize below remain the final command-line guard.
python3 - "$ROOT_DIR/mygo.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    config = json.load(source)
macos = config.get("macos", {}) or {}
windows = config.get("windows", {}) or {}
identity = macos.get("signingIdentity", "")
if identity not in ("", "-"):
    raise SystemExit("mygo.json macos.signingIdentity must be empty or '-' for community builds")
if macos.get("notarize"):
    raise SystemExit("mygo.json macos.notarize is not allowed for community builds")
if macos.get("provisioningProfile"):
    raise SystemExit("mygo.json macos.provisioningProfile is not allowed for community builds")
if windows.get("certificate") or windows.get("signCommand"):
    raise SystemExit("mygo.json Windows signing is not allowed for community builds")
PY

SIGN_IDENTITY="-"
if [[ "$HAS_DARWIN" == 1 ]]; then
  if [[ -z "$MAC_SIGNING_IDENTITY" ]]; then
    [[ "$ALLOW_ADHOC" == 1 ]] || die "macOS community builds require DESKTOP_MACOS_SIGNING_IDENTITY and DESKTOP_MACOS_SIGNER_CERTIFICATE_SHA1 (set DESKTOP_ALLOW_ADHOC=1 only for a smoke build)"
    MAC_SIGNING_IDENTITY="-"
  fi
  # An explicit '-' has the same ad-hoc meaning as the default fallback. It
  # must not bypass the explicit smoke-build opt-in merely because the value
  # came from an environment variable instead of the empty-value fallback.
  if [[ "$MAC_SIGNING_IDENTITY" == "-" ]]; then
    [[ "$ALLOW_ADHOC" == 1 ]] || die "macOS ad-hoc signing is smoke-only; set DESKTOP_ALLOW_ADHOC=1 explicitly"
    # Ad-hoc output has no certificate identity. Do not carry a stale
    # certificate fingerprint from the caller's environment into the release
    # manifest, where it could be mistaken for community self-sign evidence.
    MAC_CERTIFICATE_SHA1=""
  elif [[ "$MAC_SIGNING_IDENTITY" == *"Developer ID"* ]]; then
    die "Developer ID identities are not allowed; use the persistent project community self-signed identity"
  else
    [[ "$MAC_SIGNING_IDENTITY" != *$'\n'* ]] || die "macOS signing identity must be one line"
    [[ "$MAC_CERTIFICATE_SHA1" =~ ^[A-Fa-f0-9]{40}$ ]] ||
      die "DESKTOP_MACOS_SIGNER_CERTIFICATE_SHA1 must be 40 hexadecimal characters"
    [[ "$(uname -s)" == "Darwin" ]] || die "persistent macOS community signing must run on macOS"
    command -v security >/dev/null 2>&1 || die "security is required to verify the community certificate"
    command -v openssl >/dev/null 2>&1 || die "openssl is required to verify the community certificate"
    certificate_file="$TEMP_DIR/community-cert.pem"
    security find-certificate -c "$MAC_SIGNING_IDENTITY" -p > "$certificate_file" ||
      die "community signing identity was not found in the keychain"
    actual_certificate_sha1="$(openssl x509 -in "$certificate_file" -noout -fingerprint -sha1 | cut -d= -f2 | tr -d ':' | tr '[:lower:]' '[:upper:]')"
    expected_certificate_sha1="$(printf '%s' "$MAC_CERTIFICATE_SHA1" | tr '[:lower:]' '[:upper:]')"
    [[ "$actual_certificate_sha1" == "$expected_certificate_sha1" ]] ||
      die "community certificate SHA-1 does not match DESKTOP_MACOS_SIGNER_CERTIFICATE_SHA1"
    certificate_subject="$(openssl x509 -in "$certificate_file" -noout -subject -nameopt RFC2253 | sed 's/^subject=//')"
    certificate_issuer="$(openssl x509 -in "$certificate_file" -noout -issuer -nameopt RFC2253 | sed 's/^issuer=//')"
    [[ -n "$certificate_subject" && "$certificate_subject" == "$certificate_issuer" ]] ||
      die "community signing certificate must be self-signed (certificate subject and issuer differ)"
    openssl x509 -in "$certificate_file" -noout -checkend 2592000 >/dev/null ||
      die "community signing certificate expires within 30 days"
    MAC_CERTIFICATE_SHA1="$actual_certificate_sha1"
    SIGN_IDENTITY="$MAC_SIGNING_IDENTITY"
  fi
fi

# A release without a Darwin target still records the macOS policy object for
# consistency. Keep that object internally coherent when the build is an
# unsigned Windows/Linux-only subset.
if [[ "$SIGN_IDENTITY" == "-" ]]; then
  MAC_CERTIFICATE_SHA1=""
fi

mkdir -p "$OUT_DIR"
ARTIFACT_DIR="$OUT_DIR/artifacts"
mkdir -p "$ARTIFACT_DIR"

target_csv="$(IFS=,; printf '%s' "${TARGET_ARGS[*]}")"
mygo_args=(
  run github.com/egoist/mygo/cmd/mygo build
  -platform "$target_csv"
  -o "$ARTIFACT_DIR"
  -sign "$SIGN_IDENTITY"
  -skip-notarize
)
if [[ "$SKIP_DMG" == 1 ]]; then
  mygo_args+=( -skip-dmg )
fi

printf 'Building agent-remote-desktop for %s\n' "$target_csv"
if [[ "$SIGN_IDENTITY" == "-" ]]; then
  printf 'Packaging policy: macOS ad-hoc smoke signing (-); Windows/Linux unsigned; no notarization or upload\n'
else
  printf 'Packaging policy: macOS project community self-signing (%s); Windows/Linux unsigned; no notarization or upload\n' "$SIGN_IDENTITY"
fi
(
  cd "$ROOT_DIR"
  # mygo and the app are pure Go. Pinning cgo off makes cross-build behavior
  # explicit and avoids an accidental host C toolchain dependency. Keep Go's
  # default toolchain selection so the go.mod minimum (currently Go 1.27.1)
  # is honored when the host has an older bootstrap binary.
  CGO_ENABLED="${CGO_ENABLED:-0}" GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" go "${mygo_args[@]}"
)

# mygo derives the Windows executable name from the human-facing application
# name in mygo.json. Normalize it before writing the release manifest so every
# consumer (installer, verifier and documentation) can use the stable product
# slug without handling spaces or display-name changes.
for target in "${TARGET_ARGS[@]}"; do
  if [[ "$target" == windows/* ]]; then
    target_key="${target//\//-}"
    windows_dir="$ARTIFACT_DIR/$target_key"
    windows_binary="$windows_dir/Agent Remote Desktop.exe"
    normalized_binary="$windows_dir/agent-remote-desktop.exe"
    if [[ -f "$windows_binary" && ! -e "$normalized_binary" ]]; then
      mv -- "$windows_binary" "$normalized_binary"
    fi
    [[ -f "$normalized_binary" ]] || die "mygo did not produce the Windows executable for $target"
  fi
done

# Ship the source-independent installer helper alongside each target. It
# validates the release manifest and performs only local atomic replacement;
# it never downloads, elevates, signs, or handles Agent credentials.
for target in "${TARGET_ARGS[@]}"; do
  target_key="${target//\//-}"
  target_goos="${target%%/*}"
  target_goarch="${target##*/}"
  installer_name="agent-remote-desktop-installer"
  [[ "$target_goos" == "windows" ]] && installer_name+=".exe"
  mkdir -p "$ARTIFACT_DIR/$target_key"
  (
    cd "$ROOT_DIR"
    CGO_ENABLED="${CGO_ENABLED:-0}" GOOS="$target_goos" GOARCH="$target_goarch" GOTOOLCHAIN="${GOTOOLCHAIN:-auto}" \
      go build -trimpath -o "$ARTIFACT_DIR/$target_key/$installer_name" ./cmd/release-installer
  )
done

if [[ "$HAS_DARWIN" == 1 && "$SIGN_IDENTITY" != "-" ]]; then
  # Verify every requested Darwin architecture. Checking only the last target
  # allows a multi-architecture release to ship one unsigned or mis-signed
  # bundle while the other bundle passes the policy gate.
  for darwin_label in "${DARWIN_LABELS[@]}"; do
    app_bundle="$(find "$ARTIFACT_DIR/$darwin_label" -maxdepth 1 -type d -name '*.app' -print -quit)"
    [[ -n "$app_bundle" ]] || die "mygo did not produce a macOS app bundle for $darwin_label"
    signature_details="$(codesign --display --verbose=4 "$app_bundle" 2>&1)"
    grep -Fxq "Authority=$SIGN_IDENTITY" <<< "$signature_details" ||
      die "macOS app does not use the configured community signing identity ($darwin_label)"
    grep -Eq '^CodeDirectory .*flags=.*\(.*runtime.*\)' <<< "$signature_details" ||
      die "macOS community app is missing Hardened Runtime ($darwin_label)"
    codesign --verify --deep --strict --verbose=2 "$app_bundle" >/dev/null ||
      die "macOS community app failed deep code-signature verification ($darwin_label)"
  done
fi

MANIFEST_DIR="$OUT_DIR/metadata"
mkdir -p "$MANIFEST_DIR"
GO_VERSION="$(go version)"
MYGO_VERSION="$(cd "$ROOT_DIR" && go list -m -f '{{.Version}}' github.com/egoist/mygo)"
APP_VERSION="$(python3 - "$ROOT_DIR/mygo.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as source:
    print(json.load(source).get("version", ""))
PY
)"
APP_NAME="$(python3 - "$ROOT_DIR/mygo.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as source:
    print(json.load(source).get("name", "agent-remote-desktop"))
PY
)"
APP_IDENTIFIER="$(python3 - "$ROOT_DIR/mygo.json" <<'PY'
import json
import sys
with open(sys.argv[1], encoding="utf-8") as source:
    print(json.load(source).get("identifier", ""))
PY
)"

GIT_COMMIT="$(cd "$ROOT_DIR" && git rev-parse HEAD 2>/dev/null || true)"
GIT_DIRTY="$(cd "$ROOT_DIR" && if [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then printf true; else printf false; fi)"
[[ -n "$GIT_COMMIT" ]] || GIT_COMMIT="uncommitted"

# Keep a plain module list alongside the JSON manifest. It is useful when
# reviewing a release without having the source checkout available.
(
  cd "$ROOT_DIR"
  go list -m all > "$MANIFEST_DIR/go-modules.txt"
  cp go.mod "$MANIFEST_DIR/go.mod"
  cp go.sum "$MANIFEST_DIR/go.sum"
  cp THIRD_PARTY_NOTICES.md "$MANIFEST_DIR/THIRD_PARTY_NOTICES.md"
  # Keep import paths and resolve directories only in a temporary build file;
  # absolute module-cache paths must not leak into a release manifest.
  go list -deps -f '{{printf "%s\t%s" .ImportPath .Dir}}' . | sort -u > "$TEMP_DIR/dependency-dirs.txt"
)

# The helper below reads plugin manifests from the exact module graph used by
# the build and computes hashes of shipped files.
python3 - "$ROOT_DIR" "$OUT_DIR" "$TEMP_DIR/dependency-dirs.txt" "$target_csv" "$GO_VERSION" "$MYGO_VERSION" "$APP_NAME" "$APP_VERSION" "$APP_IDENTIFIER" "$GIT_COMMIT" "$GIT_DIRTY" "$SIGN_IDENTITY" "$MAC_CERTIFICATE_SHA1" <<'PY'
from __future__ import annotations

import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import sys
from datetime import datetime, timezone

root = pathlib.Path(sys.argv[1]).resolve()
out = pathlib.Path(sys.argv[2]).resolve()
dependency_file = pathlib.Path(sys.argv[3]).resolve()
manifest_dir = out / "metadata"
targets = [item for item in sys.argv[4].split(",") if item]
go_version, mygo_version, app_name, app_version, app_identifier = sys.argv[5:10]
git_commit, git_dirty, signing_identity, certificate_sha1 = sys.argv[10:14]
artifact_dir = out / "artifacts"

def sha256(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()

def json_or_none(path: pathlib.Path):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        return None

def probe_command(explicit_env: str, command: str):
    purpose = {
        "agent-remote": "remote account/session management",
        "fclaude": "remote Agent sessions",
        "codex": "local Codex sessions",
        "claude": "local Claude sessions",
    }[command]
    explicit = os.environ.get(explicit_env, "").strip()
    path = explicit or shutil.which(command)
    result = {
        "name": command,
        "required_for": purpose,
        "bundled": False,
        "source": "agent-remote-cli release or the official Agent distribution",
        "license": "GPL-3.0-only" if command in ("agent-remote", "fclaude") else "upstream license; not redistributed",
        "version_probe": [command, "--version"],
        "found": bool(path),
        "selection": "explicit environment variable " + explicit_env if explicit else "PATH",
    }
    if not path:
        result["status"] = "not-found"
        return result
    result["status"] = "found"
    try:
        result["sha256"] = sha256(pathlib.Path(path).resolve())
    except OSError as exc:
        result["status"] = "unreadable"
        result["error"] = str(exc)
        return result
    try:
        completed = subprocess.run(
            [path, "--version"], cwd=root, stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True,
            timeout=10, check=False,
        )
        output = " ".join(completed.stdout.split())
        if output:
            result["version"] = output[:512]
        if completed.returncode:
            result["version_probe_exit_code"] = completed.returncode
    except (OSError, subprocess.TimeoutExpired) as exc:
        result["version_probe_error"] = str(exc)
    return result

cli = [
    probe_command("DESKTOP_CLI_AGENT_REMOTE_BIN", "agent-remote"),
    probe_command("DESKTOP_CLI_FCLAUDE_BIN", "fclaude"),
    probe_command("DESKTOP_CLI_CODEX_BIN", "codex"),
    probe_command("DESKTOP_CLI_CLAUDE_BIN", "claude"),
]

native = []
dependency_dirs = dependency_file.read_text(encoding="utf-8").splitlines()
for line in dependency_dirs:
    if not line:
        continue
    package, _, directory = line.partition("\t")
    if not package or not directory:
        continue
    path = pathlib.Path(directory) / "mygo-plugin.json"
    data = json_or_none(path)
    if not data:
        continue
    native.append({
        "package": package,
        "manifest": "mygo-plugin.json",
        "manifest_sha256": sha256(path),
        "libraries": data.get("libraries", []),
    })

native_files = []
for entry in native:
    for library in entry["libraries"]:
        for target in targets:
            target_key = target.replace("/", "-")
            file_info = library.get("files", {}).get(target_key)
            if not file_info:
                continue
            name = file_info.get("name", "")
            matches = [
                candidate for candidate in (artifact_dir / target_key).rglob(name)
                if candidate.is_file()
            ]
            native_files.append({
                "name": library.get("name", ""),
                "license": "MIT" if library.get("name") == "libghostty-vt" else "see metadata/THIRD_PARTY_NOTICES.md",
                "target": target,
                "source": library.get("source", ""),
                "url": file_info.get("url", ""),
                "expected_sha256": file_info.get("sha256", ""),
                "file": name,
                "checksum_scope": "upstream_unsigned_bytes",
                "post_build_transform": (
                    "macOS code signing embeds a signature in the dylib after mygo copies it"
                    if target.startswith("darwin/") and signing_identity != "-" else None
                ),
                "shipped": [
                    {
                        "path": str(candidate.relative_to(out)),
                        "sha256": sha256(candidate),
                        "matches_manifest": sha256(candidate) == file_info.get("sha256", ""),
                        "matches_upstream": sha256(candidate) == file_info.get("sha256", ""),
                    }
                    for candidate in matches
                ],
            })

artifacts = []
for path in sorted(artifact_dir.rglob("*")):
    if path.is_file():
        artifacts.append({
            "path": str(path.relative_to(out)),
            "size": path.stat().st_size,
            "sha256": sha256(path),
        })

manifest = {
    "schema_version": 1,
    "generated_at": datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z"),
    "app": {"name": app_name, "version": app_version, "identifier": app_identifier},
    "source": {"git_commit": git_commit, "git_dirty": git_dirty},
    "toolchain": {"go": go_version, "mygo_module": "github.com/egoist/mygo", "mygo_version": mygo_version},
    "targets": targets,
    "signing_policy": {
        "macos": {
            "identity": signing_identity,
            "mode": "project-self-signed" if signing_identity != "-" else "ad-hoc/smoke",
            "certificate_sha1": certificate_sha1 or None,
            "notarization": False,
        },
        "windows": {"publisher_code_signing": False},
        "linux": {"publisher_code_signing": False},
    },
    "runtime_requirements": {
        "macos": "macOS 13+; bundled libghostty-vt.dylib; local Agent CLI binaries are external",
        "windows": "Windows 10+ with native UI and ConPTY; WebView2 is optional for future page windows; bundled ghostty-vt.dll; local Agent CLI binaries are external",
        "linux": "GTK 3 and an AppIndicator implementation for tray support; WebKitGTK 4.1/4.0 is optional for future page windows; bundled libghostty-vt.so; local Agent CLI binaries are external",
    },
    "dependencies": {
        "go_modules": "metadata/go-modules.txt",
        "go_mod_sha256": sha256(manifest_dir / "go.mod"),
        "go_sum_sha256": sha256(manifest_dir / "go.sum"),
        "third_party_notices": "metadata/THIRD_PARTY_NOTICES.md",
        "cli": cli,
        "native_manifests": native,
        "native_files": native_files,
    },
    "artifacts": artifacts,
    "verification": {
        "checksums": "SHA256SUMS",
        "command": "sha256sum -c SHA256SUMS (or shasum -a 256 -c SHA256SUMS)",
        "notes": [
            "CLI binaries are not bundled; their presence and hashes are recorded when found during the build.",
            "A project-signed macOS dylib can differ from the unsigned upstream checksum because codesign embeds a signature; the manifest records both values and the post-build transform.",
            "Cross-compilation proves compilation and packaging only; native window, terminal and permission checks remain platform validation tasks.",
        ],
    },
}
(manifest_dir / "release-manifest.json").write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

with (out / "SHA256SUMS").open("w", encoding="utf-8") as destination:
    for entry in artifacts:
        destination.write(f"{entry['sha256']}  {entry['path']}\n")
PY

# Re-open the completed directory through the same offline verifier shipped
# with the repository. This catches a stale/edited manifest or an accidental
# signing-policy change before the directory is handed to a package integrator.
verify_args=("$OUT_DIR")
if [[ "$SIGN_IDENTITY" == "-" && "$HAS_DARWIN" == 1 ]]; then
  verify_args+=(--allow-adhoc)
fi
"$ROOT_DIR/scripts/verify-release.sh" "${verify_args[@]}"

printf '\nRelease output: %s\n' "$OUT_DIR"
printf 'Manifest: %s\n' "$MANIFEST_DIR/release-manifest.json"
printf 'Checksums: %s\n' "$OUT_DIR/SHA256SUMS"
