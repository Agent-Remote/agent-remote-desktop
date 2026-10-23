#!/usr/bin/env bash
# Run runtime-only acceptance probes on the host OS.
#
# This entry point deliberately never sets GOOS/GOARCH. A successful run is
# evidence for the current host (and its display/session), while a cross-build
# remains a separate build check. Linux CI may provide Xvfb; that is recorded
# as a virtual display and does not claim a physical desktop environment.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVIDENCE_DIR="${DESKTOP_NATIVE_EVIDENCE_DIR:-${TMPDIR:-/tmp}/agent-remote-desktop-native-acceptance}"
EXPECTED_GOOS="${DESKTOP_NATIVE_EXPECTED_GOOS:-}"
while (($# > 0)); do
  case "$1" in
    --target|--goos)
      (($# >= 2)) || { printf 'native-acceptance: %s needs darwin, windows or linux\n' "$1" >&2; exit 2; }
      EXPECTED_GOOS="$2"
      shift 2
      ;;
    -h|--help)
      cat <<'EOF'
Usage: scripts/native-acceptance.sh [--target darwin|windows|linux]

Run host-only probes. If --target (or DESKTOP_NATIVE_EXPECTED_GOOS) does not
match the executing host, write status=not_run evidence and fail; this entry
point never cross-runs a target binary.
EOF
      exit 0
      ;;
    *)
      printf 'native-acceptance: unknown option: %s\n' "$1" >&2
      exit 2
      ;;
  esac
done
if [[ -n "$EXPECTED_GOOS" ]]; then
  case "$EXPECTED_GOOS" in
    darwin|windows|linux) ;;
    *) printf 'native-acceptance: unsupported target %s (FreeBSD is not planned)\n' "$EXPECTED_GOOS" >&2; exit 2 ;;
  esac
fi
mkdir -p "$EVIDENCE_DIR"
# A caller may intentionally reuse an evidence directory between retries. Do
# not let a previous successful broker/log result be combined with this run's
# summary (especially when the current run does not request WireGuard).
rm -f "$EVIDENCE_DIR"/summary.txt "$EVIDENCE_DIR"/metadata.txt \
  "$EVIDENCE_DIR"/platform.log "$EVIDENCE_DIR"/installer.log \
  "$EVIDENCE_DIR"/native-window.log "$EVIDENCE_DIR"/tray.log "$EVIDENCE_DIR"/workbench.log \
  "$EVIDENCE_DIR"/wireguard.log "$EVIDENCE_DIR"/target-preflight.log "$EVIDENCE_DIR"/failure.log
rm -rf "$EVIDENCE_DIR/wireguard"

cd "$ROOT_DIR"

goos="unknown"
goarch="unknown"
CURRENT_STEP="initialization"
RUN_STATUS="running"
failure_status=0
write_summary() {
  {
    printf 'status=%s\n' "$RUN_STATUS"
    printf 'exit_code=%s\n' "$failure_status"
    printf 'failed_step=%s\n' "$CURRENT_STEP"
    printf 'runtime=%s/%s\n' "$goos" "$goarch"
    printf 'cross_compilation=false\n'
    printf 'evidence=%s\n' "$EVIDENCE_DIR"
		if [[ -f "$EVIDENCE_DIR/metadata.txt" ]]; then
			printf 'ui_display=%s\n' "$(grep '^ui_display=' "$EVIDENCE_DIR/metadata.txt" | cut -d= -f2-)"
		fi
  } > "$EVIDENCE_DIR/summary.txt"
}
on_exit() {
	local status="$?"
	if [[ "$RUN_STATUS" == "running" ]]; then
		RUN_STATUS="failed"
		failure_status="$status"
	fi
	write_summary
}
trap on_exit EXIT

# GOOS/GOARCH are build selectors and may be inherited from a caller's
# cross-build shell.  Native acceptance must always identify the machine that
# will execute the probes, so use GOHOST* and scrub selectors from every
# child process.
goos="$(go env GOHOSTOS)"
goarch="$(go env GOHOSTARCH)"
if [[ -n "$EXPECTED_GOOS" && "$EXPECTED_GOOS" != "$goos" ]]; then
  RUN_STATUS="not_run"
  failure_status=3
  CURRENT_STEP="target-preflight"
  printf 'target=%s\nactual_host=%s\nstatus=not_run\nreason=target host is unavailable; native acceptance never cross-runs\n' "$EXPECTED_GOOS" "$goos" | tee "$EVIDENCE_DIR/target-preflight.log" >&2
  exit 3
fi
case "$goos" in
  darwin|windows|linux) ;;
  *)
    printf 'native-acceptance: unsupported host OS %s (FreeBSD is not planned)\n' "$goos" >&2
    exit 2
    ;;
esac
case "$goarch" in
  amd64|arm64) ;;
  *)
    printf 'native-acceptance: unsupported host architecture %s\n' "$goarch" >&2
    exit 2
    ;;
esac

if command -v git >/dev/null 2>&1; then
  revision="$(git rev-parse --short HEAD 2>/dev/null || printf unknown)"
else
  revision="unknown"
