#!/usr/bin/env bash
# Run the opt-in real Server/Node/SSH/Mutagen synchronization chain.
#
# This is deliberately a separate entry point from protocol contracts and
# exact attachment acceptance. It invokes the Node repository's disposable
# Docker Desktop harness; a missing prerequisite is an error and is never
# recorded as a successful or partial remote run.
set -euo pipefail

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$(dirname -- "${BASH_SOURCE[0]}")")" && pwd)"
NODE_ROOT="${AGENT_REMOTE_NODE_REPO:-$ROOT_DIR/../agent-remote-node}"
SERVER_ROOT="${SYNC_SERVER_REPO:-${AGENT_REMOTE_SERVER_REPO:-$ROOT_DIR/../agent-remote-server}}"
CLI_BINARY="${SYNC_CLI_BINARY:-${AGENT_REMOTE_INTEROP_CLI_BINARY:-$ROOT_DIR/../agent-remote-cli/target/debug/agent-remote}}"
MUTAGEN_SOURCE="${SYNC_MUTAGEN_SOURCE:-${AGENT_REMOTE_SYNC_MUTAGEN_SOURCE:-}}"
EVIDENCE_DIR="${SYNC_CHAIN_EVIDENCE_DIR:-${AGENT_REMOTE_INTEROP_EVIDENCE_DIR:-${TMPDIR:-/tmp}/agent-remote-sync-chain}}"

usage() {
  cat <<'EOF'
Usage: scripts/sync-chain-acceptance.sh

Runs the Node repository's opt-in real synchronization chain. The controller
must be macOS with Docker Desktop; the disposable Node target runs Linux. Set:

  SYNC_CLI_BINARY        built agent-remote executable (agent-remote-ssh and
                         agent-remote-scp must be adjacent to it)
  SYNC_MUTAGEN_SOURCE    reviewed Mutagen source checkout (v0.18.1)
  SYNC_SERVER_REPO       agent-remote-server checkout (optional)
  AGENT_REMOTE_NODE_REPO agent-remote-node checkout (optional)

The test creates temporary Server, Node, SSH and Mutagen state, and removes it
on exit. It is real integration evidence; contract results are not a fallback.
EOF
}

die() {
  printf 'sync-chain-acceptance: %s\n' "$1" >&2
  exit 2
}

require_dir() {
  [[ -d "$1" ]] || die "missing directory: $1"
}

require_binary() {
  local path="$1"
  [[ -f "$path" && ! -L "$path" && -x "$path" ]] || die "binary must be an executable regular file: $path"
}

write_evidence() {
  local status="$1" reason="$2" log_path="$3" host_os="$4" host_arch="$5" docker_arch="$6" restart_recovery="$7"
  python3 - "$EVIDENCE_DIR/evidence.json" "$status" "$reason" "$log_path" "$host_os" "$host_arch" "$docker_arch" "$restart_recovery" "$CLI_BINARY" "$MUTAGEN_SOURCE" <<'PY'
import json
import pathlib
import sys

(
    output,
    status,
    reason,
    log_path,
    host_os,
    host_arch,
    docker_arch,
    restart_recovery,
    cli_binary,
    mutagen_source,
) = sys.argv[1:]
passed = status == "passed"
payload = {
    "schema_version": 1,
    "probe": "agent-remote-desktop.sync-chain",
    "native": True,
    "cross_compilation": False,
    "runtime": {
        "controller_os": host_os,
        "controller_arch": host_arch,
        "docker_arch": docker_arch,
        "container_os": "linux",
    },
    "binaries": {
        "agent_remote": {"path": cli_binary},
        "mutagen_source": {"path": mutagen_source},
    },
    "sync_chain": {
        "status": status,
        "execution": "real_disposable_docker",
        "log": log_path,
        "reason": reason,
    },
    "remote_execution": {
        "status": "passed" if passed else "failed",
        "server": "passed" if passed else "failed",
        "node": "passed" if passed else "failed",
        "ssh": "passed" if passed else "failed",
        "mutagen": "passed" if passed else "failed",
        "permission": "passed" if passed else "failed",
        "restart_recovery": restart_recovery,
        "restart_recovery_scope": "post_restart_persistence_and_mutation"
        if restart_recovery == "passed"
        else "not_claimed",
    },
}
path = pathlib.Path(output)
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(json.dumps(payload, indent=2, sort_keys=True) + "\n", encoding="utf-8")
PY
}

