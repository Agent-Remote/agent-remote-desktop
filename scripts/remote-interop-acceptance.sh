#!/usr/bin/env bash
# Cross-repository acceptance for the remote session path.
#
# Fixture contracts, host preflight and destructive live evidence stay
# separate. A successful --contracts run proves protocol agreement only.
# --host-only records remote_execution=not_run. --live is the only mode which
# may claim a real Server/Node/SSH session.
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$(dirname -- "${BASH_SOURCE[0]}")")" && pwd)"
CLI_ROOT="${AGENT_REMOTE_CLI_REPO:-$ROOT_DIR/../agent-remote-cli}"
SERVER_ROOT="${AGENT_REMOTE_SERVER_REPO:-$ROOT_DIR/../agent-remote-server}"
NODE_ROOT="${AGENT_REMOTE_NODE_REPO:-$ROOT_DIR/../agent-remote-node}"
EVIDENCE_DIR="${AGENT_REMOTE_INTEROP_EVIDENCE_DIR:-${TMPDIR:-/tmp}/agent-remote-remote-interop}"
CLI_BINARY="${AGENT_REMOTE_INTEROP_CLI_BINARY:-$CLI_ROOT/target/debug/agent-remote}"
FCLAUDE_BINARY="${AGENT_REMOTE_INTEROP_FCLAUDE_BINARY:-$CLI_ROOT/target/debug/fclaude}"
MUTAGEN_BINARY="${AGENT_REMOTE_TEST_MUTAGEN:-}"

usage() {
  cat <<'EOF'
Usage: scripts/remote-interop-acceptance.sh --contracts
       scripts/remote-interop-acceptance.sh --host-only
       scripts/remote-interop-acceptance.sh --live
       scripts/remote-interop-acceptance.sh --sync-chain

--contracts runs pinned Server, Node, Rust CLI and Go desktop contracts. It
uses fixtures and does not prove a real Server/Node/SSH connection.

--host-only checks native binaries, CLI desktop-control capabilities and the
host identity. It writes evidence with remote_execution=not_run.

--live runs --contracts and host preflight, then requires AGENT_REMOTE_HOME,
AGENT_REMOTE_EXACT_ATTACHMENT_TEST_SESSION and
AGENT_REMOTE_EXACT_ATTACHMENT_TEST_DEVICE. It runs the exact Server/Node/SSH
attachment test against that disposable session. Set
AGENT_REMOTE_TEST_MUTAGEN to a pinned native Mutagen binary to also run the
isolated Mutagen daemon protocol probe.

--sync-chain runs the Node repository's disposable real synchronization chain
on a macOS Docker Desktop controller. It requires SYNC_MUTAGEN_SOURCE and a
matching SYNC_CLI_BINARY (or the corresponding AGENT_REMOTE_* aliases). This
is separate from --live because it creates temporary Server/Node/Mutagen state.
EOF
}

die() {
  printf 'remote-interop-acceptance: %s\n' "$1" >&2
  exit 2
}

require_dir() {
  [[ -d "$1" ]] || die "missing repository: $1"
}

require_binary() {
  local path="$1"
  [[ -f "$path" && ! -L "$path" && -x "$path" ]] || die "binary must be an executable regular file: $path"
}

run_contracts() {
  require_dir "$CLI_ROOT"
  require_dir "$SERVER_ROOT"
  require_dir "$NODE_ROOT"
  command -v cargo >/dev/null 2>&1 || die "cargo is required"
  command -v go >/dev/null 2>&1 || die "go is required"
  command -v uv >/dev/null 2>&1 || die "uv is required for the Server pinned environment"

  (cd "$SERVER_ROOT" && uv run pytest -q \
    tests/test_session_attach.py \
    tests/test_session_attach_verify.py \
    tests/test_session_attachment_transfer.py \
    tests/test_node_api.py \
    tests/test_port_forward_integration.py)
  (cd "$NODE_ROOT" && go test ./cmd/agent-remote-attach ./internal/claudeattachments)
  # Execute the fixture contracts here.  `--no-run` only compiles these
  # binaries and used to let a broken desktop-control protocol pass the
  # acceptance gate while the script reported "contracts passed".  The exact
  # attachment test remains ignored by Cargo and is exercised only by
  # --live below after the caller supplies a disposable session/device.
  (cd "$CLI_ROOT" && cargo test --test desktop_control_contract --test desktop_mutagen_environment_contract --test exact_attachment_live_contract)
  (cd "$ROOT_DIR" && go test ./tests/cliadapter ./tests/remote -count=1)
}

