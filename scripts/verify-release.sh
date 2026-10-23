#!/usr/bin/env bash
# Verify a packaged agent-remote-desktop release without a source checkout.
#
# This verifier is intentionally offline. It checks the release manifest,
# artifact set and SHA-256 entries, then enforces the community packaging
# policy: a Darwin target must carry the project's self-signed identity, while
# Windows/Linux must declare publisher signing disabled. It never downloads,
# installs, elevates, notarizes or changes an artifact.
set -euo pipefail

ROOT_DIR=""
ALLOW_ADHOC=0
REQUIRED_TARGETS=""
while (($# > 0)); do
  case "$1" in
    --allow-adhoc)
      [[ "$ALLOW_ADHOC" == 0 ]] || {
        echo "verify-release: --allow-adhoc may only be provided once" >&2
        exit 2
      }
      ALLOW_ADHOC=1
      shift
      ;;
    --require-targets)
      [[ "$REQUIRED_TARGETS" == "" ]] || {
        echo "verify-release: --require-targets may only be provided once" >&2
        exit 2
      }
      (($# >= 2)) || {
        echo "verify-release: --require-targets needs a comma-separated target list" >&2
        exit 2
      }
      [[ -n "$2" ]] || {
        echo "verify-release: --require-targets needs a non-empty target list" >&2
        exit 2
      }
      REQUIRED_TARGETS="$2"
      shift 2
      ;;
    -h|--help)
      cat >&2 <<'EOF'
Usage: scripts/verify-release.sh RELEASE_DIR [--allow-adhoc] [--require-targets LIST]

The default verifies a formal community release. --allow-adhoc is only for an
explicit development smoke directory whose macOS manifest says ad-hoc/smoke.
--require-targets enforces the exact comma-separated target set for a formal
release workflow; local smoke verification may omit it.
EOF
      exit 0
      ;;
    -*)
      echo "verify-release: unknown option: $1" >&2
      exit 2
      ;;
    *)
      [[ -z "$ROOT_DIR" ]] || {
        echo "verify-release: expected one release directory" >&2
        exit 2
      }
      ROOT_DIR="$1"
      shift
      ;;
  esac
done

if [[ -z "$ROOT_DIR" || "$ROOT_DIR" == -* ]]; then
  cat >&2 <<'EOF'
Usage: scripts/verify-release.sh RELEASE_DIR [--allow-adhoc] [--require-targets LIST]

The default verifies a formal community release. --allow-adhoc is only for an
explicit development smoke directory whose macOS manifest says ad-hoc/smoke.
--require-targets enforces the exact comma-separated target set for a formal
release workflow; local smoke verification may omit it.
EOF
  exit 2
fi

command -v python3 >/dev/null 2>&1 || {
  echo "verify-release: python3 is required" >&2
  exit 2
}

python3 - "$ROOT_DIR" "$ALLOW_ADHOC" "$REQUIRED_TARGETS" <<'PY'
from __future__ import annotations

import hashlib
import json
import pathlib
import re
import shutil
import subprocess
import sys

root = pathlib.Path(sys.argv[1]).resolve()
allow_adhoc = sys.argv[2] == "1"
required_targets_arg = sys.argv[3]

def fail(message: str) -> None:
    raise SystemExit(f"verify-release: {message}")

if not root.is_dir():
    fail(f"release directory is not a directory: {root}")

manifest_path = root / "metadata" / "release-manifest.json"
checksums_path = root / "SHA256SUMS"
if manifest_path.is_symlink() or not manifest_path.is_file():
    fail("metadata/release-manifest.json is missing or is not a regular file")
if checksums_path.is_symlink() or not checksums_path.is_file():
    fail("SHA256SUMS is missing or is not a regular file")

try:
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
except (OSError, json.JSONDecodeError) as exc:
    fail(f"cannot read release manifest: {exc}")

if manifest.get("schema_version") != 1:
    fail("unsupported manifest schema")
app = manifest.get("app")
if not isinstance(app, dict) or not all(isinstance(app.get(key), str) and app[key] for key in ("name", "version", "identifier")):
    fail("manifest app identity is incomplete")