run() {
  [[ -z "${GOOS:-}" && -z "${GOARCH:-}" ]] || die "GOOS/GOARCH selectors are not allowed for live sync-chain acceptance"
  command -v go >/dev/null 2>&1 || die "go is required"
  command -v docker >/dev/null 2>&1 || die "Docker Desktop is required"
  command -v python3 >/dev/null 2>&1 || die "python3 is required for evidence"
  [[ "$(go env GOHOSTOS)" == "darwin" ]] || die "the sync-chain controller requires macOS Docker Desktop"
  require_dir "$NODE_ROOT"
  require_dir "$SERVER_ROOT"
  [[ -x "$SERVER_ROOT/.venv/bin/python" ]] || die "Server pinned Python environment is missing: $SERVER_ROOT/.venv/bin/python"
  require_binary "$CLI_BINARY"
  require_binary "$(dirname "$CLI_BINARY")/agent-remote-ssh"
  require_binary "$(dirname "$CLI_BINARY")/agent-remote-scp"
  [[ -n "$MUTAGEN_SOURCE" ]] || die "SYNC_MUTAGEN_SOURCE is required"
  require_dir "$MUTAGEN_SOURCE"
  [[ -f "$MUTAGEN_SOURCE/cmd/mutagen/main.go" ]] || die "Mutagen source checkout is missing cmd/mutagen/main.go"
  [[ -f "$NODE_ROOT/tests/linux_sync_chain_test.sh" ]] || die "Node sync-chain launcher is missing"

  mkdir -p "$EVIDENCE_DIR"
  chmod 700 "$EVIDENCE_DIR"
  local log_path="$EVIDENCE_DIR/sync-chain.log"
  local host_os host_arch docker_arch status reason
  host_os="$(go env GOHOSTOS)"
  host_arch="$(go env GOHOSTARCH)"
  docker_arch="$(docker version --format '{{.Server.Arch}}' 2>/dev/null || true)"
  [[ -n "$docker_arch" ]] || die "Docker daemon is unavailable"

  set +e
  (
    cd "$NODE_ROOT"
    SYNC_SERVER_REPO="$SERVER_ROOT" \
      SYNC_CLI_BINARY="$CLI_BINARY" \
      SYNC_MUTAGEN_SOURCE="$MUTAGEN_SOURCE" \
      bash tests/linux_sync_chain_test.sh
  ) 2>&1 | tee "$log_path"
  status="${PIPESTATUS[0]}"
  set -e
  local restart_recovery="not_run"
  if grep -Fxq 'RESTART_RECOVERY_STATUS=passed' "$log_path"; then
    restart_recovery="passed"
  elif grep -Fxq 'RESTART_RECOVERY_STATUS=failed' "$log_path"; then
    restart_recovery="failed"
  fi
  if [[ "$status" -eq 0 ]]; then
    if [[ "$restart_recovery" != "passed" ]]; then
      reason="real sync chain passed but restart recovery marker was absent; acceptance remains incomplete"
      write_evidence failed "$reason" "$log_path" "$host_os" "$host_arch" "$docker_arch" "$restart_recovery"
      die "$reason; evidence=$EVIDENCE_DIR"
    fi
    reason="real disposable Server/Node/SSH/Mutagen chain passed"
    write_evidence passed "$reason" "$log_path" "$host_os" "$host_arch" "$docker_arch" "$restart_recovery"
    printf 'sync-chain-acceptance: passed; evidence=%s\n' "$EVIDENCE_DIR"
    return 0
  fi
  reason="real disposable Server/Node/SSH/Mutagen chain failed (see log)"
  write_evidence failed "$reason" "$log_path" "$host_os" "$host_arch" "$docker_arch" "$restart_recovery"
  die "$reason; evidence=$EVIDENCE_DIR"
}

case "${1:-}" in
  "") run ;;
  -h|--help) usage ;;
  *) usage >&2; exit 2 ;;
esac
