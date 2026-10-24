# Platform validation

Updated 2026-10-24. 正式版目标为 Windows、macOS、Linux，FreeBSD 不在计划中。
macOS 社区自签；Windows/Linux 不要求发行者代码签名。正式 artifact 目标为
`darwin/arm64`, `windows/amd64` and `linux/amd64`. Cross-builds,
fixtures, ARM64 containers and Xvfb are useful development evidence, but they do
not satisfy a native release gate.

## Current evidence

| Area | macOS arm64 | Windows amd64 | Linux amd64 |
| --- | --- | --- | --- |
| Go build and tests | Passed on the development host | Cross-build passed; ARM64 VM probes passed; amd64 native run pending | Cross-build passed; native run pending |
| Native window, PTY/ConPTY, tray and workbench | Native probes passed with disposable homes | ARM64 VM platform/installer/tray/ConPTY passed; amd64 workbench and full host run pending | Pending real host run |
| Installer install/upgrade/rollback | Host smoke passed | Pending real host run | Pending real host run |
| IME, Agent TUI, attachments and input latency | Partial host checks; full evidence pending | Pending | Pending |
| WireGuard protected broker | Not run | Pending | Pending |
| Disposable Server/Node/SSH/Mutagen sync chain and restart recovery | Passed in Linux arm64 Docker controller/target; not a native amd64 desktop gate | Pending | Pending |
| Live Server/Node/SSH/fclaude session and exact attachment | Not run in the desktop release evidence | Pending | Pending |

The Go/mygo performance probe reports view-build P95 for a bounded 512-sample
window and a 16 ms budget. It does not measure GPU compositing, PTY rendering,
window scheduling or end-to-end input feedback. Release evidence must measure ten
active sessions and four visible panes on each target host.

## Native acceptance

Run the acceptance entry point on the host being measured. It never sets
`GOOS` or `GOARCH` and writes `cross_compilation=false` to the evidence:

```sh
DESKTOP_TERMINAL_TESTS=1 bash scripts/check.sh
scripts/native-acceptance.sh --target darwin
```

On Windows without Git Bash:

```powershell
pwsh -File scripts/native-acceptance.ps1 -Target windows -EvidenceDir "$env:TEMP\agent-remote-desktop-native"
```

The probes cover platform requirements, installer behavior, a native window and
managed PTY, tray creation/destruction, and the workbench flow. A Linux run must
record its display protocol. For release, provide separate real-host evidence
for X11 and Wayland; Xvfb may be retained as a development record only.

## Release verification

After building a candidate, verify its exact target set and then run the
installer smoke on every target host:

```sh
scripts/build-release.sh --out dist/release --clean
scripts/verify-release.sh dist/release --require-targets darwin/arm64,windows/amd64,linux/amd64
scripts/release-native-smoke.sh dist/release
```

Use `scripts/verify-native-evidence.sh` to combine host records. Add
`--require-linux-display-pair` for the final Linux X11/Wayland gate and
`--require-wireguard` when the protected broker is part of the composition.
The verifier rejects missing probe reports, cross-build records, skipped
workbench/tray probes and mismatched target metadata.

## Required service and integration evidence

Before publishing, record all of the following against the exact component
commits and disposable credentials:

- Protected WireGuard broker installation, caller authorization, replay
  rejection, tunnel liveness and restart recovery on all three targets.
- Live Server/Node/SSH/tmux/fclaude create, attach, stop, delete and exact
  attachment transfer, including cleanup receipts.
- Mutagen pause/resume, conflict handling and daemon restart recovery (passed
  in the disposable Docker sync-chain run; rebind the record to the final
  release composition).
- Agent TUI, IME, file/image attachment, tray and background-lifecycle checks.
- GPU/frame-rate and end-to-end input feedback measurements.

The Linux ARM64 Docker smoke did not satisfy the amd64 X11/Wayland gate. The Windows ARM64 VM platform, installer, tray and ConPTY probes passed, but the full native acceptance/workbench run was blocked by the VM's temporary execution/elevation policy; it is not Windows amd64 release evidence. Keep evidence outside the source tree. 测试辅助代码和夹具统一位于仓库根 `tests/`，不要把测试文件与生产代码混放。Remove temporary homes, containers,
sockets, credentials, workspaces and logs after each test run.

A disposable `linux/amd64` Docker+Xvfb attempt passed the platform and
installer probes, but did not complete the native window, tray and workbench
probes because the container runtime dependencies were unavailable. It remains
development evidence and does not satisfy the real X11/Wayland host gate.

Related documents: [`implementation-status.md`](implementation-status.md),
[`release-build.md`](release-build.md), [`remote-interop-acceptance.md`](remote-interop-acceptance.md),
and the product plan in `../../agent-remote/docs/agent-remote-desktop-design.zh-CN.md`.