# The verifier runs outside the source checkout and therefore cannot trust a
# caller-supplied manifest to identify this product.  Keep the package name
# and bundle identifier bound to the desktop project before checking any
# artifact bytes; otherwise a complete, correctly checksummed release for a
# different application could pass this delivery gate.
if app["name"] != "Agent Remote Desktop":
    fail(f"manifest app name is not Agent Remote Desktop: {app['name']!r}")
if app["identifier"] != "dev.agent-remote.desktop":
    fail(f"manifest app identifier is not dev.agent-remote.desktop: {app['identifier']!r}")

targets = manifest.get("targets")
if not isinstance(targets, list) or not targets:
    fail("manifest has no targets")
target_re = re.compile(r"^(darwin|windows|linux)/(amd64|arm64)$")
if len(set(targets)) != len(targets) or any(not isinstance(target, str) or not target_re.fullmatch(target) for target in targets):
    fail("manifest contains an unsupported or duplicate target")
if required_targets_arg:
    required_targets = [item.strip() for item in required_targets_arg.split(",") if item.strip()]
    if (not required_targets or len(set(required_targets)) != len(required_targets)
            or any(not target_re.fullmatch(target) for target in required_targets)):
        fail("--require-targets contains an unsupported or duplicate target")
    if set(targets) != set(required_targets):
        missing = sorted(set(required_targets) - set(targets))
        extra = sorted(set(targets) - set(required_targets))
        fail(f"release targets differ from required set (missing={missing}, extra={extra})")

policy = manifest.get("signing_policy")
if not isinstance(policy, dict):
    fail("manifest signing_policy is missing")
mac = policy.get("macos")
windows = policy.get("windows")
linux = policy.get("linux")
if not isinstance(mac, dict) or not isinstance(windows, dict) or not isinstance(linux, dict):
    fail("manifest signing_policy is incomplete")
if windows.get("publisher_code_signing") is not False:
    fail("Windows publisher code signing must be false for community builds")
if linux.get("publisher_code_signing") is not False:
    fail("Linux publisher code signing must be false for community builds")
if mac.get("notarization") is not False:
    fail("macOS notarization must be false for community builds")

has_darwin = any(target.startswith("darwin/") for target in targets)
mac_mode = mac.get("mode")
identity = mac.get("identity")
if has_darwin:
    if mac_mode == "ad-hoc/smoke":
        if not allow_adhoc:
            fail("ad-hoc macOS output is smoke-only; pass --allow-adhoc explicitly")
        if identity != "-":
            fail("ad-hoc macOS output must record identity '-'")
        if mac.get("certificate_sha1") not in (None, ""):
            fail("ad-hoc macOS output must not record a certificate fingerprint")
    elif mac_mode == "project-self-signed":
        if not isinstance(identity, str) or not identity.strip() or identity == "-":
            fail("macOS community identity is missing")
        if any(ord(character) < 0x20 or ord(character) == 0x7F for character in identity):
            fail("macOS community identity contains a control character")
        if "developer id" in identity.lower():
            fail("Apple Developer ID is not allowed for community releases")
        fingerprint = mac.get("certificate_sha1")
        if not isinstance(fingerprint, str) or not re.fullmatch(r"[A-Fa-f0-9]{40}", fingerprint):
            fail("macOS community certificate_sha1 must be 40 hexadecimal characters")
    else:
        fail(f"unsupported macOS signing mode: {mac_mode!r}")
elif mac_mode not in ("ad-hoc/smoke", "project-self-signed"):
    fail(f"unsupported macOS signing mode: {mac_mode!r}")

def clean_relative(value: object) -> str:
    if not isinstance(value, str) or not value or "\\" in value or value.startswith("/"):
        fail(f"invalid artifact path: {value!r}")
    path = pathlib.PurePosixPath(value)
    if str(path) != value or value in (".", "..") or ".." in path.parts:
        fail(f"invalid artifact path: {value!r}")
    return value

def regular(path: pathlib.Path, label: str) -> None:
    if path.is_symlink() or not path.is_file():
        fail(f"{label} is not a regular file: {path.relative_to(root)}")

