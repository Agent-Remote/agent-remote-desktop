# agent-remote-desktop

English | [中文](README.zh-CN.md)

`agent-remote-desktop` is a native Go/mygo workbench for local Codex and Claude terminals and remote `fclaude` sessions. It shares the existing Rust CLI's configuration, credentials and structured operation contracts.

## Current state

The Zen workspace, Agent/project layers, Control Center, local session manager, managed terminals, persistence, i18n and schema-driven CLI entry points are implemented. The UI redesign R0–R6 is complete in the current implementation record.

This is still a development build. A formal release requires real Windows amd64 and Linux amd64 host evidence, Linux X11 and Wayland coverage, protected WireGuard broker evidence, live Server/Node/SSH/fclaude integration, complete Agent TUI/IME/attachment checks and end-to-end performance measurements. FreeBSD is not planned.

## Architecture

- Go and native mygo own the window, UI, i18n, themes and lifecycle.
- `terminalhost` owns platform PTY/ConPTY sessions and terminal rendering.
- `session` owns local Agent processes independently of visible panes.
- `remote` and `cliadapter` call the Rust CLI through versioned structured contracts.
- SQLite stores desktop-only layout, theme, project and session metadata.
- CLI credentials, server state, account state and remote operation journals remain owned by the Rust CLI.

All desktop-owned text uses mygo's official i18n catalogs. All tests, helpers, fixtures and test executables live under the repository's root `tests/` directory.

## Run

Go 1.27.1 or newer is required:

```sh
go run .
go run . --home /path/to/agent-remote-home
```

Install the external tools you want to use (`agent-remote`, `fclaude`, `codex` and/or `claude`). Open a project, create a local or remote session, and use the Agent drawer, project wheel or Control Center to navigate. Closing the window hides it and keeps managed sessions alive; explicit quit asks whether local sessions should stop.

## Install a candidate

Build a complete release directory, then verify and install it on the target
host. The bundled `agent-remote-desktop-installer` only installs files from a
verified directory into a user-writable destination; it does not download,
elevate, sign or install Agent CLIs.

```sh
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
scripts/release-native-smoke.sh dist/release
```

The installer supports `--status`, normal upgrade and `--rollback`. Run the
native smoke entry point on the target host; a cross-build is not installation
evidence.

## Checks

```sh
bash scripts/check.sh
go test ./...
go run ./tests/native-probe
go run -race ./tests/workbench-probe
go run ./tests/performance-probe -frames 120
```

For terminal changes, set `DESKTOP_TERMINAL_TESTS=1`. Native acceptance scripts must run on the target host; cross-compilation is not runtime evidence. Remote contract fixtures and host-only preflight do not prove a live Server/Node session.

The long headless `tests/app` package runs as four serial shards on isolated CI runners for each target OS. Native runtime jobs run the remaining Go packages and host acceptance checks. A local `go test ./...` still runs the complete package in one process.

## Release

Formal targets are `darwin/arm64`, `windows/amd64` and `linux/amd64`. macOS uses the project's community self-signing identity. Windows/Linux do not require publisher code signing, and Apple Developer ID/notarization are not prerequisites. FreeBSD is not planned.

Build and verify a candidate with:

```sh
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
```

The release workflow may publish only after native evidence, broker evidence, remote integration and performance gates are complete. The workflow itself must not be treated as those gates.

Prepare a component release from `main` with the repository-owned version and changelog files:

```sh
scripts/prepare-release.sh 0.1.0
git diff -- mygo.json CHANGELOG.md
```

The protected workflow performs the same preparation and complete checks, then commits the release source and pushes the matching `v*` tag. The tag starts the release workflow; review every open gate in `docs/implementation-status.md` before running it.

## Acceptance and release scripts

The repository keeps separate contract, host and artifact entry points:

```sh
# Build and verify a candidate artifact directory.
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
# Prepare repository-owned version and changelog files.
scripts/prepare-release.sh <version>
scripts/update-changelog.sh <version>
# Native host acceptance and packaged smoke.
scripts/native-acceptance.sh
scripts/release-native-smoke.sh dist/release
# Remote, attachment and synchronization contracts.
scripts/remote-interop-acceptance.sh --contracts
scripts/remote-interop-acceptance.sh --host-only
scripts/exact-attachment-acceptance.sh --contracts
scripts/sync-chain-acceptance.sh
# Refresh the generated CLI coverage catalog.
go run ./cmd/command-catalog > /tmp/desktop-catalog.json
python3 scripts/update-cli-coverage.py /tmp/desktop-catalog.json
# Join host evidence after the target hosts have produced records.
scripts/verify-native-evidence.sh <macos-dir> <windows-dir> <linux-dir>
```

On Windows, run the PowerShell equivalents from a target host:

```powershell
pwsh -File scripts/native-acceptance.ps1
pwsh -File scripts/release-native-smoke.ps1 -ReleaseDir dist/release
```

`verify-native-evidence.sh` is the final evidence joiner; it does not create
host evidence. The acceptance scripts use temporary homes and disposable
fixtures. Remove their evidence directories and test workspaces after review.

## Documentation

- Product plan: `../agent-remote/docs/agent-remote-desktop-design.zh-CN.md`
- Implementation status: `docs/implementation-status.md`
- CLI coverage: `docs/cli-coverage.md`
- Platform validation: `docs/platform-validation.md`
- Development rules: `docs/development-standards.md`
- Release build: `docs/release-build.md`
- Remote interop: `docs/remote-interop-acceptance.md`
- WireGuard contracts: `docs/wireguard-*.md`
- Session contracts: `docs/remote-session-*.md`, `docs/desktop-ego-browser-requests.md`

The implementation-status file records current evidence. Older entries are historical snapshots and do not override the latest release gate summary.

## License

GPL-3.0-only. See `LICENSE` and `THIRD_PARTY_NOTICES.md`.