fi
{
  printf 'recorded_at_utc=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'runtime_goos=%s\n' "$goos"
  printf 'runtime_goarch=%s\n' "$goarch"
  printf 'go_version=%s\n' "$(go version)"
  printf 'mygo_version=%s\n' "$(go list -m -f '{{.Version}}' github.com/egoist/mygo 2>/dev/null || printf unknown)"
  printf 'revision=%s\n' "$revision"
  printf 'uname=%s\n' "$(uname -a 2>/dev/null || printf unavailable)"
  printf 'display=%s\n' "${DISPLAY:-${WAYLAND_DISPLAY:-unavailable}}"
  printf 'desktop_session=%s\n' "${XDG_CURRENT_DESKTOP:-${DESKTOP_SESSION:-unavailable}}"
} > "$EVIDENCE_DIR/metadata.txt"

run_step() {
  local name="$1"
  shift
  CURRENT_STEP="$name"
  printf '\n== %s ==\n' "$name"
  printf 'command:'
  printf ' %q' "$@"
  printf '\n'
  # `env -u` keeps an inherited cross-build selector from turning `go run`
  # into a failed or, worse, mislabelled target invocation.
  env -u GOOS -u GOARCH "$@" 2>&1 | tee "$EVIDENCE_DIR/$name.log"
}

# The native UI backends need a display. On headless Linux, Xvfb is an
# explicit virtual display and is kept in metadata so reviewers do not mistake
# it for a window-manager or physical-session run.
UI_RUNNER=()
if [[ "$goos" == linux && -z "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
  command -v xvfb-run >/dev/null 2>&1 || {
    printf 'native-acceptance: Linux has no display; install xvfb-run or set DISPLAY\n' >&2
    exit 3
  }
  printf 'ui_display=xvfb\n' >> "$EVIDENCE_DIR/metadata.txt"
  # Xvfb is an X11 server. Keep this explicit so a virtual-display review
  # cannot be mistaken for a Wayland run and formal review can require both
  # Linux display protocols when needed.
  printf 'display_protocol=x11\n' >> "$EVIDENCE_DIR/metadata.txt"
  UI_RUNNER=(xvfb-run -a)
else
  printf 'ui_display=host\n' >> "$EVIDENCE_DIR/metadata.txt"
  display_protocol="unknown"
  case "$goos" in
    darwin) display_protocol="appkit" ;;
    windows) display_protocol="win32" ;;
    linux)
      # Wayland sessions commonly expose DISPLAY for XWayland as well. The
      # session type is authoritative when present; otherwise infer from the
      # one display variable that is set and fail closed for an ambiguous
      # environment.
      case "${XDG_SESSION_TYPE:-}" in
        wayland) display_protocol="wayland" ;;
        x11) display_protocol="x11" ;;
        *)
          if [[ -n "${WAYLAND_DISPLAY:-}" && -z "${DISPLAY:-}" ]]; then
            display_protocol="wayland"
          elif [[ -n "${DISPLAY:-}" && -z "${WAYLAND_DISPLAY:-}" ]]; then
            display_protocol="x11"
          fi
          ;;
      esac
      ;;
  esac
  printf 'display_protocol=%s\n' "$display_protocol" >> "$EVIDENCE_DIR/metadata.txt"
fi

run_step platform env DESKTOP_NATIVE_ACCEPTANCE=1 "${UI_RUNNER[@]}" go run ./tests/platform-probe
run_step installer env DESKTOP_NATIVE_ACCEPTANCE=1 go run ./tests/installer-probe
run_step native-window env DESKTOP_TERMINAL_TESTS=1 "${UI_RUNNER[@]}" go run ./tests/native-probe
run_step tray env DESKTOP_NATIVE_ACCEPTANCE=1 "${UI_RUNNER[@]}" go run ./tests/tray-probe
if [[ "${DESKTOP_NATIVE_SKIP_WORKBENCH:-0}" == 1 ]]; then
  printf 'workbench=skipped (DESKTOP_NATIVE_SKIP_WORKBENCH=1)\n' | tee "$EVIDENCE_DIR/workbench.log"
else
  run_step workbench env DESKTOP_TERMINAL_TESTS=1 "${UI_RUNNER[@]}" go run ./tests/workbench-probe
fi

# WireGuard broker validation is an explicit host operation.  Do not silently
# manufacture evidence when the Rust CLI binary or an installed service is
# unavailable; the normal desktop probe records that this gate was not run.
if [[ -n "${DESKTOP_NATIVE_WIREGUARD_CLI_BINARY:-}" ]]; then
  cli_script="$ROOT_DIR/../agent-remote-cli/scripts/native-wireguard-broker-acceptance.sh"
  [[ -f "$cli_script" ]] || { printf 'native-acceptance: missing WireGuard broker acceptance script: %s\n' "$cli_script" >&2; exit 4; }
  wireguard_dir="$EVIDENCE_DIR/wireguard"
  mkdir -p "$wireguard_dir"
  run_step wireguard-broker env AGENT_REMOTE_WIREGUARD_EVIDENCE_DIR="$wireguard_dir" AGENT_REMOTE_CLI_BINARY="$DESKTOP_NATIVE_WIREGUARD_CLI_BINARY" bash "$cli_script"
else
  printf 'status=not_run\nreason=DESKTOP_NATIVE_WIREGUARD_CLI_BINARY was not provided; no broker evidence was claimed\n' | tee "$EVIDENCE_DIR/wireguard.log"
fi

RUN_STATUS="passed"
failure_status=0
CURRENT_STEP="complete"
write_summary
cat "$EVIDENCE_DIR/summary.txt"
