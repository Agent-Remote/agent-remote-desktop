#!/usr/bin/env bash
# Acceptance entry point for the exact-session attachment bridge.
#
# --contracts runs the three repository contract suites and Desktop's
# capability/status checks. --live additionally runs the ignored CLI test,
# which opens a real Server-authorized SSH forced-command gateway and verifies
# one upload plus scoped cleanup. The live mode is deliberately fail-closed:
# missing credentials or identity variables are an error, never a skip.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CLI_ROOT=${AGENT_REMOTE_CLI_REPO:-"$ROOT/../agent-remote-cli"}
SERVER_ROOT=${AGENT_REMOTE_SERVER_REPO:-"$ROOT/../agent-remote-server"}
NODE_ROOT=${AGENT_REMOTE_NODE_REPO:-"$ROOT/../agent-remote-node"}

usage() {
  cat <<'EOF'
Usage: scripts/exact-attachment-acceptance.sh --contracts
       scripts/exact-attachment-acceptance.sh --live

--contracts runs Server, Node, CLI and Desktop contract tests. These tests use
fixtures and do not prove a real SSH forced-command transfer.

--live runs --contracts, then the ignored CLI test against the authenticated
home selected by AGENT_REMOTE_HOME. It requires:
  AGENT_REMOTE_HOME
  AGENT_REMOTE_EXACT_ATTACHMENT_TEST_SESSION
  AGENT_REMOTE_EXACT_ATTACHMENT_TEST_DEVICE

The live test uploads one temporary file through the exact gateway and requires
a confirmed cleanup receipt. It must be run only against a disposable test
session; no credentials, payload paths or server responses are printed.
EOF
}

die() {
  echo "exact-attachment-acceptance: $*" >&2
  exit 2
}

require_dir() {
  [[ -d "$1" ]] || die "repository directory is missing: $1"
}

run_contracts() {
  require_dir "$CLI_ROOT"
  require_dir "$SERVER_ROOT"
  require_dir "$NODE_ROOT"

  command -v cargo >/dev/null 2>&1 || die "cargo is required"
  command -v go >/dev/null 2>&1 || die "go is required"
  if command -v uv >/dev/null 2>&1; then
    (cd "$SERVER_ROOT" && uv run pytest -q tests/test_session_attachment_transfer.py)
  elif command -v pytest >/dev/null 2>&1; then
    die "uv is required to resolve the Server's pinned Python environment; refusing a system pytest"
  else
    die "uv is required for Server contracts"
  fi
  (cd "$NODE_ROOT" && go test ./cmd/agent-remote-attach ./internal/claudeattachments)
  # Run the non-ignored contract target, rather than merely compiling it.
  # The real upload test is explicitly ignored and is invoked by --live only
  # after exact session/device identities and an authenticated home are set.
  (cd "$CLI_ROOT" && cargo test --test exact_attachment_live_contract)
  (cd "$ROOT" && go test ./tests/remote ./tests/cliadapter)
}

case "${1:-}" in
  --contracts)
    run_contracts
    echo "exact-attachment-acceptance: contract fixtures passed; real SSH evidence remains pending"
    ;;
  --live)
    run_contracts
    command -v ssh >/dev/null 2>&1 || die "ssh is required for live forced-command acceptance"
    [[ -n "${AGENT_REMOTE_HOME:-}" ]] || die "AGENT_REMOTE_HOME is required in --live mode"
    [[ -n "${AGENT_REMOTE_EXACT_ATTACHMENT_TEST_SESSION:-}" ]] || die "AGENT_REMOTE_EXACT_ATTACHMENT_TEST_SESSION is required"
    [[ -n "${AGENT_REMOTE_EXACT_ATTACHMENT_TEST_DEVICE:-}" ]] || die "AGENT_REMOTE_EXACT_ATTACHMENT_TEST_DEVICE is required"
    (cd "$CLI_ROOT" && cargo test --test exact_attachment_live_contract -- --ignored --nocapture)
    echo "exact-attachment-acceptance: live Server/Node/SSH forced-command transfer passed"
    ;;
  -h|--help)
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
