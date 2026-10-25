# Implementation status

Updated 2026-10-25. This file describes the current source tree and release
evidence. It is the status source for the desktop README; historical work logs
are intentionally not kept here.

## Implemented

- The native Go/mygo workbench implements the Zen workspace, Agent and project
  layers, Control Center, project slots, local session manager, managed PTY or
  ConPTY terminals, split panes, session history, persistence and tray
  lifecycle.
- The redesign R0–R6 is implemented: daybreak, midnight and sakura themes;
  reduced motion and transparency; keyboard chords; drag-linked sheets and
  drawers; HUD command search; responsive 900×600 layouts; and English,
  Simplified Chinese and pseudo-localized UI checks.
- Desktop-owned text uses the generated mygo i18n catalog. CLI output, Agent
  output, paths, user content, credentials and protocol identifiers remain in
  their owning boundaries.
- The desktop uses the Rust CLI through versioned structured contracts. SQLite
  stores only desktop layout, project, theme and public session metadata. CLI
  credentials, server state and operation journals remain owned by the CLI.
- Local Codex and Claude sessions, remote `fclaude` listing/creation/attach,
  account lifecycle, password and device-code login, device registration,
  configuration import, runtime recovery, diagnostics, local setup, Skills,
  node and ego-browser workflows, synchronization, Mutagen environment
  recovery, WireGuard inspection/configuration/tunnel plans, port forwarding,
  attachments and operation summaries have dedicated contracts or reviewed
  terminal workflows. Mutations require an exact context and explicit user
  confirmation; uncertain results are retained and never replayed.
- The schema-driven command page covers the currently discovered leaf commands:
  `agent-remote` 106 command nodes and `fclaude` 12 command nodes in the pinned
  catalog. Sensitive fields use dedicated credential flows and are not placed
  in generic argv forms.
- Tests, helpers, fixtures and test executables are under the repository-root
  `tests/` directory. The check script rejects misplaced test files and stale
  generated catalog code.

## Evidence currently available

The following checks have passed on the current macOS arm64 development host,
using temporary homes and disposable fixtures:

```sh
bash scripts/check.sh
go test ./...
go test -race ./tests/release ./tests/remote ./tests/session ./tests/storage ./tests/theme
go vet ./...
go run ./tests/native-probe
go run ./tests/performance-probe -frames 120
```

The macOS platform, installer, native-window, tray and workbench probes pass
with disposable homes. The performance probe measures Go/mygo view construction only
(16 ms view-build budget); it does not measure GPU compositing, PTY rendering
or end-to-end input latency. The remote contract and host-only checks use
fixtures or report `remote_execution.status=not_run`; they do not prove a live
Server/Node/SSH/fclaude session.

The Parallels Windows 11 ARM64 VM passed the platform, installer, tray and
native ConPTY probes with disposable binaries. The Windows probe now uses
delayed `cmd.exe` expansion so the echoed input is checked on real ConPTY.
This is useful ARM64 development evidence only; it does not satisfy the
formal Windows amd64 host gate or the workbench/IME/upgrade evidence.

The disposable Docker sync-chain acceptance passed on 2026-10-24 using the
current Server, Node, CLI and Mutagen v0.18.1 source. It covered real Linux
sshd, worker/helper UID and ACL checks, bidirectional transfer, pause/resume,
conflict preservation, explicit resolve/reset, stale and revoked SSH-key
rejection, and Node/Mutagen restart recovery. This evidence covers the sync
chain only; it does not close the separate exact attachment or full
Server/Node/SSH/tmux/fclaude session gate.

Cross-build checks for `darwin/arm64`, `windows/amd64` and `linux/amd64` pass.
Cross-compilation is build evidence only and is never counted as native runtime
evidence.

The final hosted desktop CI run `40201235800` passed static checks, all three
native runtime jobs, all twelve isolated `tests/app` shards, the three
cross-builds and the native evidence verifier. This validates the repository
workflow and hosted acceptance suite; it does not close the real-host release
gates below.

An additional disposable `linux/amd64` Docker+Xvfb attempt passed the platform
and installer probes. Its native window, tray and workbench probes were not
completed because the container runtime dependencies were unavailable; this is
development evidence only and does not close the Linux host gate.

## Release gates still open

The repository is not a formal release yet. The following evidence is still
required for the exact release composition:

- Run the desktop on a real `windows/amd64` host, including Win32 windowing,
  ConPTY, IME, tray, installer upgrade/rollback and the workbench probe.
- Run on real `linux/amd64` hosts with both X11 and Wayland, including GTK,
  AppIndicator, PTY, tray and installer behavior. Xvfb and an ARM64 container
  are useful checks but do not satisfy this gate.
- Install and exercise the protected WireGuard broker on all three target
  platforms, including caller authorization, replay rejection, tunnel
  liveness and restart recovery.
- Complete a disposable live Server/Node/SSH/tmux/fclaude session, exact
  attachment transfer and cleanup receipt. The disposable Mutagen sync-chain
  and restart-recovery gate is already passed, but it must still be included
  in the final release composition review.
- Measure real Agent TUI, IME, attachment and input feedback behavior, plus
  GPU/frame-rate performance with ten active sessions and four visible panes.
- Verify the exact release artifacts and installer on each target host. The
  community policy is macOS project self-signing; Windows/Linux publisher code
  signing is not required, and Apple Developer ID or notarization is not a
  prerequisite.

Do not publish until every gate above has a passing, reviewable record. The
release workflow and `scripts/verify-release.sh` validate artifacts and policy;
they do not manufacture missing host or service evidence.

## Required commands

```sh
# Development checks
bash scripts/check.sh
DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/session ./tests/terminalhost ./tests/storage ./tests/app

# Host-only probes (run on the target host)
scripts/native-acceptance.sh
pwsh -File scripts/native-acceptance.ps1

# Candidate artifacts; publishing is a separate user decision
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
```

Every acceptance run must use a temporary CLI home, disposable credentials,
and disposable server/node data. Remove temporary containers, homes, sockets,
evidence directories and test workspaces after the run. Never use a production
credential or a real user workspace.

## Related contracts

- Product plan: `../../agent-remote/docs/agent-remote-desktop-design.zh-CN.md`
- CLI coverage: `cli-coverage.md`
- Platform matrix: `platform-validation.md`
- Development rules: `development-standards.md`
- Release build: `release-build.md`
- Remote interop: `remote-interop-acceptance.md`
- WireGuard contracts: `wireguard-*.md`
- Session contracts: `remote-session-*.md`