write_host_evidence() {
  command -v python3 >/dev/null 2>&1 || die "python3 is required to write host evidence"
  mkdir -p "$EVIDENCE_DIR"
  chmod 700 "$EVIDENCE_DIR"
  local goos goarch revision cli_version fclaude_version mutagen_version
  goos="$(go env GOHOSTOS)"
  goarch="$(go env GOHOSTARCH)"
  case "$goos" in darwin|linux|windows) ;; *) die "unsupported host $goos (FreeBSD is not a release target)" ;; esac
  case "$goarch" in amd64|arm64) ;; *) die "unsupported host architecture $goarch" ;; esac
  [[ -z "${GOOS:-}" && -z "${GOARCH:-}" ]] || die "GOOS/GOARCH selectors are not allowed for host acceptance"
  require_binary "$CLI_BINARY"
  require_binary "$FCLAUDE_BINARY"
  cli_version="$("$CLI_BINARY" --version 2>/dev/null | head -n 1 || true)"
  fclaude_version="$("$FCLAUDE_BINARY" --version 2>/dev/null | head -n 1 || true)"
  [[ -n "$cli_version" ]] || die "agent-remote --version produced no output"
  [[ -n "$fclaude_version" ]] || die "fclaude --version produced no output"
  if [[ -n "$MUTAGEN_BINARY" ]]; then
    require_binary "$MUTAGEN_BINARY"
    mutagen_version="$("$MUTAGEN_BINARY" version 2>/dev/null | head -n 1 || true)"
  else
    mutagen_version="not_provided"
  fi

  local home request response capabilities_file
  home="$(mktemp -d "${TMPDIR:-/tmp}/agent-remote-interop-home.XXXXXX")"
  request='{"protocol_version":1,"request_id":"remote-interop-capabilities","method":"capabilities","params":{}}'
  capabilities_file="$EVIDENCE_DIR/capabilities.json"
  set +e
  response="$(printf '%s' "$request" | env -u GOOS -u GOARCH AGENT_REMOTE_HOME="$home" "$CLI_BINARY" --desktop-control 2>"$EVIDENCE_DIR/capabilities.stderr")"
  local status=$?
  set -e
  [[ "$status" -eq 0 ]] || die "agent-remote capabilities request failed (see $EVIDENCE_DIR/capabilities.stderr)"
  printf '%s\n' "$response" > "$capabilities_file"

  local parsed
  parsed="$(python3 - "$capabilities_file" "$goos" <<'PY'
import json, pathlib, sys
value = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding="utf-8"))
host = sys.argv[2]
if value.get("type") != "result" or value.get("protocol_version") != 1:
    raise SystemExit("invalid desktop-control result")
result = value.get("result")
if not isinstance(result, dict) or result.get("protocol_version") != 1:
    raise SystemExit("invalid capabilities result")
if result.get("platform") != {"darwin":"macos"}.get(host, host):
    raise SystemExit("capabilities platform does not match host")
operations = result.get("operations")
terminal = result.get("terminal_operations")
if not isinstance(operations, list) or not isinstance(terminal, list):
    raise SystemExit("capabilities are missing operation lists")
methods = {item.get("method") for item in operations if isinstance(item, dict)}
terminal_methods = {item.get("method") for item in terminal if isinstance(item, dict)}
required = {"sessions.list", "sessions.create", "nodes.list", "ssh.inspect", "mutagen.environment.preview", "mutagen.environment.apply"}
if not required <= methods:
    raise SystemExit("required remote methods are missing")
if "sessions.attach" not in terminal_methods:
    raise SystemExit("sessions.attach terminal capability is missing")
print(json.dumps({"operations": len(operations), "terminal_operations": len(terminal), "required": sorted(required)}, sort_keys=True))
PY
)" || die "desktop-control capabilities failed closed"
  printf '%s\n' "$parsed" > "$EVIDENCE_DIR/capability-summary.json"

  python3 - "$EVIDENCE_DIR/evidence.json" "$goos" "$goarch" "$cli_version" "$fclaude_version" "$mutagen_version" "$parsed" "$CLI_BINARY" "$FCLAUDE_BINARY" "${MUTAGEN_BINARY:-not_provided}" "$(go version)" "$(go list -m -f '{{.Version}}' github.com/egoist/mygo 2>/dev/null || printf unknown)" <<'PY'
