#!/usr/bin/env bash
# Verify host-only native acceptance evidence before a formal release.
#
# The native acceptance probes write evidence independently on each host. This
# verifier joins those records without executing a cross-compiled binary. A
# formal release needs one completed, non-virtual desktop run for each of
# macOS, Windows and Linux. CI/Xvfb evidence can be reviewed explicitly with
# --allow-virtual-display, but it is never accepted as formal desktop proof.
set -euo pipefail

ALLOW_VIRTUAL_DISPLAY=0
REQUIRE_WIREGUARD=0
REQUIRE_LINUX_DISPLAY_PAIR=0
EVIDENCE_DIRS=()
while (($# > 0)); do
  case "$1" in
    --allow-virtual-display)
      ALLOW_VIRTUAL_DISPLAY=1
      shift
      ;;
    --require-wireguard)
      REQUIRE_WIREGUARD=1
      shift
      ;;
    --require-linux-display-pair)
      REQUIRE_LINUX_DISPLAY_PAIR=1
      shift
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/verify-native-evidence.sh [--allow-virtual-display] [--require-wireguard] [--require-linux-display-pair] DIR [DIR ...]

Verify native-acceptance evidence for the formal macOS/Windows/Linux release
targets (darwin/arm64, windows/amd64 and linux/amd64). Formal verification
requires a host display; use the explicit virtual-display flag only when
reviewing Xvfb/CI evidence.
--require-wireguard additionally requires completed native broker evidence.
--require-linux-display-pair additionally requires two Linux records, one
using a native X11 session and one using a native Wayland session. This is a
formal Linux display review gate; the default CI review remains one Linux
record and may use Xvfb with --allow-virtual-display.
EOF
      exit 0
      ;;
    -* )
      echo "verify-native-evidence: unknown option: $1" >&2
      exit 2
      ;;
    *)
      EVIDENCE_DIRS+=("$1")
      shift
      ;;
  esac
done