def sha256(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()

checksum_entries: dict[str, str] = {}
for line_number, raw in enumerate(checksums_path.read_text(encoding="utf-8").splitlines(), 1):
    line = raw.strip()
    if not line:
        continue
    fields = line.split("  ", 1)
    if len(fields) != 2 or not re.fullmatch(r"[A-Fa-f0-9]{64}", fields[0]):
        fail(f"invalid SHA256SUMS line {line_number}")
    rel = clean_relative(fields[1].strip())
    if rel in checksum_entries:
        fail(f"duplicate SHA256SUMS path: {rel}")
    checksum_entries[rel] = fields[0].lower()
if not checksum_entries:
    fail("SHA256SUMS is empty")

manifest_entries = manifest.get("artifacts")
if not isinstance(manifest_entries, list) or not manifest_entries:
    fail("manifest has no artifacts")
manifest_map: dict[str, tuple[int, str]] = {}
target_keys = {"artifacts/" + target.replace("/", "-") + "/" for target in targets}
for entry in manifest_entries:
    if not isinstance(entry, dict):
        fail("manifest contains a non-object artifact")
    rel = clean_relative(entry.get("path"))
    if not any(rel.startswith(prefix) for prefix in target_keys):
        fail(f"artifact is outside a declared target: {rel}")
    size = entry.get("size")
    digest = entry.get("sha256")
    if not isinstance(size, int) or size < 0 or not isinstance(digest, str) or not re.fullmatch(r"[A-Fa-f0-9]{64}", digest):
        fail(f"invalid artifact metadata: {rel}")
    if rel in manifest_map:
        fail(f"duplicate manifest artifact: {rel}")
    manifest_map[rel] = (size, digest.lower())

for target in targets:
    prefix = "artifacts/" + target.replace("/", "-") + "/"
    if not any(path.startswith(prefix) for path in manifest_map):
        fail(f"declared target has no artifacts: {target}")

# A target directory containing an arbitrary file is not a usable desktop
# release. Require the executable and the source-independent installer helper
# that build-release.sh promises for every formal target. This check is kept
# in the offline verifier so a hand-edited manifest cannot turn a partial
# archive into release evidence.
for target in targets:
    prefix = "artifacts/" + target.replace("/", "-") + "/"
    paths = [path[len(prefix):] for path in manifest_map if path.startswith(prefix)]
    installer = "agent-remote-desktop-installer.exe" if target.startswith("windows/") else "agent-remote-desktop-installer"
    if installer not in paths:
        fail(f"{target}: required installer helper is missing")
    if target.startswith("windows/"):
        executable = "agent-remote-desktop.exe"
        if executable not in paths:
            fail(f"{target}: required desktop executable is missing: {executable}")
    elif target.startswith("linux/"):
        executable = "agent-remote-desktop"
        if executable not in paths:
            fail(f"{target}: required desktop executable is missing: {executable}")
    else:
        if not any(".app/Contents/MacOS/" in path and path.rsplit("/", 1)[-1] for path in paths):
            fail(f"{target}: required macOS app executable is missing")

if set(manifest_map) != set(checksum_entries):
    missing = sorted(set(manifest_map) - set(checksum_entries))
    extra = sorted(set(checksum_entries) - set(manifest_map))
    fail(f"manifest/SHA256SUMS set differs (missing={missing}, extra={extra})")

for rel, (expected_size, expected_hash) in manifest_map.items():
    path = root / pathlib.PurePosixPath(rel)
    regular(path, "artifact")
    actual_size = path.stat().st_size
    if actual_size != expected_size:
        fail(f"artifact size mismatch for {rel}: got {actual_size}, want {expected_size}")
    actual_hash = sha256(path)
    if actual_hash != expected_hash or actual_hash != checksum_entries[rel]:
        fail(f"artifact checksum mismatch for {rel}")

# Reject unlisted files and symlinks below every declared target. This catches
# an installer helper or native library copied after manifest generation.
artifacts_root = root / "artifacts"
if artifacts_root.is_symlink() or not artifacts_root.is_dir():
    fail("artifacts directory is missing or is not a regular directory")
declared_target_dirs = {target.replace("/", "-") for target in targets}
for entry in artifacts_root.iterdir():
    if entry.is_symlink() or not entry.is_dir() or entry.name not in declared_target_dirs:
        fail(f"undeclared target artifact directory: {entry.name}")
for target in targets:
    target_root = root / "artifacts" / target.replace("/", "-")
    if target_root.is_symlink() or not target_root.is_dir():
        fail(f"declared target artifacts directory is missing: {target}")
    for path in target_root.rglob("*"):
        rel = path.relative_to(root).as_posix()
        if path.is_symlink() or (not path.is_dir() and rel not in manifest_map):
            fail(f"unlisted or symlinked target artifact: {rel}")

# A native macOS verifier can inspect the actual bundle. On other hosts the
# manifest and checksum checks remain authoritative, while native signature
# evidence is explicitly reported as deferred instead of being fabricated.
if has_darwin and mac_mode == "project-self-signed" and sys.platform == "darwin":
    identity = mac["identity"]
    expected_fingerprint = mac["certificate_sha1"].upper()
    security = shutil.which("security")
    openssl = shutil.which("openssl")
    if not security or not openssl:
        fail("macOS community release verification requires security and openssl")
    certificate = subprocess.run(
        [security, "find-certificate", "-c", identity, "-p"],
        capture_output=True,
    )
    if certificate.returncode:
        fail(f"community signing identity is not present in the verifier keychain: {identity}")
    fingerprint = subprocess.run(
        [openssl, "x509", "-noout", "-fingerprint", "-sha1"],
        input=certificate.stdout,
        capture_output=True,
    )
    if fingerprint.returncode:
        fail("cannot inspect the community signing certificate")
    fingerprint_text = fingerprint.stdout.decode("utf-8", "replace").strip()
    actual_fingerprint = fingerprint_text.split("=", 1)[-1].replace(":", "").upper()
    if actual_fingerprint != expected_fingerprint:
        fail(
            "community certificate fingerprint does not match manifest "
            f"(got {actual_fingerprint!r}, want {expected_fingerprint!r})"
        )
    subject_issuer = subprocess.run(
        [openssl, "x509", "-noout", "-subject", "-issuer", "-nameopt", "RFC2253"],
        input=certificate.stdout,
        capture_output=True,
    )
    if subject_issuer.returncode:
        fail("cannot inspect the community signing certificate subject/issuer")
    names: dict[str, str] = {}
    for line in subject_issuer.stdout.decode("utf-8", "replace").splitlines():
        key, separator, value = line.partition("=")
        if separator and key in {"subject", "issuer"}:
            names[key] = value.strip()
    if not names.get("subject") or names.get("subject") != names.get("issuer"):
        fail("community signing certificate must be self-signed (subject and issuer differ)")
    bundles = []
    for target in targets:
        if not target.startswith("darwin/"):
            continue
        bundles.extend((root / "artifacts" / target.replace("/", "-")).glob("*.app"))
    if not bundles:
        fail("macOS community release has no .app bundle to verify")
    for bundle in bundles:
        verify = subprocess.run(["codesign", "--verify", "--deep", "--strict", "--verbose=2", str(bundle)], capture_output=True, text=True)
        if verify.returncode:
            fail(f"macOS code signature verification failed for {bundle.name}: {verify.stderr.strip()}")
        details = subprocess.run(["codesign", "--display", "--verbose=4", str(bundle)], capture_output=True, text=True)
        if details.returncode or f"Authority={identity}" not in (details.stdout + details.stderr).splitlines():
            fail(f"macOS bundle does not use configured community identity: {bundle.name}")
    signature_note = "native macOS bundle signature verified"
elif has_darwin and mac_mode == "project-self-signed":
    signature_note = "macOS bundle signature deferred (verifier host is not macOS)"
elif has_darwin and mac_mode == "ad-hoc/smoke":
    # Ad-hoc output is accepted only for explicitly requested smoke review.
    # Keep the report honest: it contains a Darwin target, but there is no
    # project certificate identity to verify.
    signature_note = "macOS ad-hoc smoke signature accepted by explicit opt-in"
else:
    signature_note = "no macOS target"

print(json.dumps({
    "verified": True,
    "release": str(root),
    "version": app["version"],
    "targets": targets,
    "artifact_count": len(manifest_map),
    "signing_policy": {
        "macos_mode": mac_mode,
        "windows_publisher_code_signing": False,
        "linux_publisher_code_signing": False,
        "notarization": False,
    },
    "signature_check": signature_note,
}, ensure_ascii=False))
PY