import json, pathlib, sys
_, output, goos, goarch, cli_version, fclaude_version, mutagen_version, parsed, cli_binary, fclaude_binary, mutagen_binary, go_version, mygo_version = sys.argv
payload = {
  "schema_version": 1,
  "probe": "agent-remote-desktop.remote-interop",
  "native": True,
  "cross_compilation": False,
  "runtime": {"goos": goos, "goarch": goarch, "go_version": go_version, "mygo_version": mygo_version},
  "binaries": {"agent_remote": {"path": cli_binary, "version": cli_version}, "fclaude": {"path": fclaude_binary, "version": fclaude_version}, "mutagen": {"path": mutagen_binary, "version": mutagen_version}},
  "desktop_control": {"status": "passed", "capability_summary": json.loads(parsed)},
  "remote_execution": {"status": "not_run", "server": "not_run", "node": "not_run", "ssh": "not_run", "fclaude": "not_run", "mutagen": "not_run", "permission": "not_run", "restart_recovery": "not_run", "reason": "host-only preflight does not claim a remote connection"}
}
pathlib.Path(output).write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  printf 'remote-interop-acceptance: host preflight passed; remote_execution=not_run evidence=%s\n' "$EVIDENCE_DIR"
}

run_live() {
  [[ -n "${AGENT_REMOTE_HOME:-}" ]] || die "AGENT_REMOTE_HOME is required in --live mode"
  [[ -n "${AGENT_REMOTE_EXACT_ATTACHMENT_TEST_SESSION:-}" ]] || die "AGENT_REMOTE_EXACT_ATTACHMENT_TEST_SESSION is required"
  [[ -n "${AGENT_REMOTE_EXACT_ATTACHMENT_TEST_DEVICE:-}" ]] || die "AGENT_REMOTE_EXACT_ATTACHMENT_TEST_DEVICE is required"
  command -v ssh >/dev/null 2>&1 || die "ssh is required in --live mode"
  (cd "$ROOT_DIR" && AGENT_REMOTE_INTEROP_EVIDENCE_DIR="$EVIDENCE_DIR" scripts/exact-attachment-acceptance.sh --live)
  local mutagen_probe_status="not_run"
  if [[ -n "$MUTAGEN_BINARY" ]]; then
    # The Rust native probe is Unix-only (`#![cfg(unix)]`). Cargo exits zero
    # with zero tests on Windows, which must never be promoted to a passed
    # Mutagen evidence record.
    if [[ "$(go env GOHOSTOS)" == "windows" ]]; then
      printf 'remote-interop-acceptance: Mutagen native probe is Unix-only; mutagen=not_run on Windows\n' >&2
    else
      (cd "$CLI_ROOT" && AGENT_REMOTE_TEST_MUTAGEN="$MUTAGEN_BINARY" cargo test --test mutagen_native_contract -- --nocapture)
      mutagen_probe_status="passed"
    fi
  else
    printf 'remote-interop-acceptance: Mutagen live probe not run; set AGENT_REMOTE_TEST_MUTAGEN to a pinned native binary\n' >&2
  fi
  python3 - "$EVIDENCE_DIR/evidence.json" "$mutagen_probe_status" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
value = json.loads(path.read_text(encoding="utf-8"))
remote = value.setdefault("remote_execution", {})
remote.update({"status":"partial", "server":"passed", "node":"passed", "ssh":"passed", "fclaude":"not_run", "mutagen":sys.argv[2], "permission":"passed", "restart_recovery":"not_run"})
remote["reason"] = "exact attachment live acceptance completed for Server/Node/SSH; the fclaude executable was version-checked but an interactive fclaude attach was not claimed"
path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
  printf 'remote-interop-acceptance: live Server/Node/SSH acceptance passed; overall evidence is partial until fclaude, Mutagen and restart recovery are separately exercised; evidence=%s\n' "$EVIDENCE_DIR"
}

run_sync_chain() {
  (cd "$ROOT_DIR" && scripts/sync-chain-acceptance.sh)
}

case "${1:-}" in
  --contracts) run_contracts; printf 'remote-interop-acceptance: contract fixtures passed; live evidence remains pending\n' ;;
  --host-only) write_host_evidence ;;
  --live) run_contracts; write_host_evidence; run_live ;;
  --sync-chain) run_sync_chain ;;
  -h|--help) usage ;;
  *) usage >&2; exit 2 ;;
esac