((${#EVIDENCE_DIRS[@]} > 0)) || {
  echo "verify-native-evidence: at least one evidence directory is required" >&2
  exit 2
}

command -v python3 >/dev/null 2>&1 || {
  echo "verify-native-evidence: python3 is required" >&2
  exit 2
}

python3 - "$ALLOW_VIRTUAL_DISPLAY" "$REQUIRE_WIREGUARD" "$REQUIRE_LINUX_DISPLAY_PAIR" "${EVIDENCE_DIRS[@]}" <<'PY'
from __future__ import annotations

import json
import pathlib
import sys

allow_virtual = sys.argv[1] == "1"
require_wireguard = sys.argv[2] == "1"
require_linux_display_pair = sys.argv[3] == "1"
def fail(message: str) -> None:
    raise SystemExit(f"verify-native-evidence: {message}")


raw_roots = [pathlib.Path(item) for item in sys.argv[4:]]
for raw_root in raw_roots:
    if raw_root.is_symlink():
        fail(f"evidence directory must not be a symlink: {raw_root}")
roots = [item.resolve() for item in raw_roots]


def read_pairs(path: pathlib.Path, label: str) -> dict[str, str]:
    if path.is_symlink() or not path.is_file():
        fail(f"{label} is missing or is not a regular file: {path}")
    pairs: dict[str, str] = {}
    try:
        # Windows PowerShell 5 writes `-Encoding utf8` with a UTF-8 BOM,
        # whereas PowerShell 7 writes the same file without one.  Accept
        # either representation so host evidence produced by the supported
        # Windows shells is parsed identically.
        lines = path.read_text(encoding="utf-8-sig").splitlines()
    except OSError as exc:
        fail(f"cannot read {label}: {exc}")
    for line_number, line in enumerate(lines, 1):
        if not line:
            continue
        key, separator, value = line.partition("=")
        if not separator or not key or key in pairs:
            fail(f"invalid or duplicate {label} entry at line {line_number}")
        pairs[key] = value
    return pairs


if len({str(root) for root in roots}) != len(roots):
    fail("evidence directories must be distinct")

records: list[dict[str, object]] = []
seen_platforms: set[str] = set()
linux_display_protocols: set[str] = set()
required_logs = ("platform.log", "installer.log", "native-window.log", "tray.log", "workbench.log")


def read_wireguard_evidence(root: pathlib.Path, runtime: str, label: str) -> dict[str, object] | None:
    """Validate optional broker evidence without treating status-only as execution."""
    path = root / "wireguard" / "evidence.json"
    if not path.exists():
        return None
    if path.is_symlink() or not path.is_file():
        fail(f"{label}: WireGuard evidence is not a regular file")
    try:
        # PowerShell 5 writes JSON with a UTF-8 BOM; PowerShell 7 does not.
        # Accept both encodings just as we do for summary/metadata pairs.
        raw_evidence = path.read_text(encoding="utf-8-sig")
        value = json.loads(raw_evidence)
    except (OSError, json.JSONDecodeError) as exc:
        fail(f"{label}: invalid WireGuard evidence: {exc}")
    if not isinstance(value, dict) or value.get("probe") != "agent-remote-wireguard.native":
        fail(f"{label}: missing native WireGuard probe marker")
    expected_keys = {
        "schema_version", "probe", "native", "cross_compilation", "runtime",
        "installation", "service_identity", "status_handshake", "caller_authentication",
        "up_receipt", "down_receipt", "replay_rejected", "service_restart", "execution",
    }
    if set(value) != expected_keys:
        fail(f"{label}: WireGuard evidence has unexpected or missing fields")
    if value.get("native") is not True or value.get("cross_compilation") is not False:
        fail(f"{label}: WireGuard evidence is not explicitly native")
    runtime_value = value.get("runtime")
    if not isinstance(runtime_value, dict) or f"{runtime_value.get('goos')}/{runtime_value.get('goarch')}" != runtime:
        fail(f"{label}: WireGuard runtime does not match desktop runtime")
    for key in ("installation", "service_identity", "status_handshake", "caller_authentication", "up_receipt", "down_receipt", "replay_rejected", "service_restart", "execution"):
        if not isinstance(value.get(key), dict) or not isinstance(value[key].get("status"), str):
            fail(f"{label}: WireGuard evidence missing {key} status")
    for marker in ("simulated", "fixture", "mock", "xvfb"):
        if marker in raw_evidence.lower():
            fail(f"{label}: WireGuard evidence contains forbidden {marker} marker")
    execution = value["execution"]["status"]
    if execution != "passed":
        fail(f"{label}: WireGuard evidence is incomplete ({execution!r}); formal verification requires execution=passed")
    for key in ("installation", "service_identity", "status_handshake", "caller_authentication", "up_receipt", "down_receipt", "replay_rejected", "service_restart"):
        if value[key]["status"] != "passed":
            fail(f"{label}: WireGuard execution passed without {key}=passed")
    return {
        "execution": execution,
        "installation": value["installation"]["status"],
        "status_handshake": value["status_handshake"]["status"],
    }


def read_platform_report(path: pathlib.Path, runtime: str, label: str) -> dict[str, object]:
    """Validate the structured host-only JSON emitted by platform-probe."""
    if path.is_symlink() or not path.is_file():
        fail(f"{label}: platform probe log is missing or is not a regular file")
    try:
        raw = path.read_text(encoding="utf-8")
    except OSError as exc:
        fail(f"{label}: cannot read platform probe log: {exc}")
    decoder = json.JSONDecoder()
    reports: list[dict[str, object]] = []
    for index, character in enumerate(raw):
        if character != "{":
            continue
        try:
            candidate, _ = decoder.raw_decode(raw[index:])
        except json.JSONDecodeError:
            continue
        if isinstance(candidate, dict) and candidate.get("probe") == "agent-remote-desktop.platform":
            reports.append(candidate)
    if not reports:
        fail(f"{label}: platform.log has no structured host probe report")
    if len(reports) != 1:
        fail(f"{label}: platform.log contains multiple structured host probe reports")
    report = reports[-1]
    if report.get("schema_version") != 1:
        fail(f"{label}: platform probe has unsupported schema")
    if report.get("host_runtime") is not True or report.get("cross_compilation") is not False:
        fail(f"{label}: platform probe is not explicitly host-only")
    goos, goarch = runtime.split("/", 1)
    expected_platform = {"darwin": "macos", "windows": "windows", "linux": "linux"}[goos]
    if report.get("goos") != goos or report.get("goarch") != goarch or report.get("platform") != expected_platform:
        fail(f"{label}: platform probe target does not match summary runtime")
    if report.get("supported") is not True:
        fail(f"{label}: platform probe did not pass required host checks")
    requirements = report.get("requirements")
    if not isinstance(requirements, list) or not requirements:
        fail(f"{label}: platform probe omitted requirements")
    seen_ids: set[str] = set()
    for requirement in requirements:
        if not isinstance(requirement, dict):
            fail(f"{label}: platform probe contains an invalid requirement")
        identifier = requirement.get("id")
        if not isinstance(identifier, str) or not identifier or identifier in seen_ids:
            fail(f"{label}: platform probe contains duplicate/empty requirement IDs")
        # The probe contract is deliberately small.  A malformed or hand
        # edited status must never be interpreted as a successful host check;
        # only the three values emitted by the Go probe are meaningful.
        status = requirement.get("status")
        if not isinstance(status, str) or status not in {"pass", "fail", "unknown"}:
            fail(f"{label}: platform probe contains an invalid status for {identifier}: {status!r}")
        if not isinstance(requirement.get("required"), bool):
            fail(f"{label}: platform probe has a non-boolean required flag for {identifier}")
        seen_ids.add(identifier)
        if requirement.get("required") is True and status != "pass":
            fail(f"{label}: required platform check did not pass: {identifier}")
    required_ids = {"supported-target", "shell", "terminal"}
    # The workbench uses mygo's native-content renderer exclusively. WebView
    # runtimes are loaded lazily by mygo only for page/WebView windows, so keep
    # their observations in host evidence without turning their absence into
    # a false platform failure.
    observed_ids: set[str] = set()
    if goos == "linux":
        required_ids.update({"display", "gtk3", "tray"})
        observed_ids.add("webkitgtk")
    elif goos == "windows":
        observed_ids.add("webview2")
    missing_ids = sorted(required_ids - seen_ids)
    if missing_ids:
        fail(f"{label}: platform probe omitted required checks: {', '.join(missing_ids)}")
    missing_observations = sorted(observed_ids - seen_ids)
    if missing_observations:
        fail(f"{label}: platform probe omitted optional runtime observations: {', '.join(missing_observations)}")
    for identifier in observed_ids:
        # Keep the contract explicit: a future page window must promote the
        # corresponding runtime to required in the probe and verifier rather
        # than silently changing the meaning of existing evidence.
        for requirement in requirements:
            if isinstance(requirement, dict) and requirement.get("id") == identifier and requirement.get("required") is True:
                fail(f"{label}: optional runtime observation is incorrectly required: {identifier}")
    return report


def require_log_marker(path: pathlib.Path, marker: str, label: str) -> str:
    if path.is_symlink() or not path.is_file():
        fail(f"{label}: required probe log is missing or is not a regular file")
    try:
        content = path.read_text(encoding="utf-8")
    except OSError as exc:
        fail(f"{label}: cannot read probe log: {exc}")
    if marker not in content:
        fail(f"{label}: probe log does not contain success marker")
    return content


def read_probe_json(content: str, probe: str, label: str) -> dict[str, object]:
    """Read the last complete JSON object for a probe from a mixed log."""
    decoder = json.JSONDecoder()
    reports: list[dict[str, object]] = []
    for index, character in enumerate(content):
        if character != "{":
            continue
        try:
            candidate, _ = decoder.raw_decode(content[index:])
        except json.JSONDecodeError:
            continue
        if isinstance(candidate, dict) and candidate.get("probe") == probe:
            reports.append(candidate)
    if not reports:
        fail(f"{label}: missing structured {probe} report")
    if len(reports) != 1:
        fail(f"{label}: multiple structured {probe} reports are ambiguous")
    return reports[-1]
for root in roots:
    if root.is_symlink() or not root.is_dir():
        fail(f"evidence directory is missing or is not a real directory: {root}")
    summary = read_pairs(root / "summary.txt", "summary.txt")
    metadata = read_pairs(root / "metadata.txt", "metadata.txt")
    if summary.get("status") != "passed" or summary.get("exit_code") != "0":
        fail(f"{root}: acceptance did not pass ({summary.get('status')}, exit {summary.get('exit_code')})")
    if summary.get("failed_step") != "complete":
        fail(f"{root}: failed_step is not complete")
    if summary.get("cross_compilation") != "false":
        fail(f"{root}: evidence is not explicitly host-only")
    runtime = summary.get("runtime", "")
    if "/" not in runtime:
        fail(f"{root}: runtime must be GOOS/GOARCH")
    goos, goarch = runtime.split("/", 1)
    formal_arch = {"darwin": "arm64", "windows": "amd64", "linux": "amd64"}.get(goos)
    if formal_arch is None or goarch != formal_arch:
        fail(f"{root}: unsupported native target {runtime!r}")
    platform = {"darwin": "macos", "windows": "windows", "linux": "linux"}[goos]
    if platform in seen_platforms and not (require_linux_display_pair and platform == "linux"):
        fail(f"duplicate native platform evidence: {platform}")
    seen_platforms.add(platform)
    if metadata.get("runtime_goos") != goos or metadata.get("runtime_goarch") != goarch:
        fail(f"{root}: summary runtime and metadata host target differ")
    mygo_version = metadata.get("mygo_version", "").strip()
    if not mygo_version or mygo_version == "unknown":
        fail(f"{root}: metadata must record the resolved mygo module version")
    platform_report = read_platform_report(root / "platform.log", runtime, str(root))
    wireguard = read_wireguard_evidence(root, runtime, str(root))
    if require_wireguard and wireguard is None:
        fail(f"{root}: missing required native WireGuard evidence")
    if wireguard is not None and wireguard["execution"] != "passed":
        fail(f"{root}: WireGuard evidence is {wireguard['execution']!r}; status-only or failed broker evidence is incomplete")
    display = metadata.get("ui_display", "")
    if display != "host":
        if not (allow_virtual and goos == "linux" and display == "xvfb"):
            fail(f"{root}: ui_display={display!r} is not formal host-display evidence")
    display_protocol = metadata.get("display_protocol", "")
    expected_protocol = {"darwin": "appkit", "windows": "win32", "linux": ""}[goos]
    if goos == "linux":
        if display_protocol not in {"x11", "wayland"}:
            fail(f"{root}: Linux metadata must identify display_protocol=x11 or wayland")
        if display == "xvfb" and display_protocol != "x11":
            fail(f"{root}: Xvfb evidence must identify display_protocol=x11")
        if display_protocol in linux_display_protocols:
            fail(f"duplicate Linux display protocol evidence: {display_protocol}")
        linux_display_protocols.add(display_protocol)
    elif display_protocol != expected_protocol:
        fail(f"{root}: display_protocol={display_protocol!r} does not match {goos}")
    for name in required_logs:
        path = root / name
        if path.is_symlink() or not path.is_file():
            fail(f"{root}: required probe log is missing: {name}")
    installer_log = require_log_marker(root / "installer.log", f'"probe":"installer"', str(root / "installer.log"))
    installer_report = read_probe_json(installer_log, "installer", str(root / "installer.log"))
    if installer_report.get("platform") != goos or installer_report.get("architecture") != goarch:
        fail(f"{root}: installer probe target does not match summary runtime")
    # The installer probe deliberately uses fixed synthetic releases.  Keep
    # these values exact: accepting merely non-empty strings would allow a
    # hand-edited log to claim that upgrade, downgrade rejection, rollback or
    # the running lock was exercised without proving the intended sequence.
    expected_installer = {
        "initial": "1.0.0",
        "upgraded": "1.1.0",
        "rolled_back": "1.0.0",
        "running_lock": "passed",
    }
    for key, expected in expected_installer.items():
        if installer_report.get(key) != expected:
            fail(f"{root}: installer probe {key}={installer_report.get(key)!r}, want {expected!r}")
    native_window_log = require_log_marker(root / "native-window.log", "native locale switch and window hide/show: true", str(root / "native-window.log"))
    if "PTY exit: 0" not in native_window_log:
        fail(f"{root}: native-window probe did not report a zero PTY exit")
    tray_log = require_log_marker(root / "tray.log", '"probe":"agent-remote-desktop.tray"', str(root / "tray.log"))
    tray_report = read_probe_json(tray_log, "agent-remote-desktop.tray", str(root / "tray.log"))
    if tray_report.get("schema_version") != 1:
        fail(f"{root}: tray probe has unsupported schema")
    if not isinstance(tray_report.get("goos"), str) or not isinstance(tray_report.get("goarch"), str):
        fail(f"{root}: tray probe target fields are malformed")
    if not isinstance(tray_report.get("host_runtime"), bool) or not isinstance(tray_report.get("cross_compilation"), bool):
        fail(f"{root}: tray probe host-runtime fields are malformed")
    if tray_report.get("host_runtime") is not True or tray_report.get("cross_compilation") is not False:
        fail(f"{root}: tray probe is not explicitly host-only")
    if tray_report.get("goos") != goos or tray_report.get("goarch") != goarch:
        fail(f"{root}: tray probe target does not match summary runtime")
    if tray_report.get("status") != "passed":
        fail(f"{root}: tray probe did not pass")
    workbench_log = require_log_marker(root / "workbench.log", "workbench launch, focus, drag, shutdown", str(root / "workbench.log"))
    # The probe emits one stable success line.  Match the complete line so a
    # failure line elsewhere in a mixed log cannot be promoted by a generic
    # `: true` substring.
    workbench_success = "workbench launch, focus, drag, shutdown, exact remote context/reuse/history replacement, exact binding attachment/reuse and history refresh: true"
    if workbench_success not in workbench_log:
        fail(f"{root}: workbench probe did not report the complete success contract")
    records.append({
        "platform": platform,
        "runtime": runtime,
        "evidence": str(root),
        "ui_display": display,
        "platform_probe": {
            "supported": platform_report["supported"],
            "requirements": len(platform_report["requirements"]),
        },
        "wireguard": wireguard,
    })

missing = sorted({"macos", "windows", "linux"} - seen_platforms)
if missing:
    fail("missing native platform evidence: " + ", ".join(missing))
if require_linux_display_pair and linux_display_protocols != {"x11", "wayland"}:
    missing_protocols = sorted({"x11", "wayland"} - linux_display_protocols)
    if missing_protocols:
        fail("missing Linux native display protocol evidence: " + ", ".join(missing_protocols))

print(json.dumps({
    "verified": True,
    "native_runtime": True,
    "cross_compilation": False,
    "virtual_display_allowed": allow_virtual,
    "wireguard_required": require_wireguard,
    "platforms": records,
}, ensure_ascii=False, sort_keys=True))
PY
