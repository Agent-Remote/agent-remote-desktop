# agent-remote-desktop

The formal release will be a community build: macOS uses the project's community
self-signing identity; Windows/Linux do not require publisher code signing.
Apple Developer ID and notarization are not release prerequisites. The packaging
foundation is available through [`scripts/build-release.sh`](scripts/build-release.sh);
full installation, protected component and update validation remain platform
acceptance tasks. The formal release targets Windows, macOS and Linux; FreeBSD is
not planned. See the [community release build guide](docs/release-build.md).

Tagged publication is handled by the repository's [community release workflow](.github/workflows/release.yml).
It requires a `v*` tag matching `mygo.json`, imports the protected project
community certificate only on a temporary macOS keychain, verifies the complete
archive offline, and publishes after verification. Windows/Linux publisher
signing and Apple notarization are not enabled by that workflow. Native host
acceptance remains a separate release gate.
The workflow passes and verifies the exact first-release target set
`darwin/arm64,windows/amd64,linux/amd64`; local smoke builds may use a subset.

English | [中文](README.zh-CN.md)

A native Go multi-Agent workbench built with mygo. It manages local Codex and
Claude terminals alongside remote fclaude sessions, with projects and split panes.

This is an implementation in progress, not a production release. Schema-driven
generic forms now cover discovered leaf commands: users edit parameters, review
and freeze argv, then run the command in a managed terminal. Dedicated business
workflows, complete illustrated theme-motion acceptance, distribution, and
Windows/Linux runtime validation remain on the implementation plan.

The workbench uses the redesign's token set (prototype-v3 colours): Light,
Dark and Kawaii, stored as `daybreak`, `midnight` and `sakura`. New installs
follow the system appearance; a state saved by an earlier build keeps its
explicit theme. Choosing a theme in the Control Center or Settings makes the
choice explicit. Terminal panes use an opaque background per theme with
contrast-checked cursor and selection colours; ANSI colours are not re-tinted.
Settings → Appearance also offers Reduce transparency (solid surfaces, no blur;
the system's increase-contrast setting implies it). Reduce motion makes
movement instant and keeps short cross-fades, alongside mygo's native Reduce
Motion preference. The earlier Sakura portrait has been removed.
`App.MotionStats` retains a bounded 512-frame view-build sample window and
reports nearest-rank P95; `go run ./tests/performance-probe -frames 120`
prints a host-only JSON reading for a ten-session, four-pane persisted layout.
This probe covers Go/mygo view construction only. Native GPU, PTY rendering
and input-feedback measurements still require platform host evidence.

Account management now includes create, status, bind, verify and disable flows with exact-context confirmation. Binding login uses a native terminal for the confirmed account and binding attempt; saved binding history refreshes status before a new confirmation. Request acceptance does not mean login completion. Disable revokes port-forward grants but does not erase credentials or stop existing sessions. See the [account protocol](../agent-remote-cli/docs/desktop-accounts.md).

Administrator runtime recovery now has a user-only authority check, exact migration inspection, and separate confirmation for target verification, source verification or source permission repair. The CLI owns retained request keys; the desktop reloads them after restart, queries the original binding, and removes only explicitly rejected or confirmed terminal local records. Older manually entered keys remain query-only. Inspection and acceptance do not imply completed recovery. See the [runtime recovery contract](../agent-remote-cli/docs/desktop-runtime-recovery.md). Native Windows/Linux desktop acceptance remains pending.

Account configuration import now has separate local discovery and file-review actions, an optional explicitly confirmed Server policy check, and a separate upload confirmation. The full manifest is paginated without truncation; history and skills options follow CLI defaults. File contents stay in Rust. The CLI retains the original request key and public context before uploading; **Configuration import history** automatically reloads those records from the CLI journal when opened after restart and queries only the original request and exact task. Unknown results are never resubmitted automatically. Removing a settled local record requires its own confirmation. See the [import protocol](../agent-remote-cli/docs/desktop-account-import.md). Native Windows/Linux and deployed Server/Node acceptance remain pending.

Project synchronization now has a selected-project page for `sync ensure/status/pause/resume/resolve/reset`. Every change reviews the exact shared context and destination fingerprint; Server and local Mutagen observations remain distinct from Agent state. Partial and unknown phase receipts survive navigation and are never replayed automatically. **Skip local execution** retains CLI dry-run semantics: Server and shared SQLite changes still occur. Resolve flushes deliberately edited files; it does not choose an overwrite winner. Reset rebuilds the baseline and may restore deleted files. See the [sync protocol](../agent-remote-cli/docs/desktop-sync.md). Old mappings now have a read-only recovery preview: a verified Server 404, complete local mapping and exact Mutagen identity must be reviewed before removal. Files and remote resources remain intact; a fresh relationship requires a separate review, with earlier results retained. Real Server/Node/SSH/Mutagen file transfer has passed in an isolated macOS-to-Docker Linux chain. A separate synchronization environment page now reviews the complete shared Mutagen service before a confirmed restart. Windows/Linux native desktop acceptance remains pending.
The operation center also restores a bounded stage summary for synchronization and Mutagen environment work after a desktop restart; exact receipts remain in the CLI journal and are never reconstructed or replayed by the desktop.

The **Synchronization environment** page is available from Diagnostics, project synchronization and command search, without a project or Server login. It shows the last recorded and desired CLI/SSH environments and the complete synchronization/forwarding inventory, including paused sessions and stopped-daemon data. A separate confirmation restarts the shared service; previous receipts remain available and uncertain results are never replayed. External service registrations are reported as blocked pending dedicated service integration. See the [environment contract](../agent-remote-cli/docs/desktop-mutagen-environment.md).

Remote session cards also support an exact `fclaude stop` flow when the CLI
advertises `sessions.stop`: the confirmation freezes the session, node, runtime
backend and configuration context. The CLI owns the caller-key journal; the
desktop queries only that original key after an uncertain result. Process stop,
Skill save/publication and synchronization observations remain separate, so a
stopped process is not presented as a saved project.

The Skills page provides a `skills.list` workflow using the CLI's user session,
plus a reviewed enable/disable flow bound to the catalog generation and exact
configuration context. It projects only safe catalog and account-local status
fields, supports refresh and name/ID filtering, and rejects stale context or
ambiguous scope. Skill content, source metadata, and credentials remain inside
the CLI. The page also provides the read-only `skills.state.conflicts` check for
an exact account UUID; publication and migration conflict metadata is paginated
and remains inside the CLI boundary apart from its safe status projection. Each listed conflict now offers a dedicated review entry: publication conflicts can be resolved, while migration conflicts can be resolved, migrated or restored. The desktop only pre-fills safe conflict/Skill/account identifiers; branch, path and checkpoint choices remain user-entered and are validated by the CLI.

The same page provides `skills.state.list` history for an account directory or
one Skill. It uses a versioned read-only contract to page checkpoint and pending
finalization metadata, validating the account UUID, scope, limits and cursors
against the current context. Paths, manifests, saved inputs and credentials stay
inside the CLI.

The same page provides a dedicated `skill info` details panel through the
`skills.info` v1 read-only contract. It binds the exact Skill identifier and
optional tool/account scope to the current home/server/device context, and only
projects safe metadata such as retained revision IDs, effective rules and model
state. Source locators, tracking, paths, content and credentials remain inside
the CLI; stale context or unknown fields fail closed.

The ego-browser page can inspect active requests for each binding through
`ego_browser.requests`. Cancellation requires the user credential, an exact
request confirmation and the reviewed binding/request generation and sequence;
the CLI refreshes the binding before sending the cancellation. The desktop only
shows bounded metadata and payload sizes, and never receives browser payloads.

The same page provides structured binding lifecycle controls for `pause`, `resume`, `stop` and `revoke`. It first reads an exact generation-bound plan with the user credential, then requires explicit confirmation before `ego_browser.binding.control`; stale generations, unsupported states and unknown receipts fail closed without retry.

Revoked devices and stopped, expired, failed or revoked bindings also expose
dedicated `delete-device` and `delete-binding` review flows. The desktop freezes
the exact target and generation in `ego_browser.delete.plan`, requires explicit
confirmation for control, and preserves unknown outcomes when context,
generation, fields or receipts do not match. Local indexes are cleaned only
after a successful remote receipt, whose projection reports whether local
cleanup completed.

The Device page provides reviewed `device.status`, `device.revoke` and
`device.rotate-token` workflows. It freezes the home/server/device context,
uses the appropriate user or device credential, and reports remote success
separately from local cleanup or credential persistence. Device tokens never
enter the desktop process.

`node install` and the `ego-browser` command family use dedicated native
terminal workflows. The desktop gates each exact path by CLI capability and
platform, freezes the reviewed argv, requires `--yes` for mutations, and keeps
the terminal output in the managed PTY. The operation center exposes only safe
running/completed/failed status projections.

The operation center aggregates safe lifecycle summaries for login, accounts,
synchronization, recovery, network actions, remote sessions and background
queries. These non-sensitive summaries survive an application restart and link
back to the owning page without exposing caller keys, paths, argv or
credentials. Exact request keys, receipts and execution context remain in the
CLI journal; the desktop never replays an operation automatically.

## Run

Use Go 1.27.1 or newer:

```sh
go run .
# Isolated configuration, following the existing CLI's home semantics:
go run . --home /path/to/agent-remote-home
```

Install the Agent tools you want to run. Open a project, create a session, and
split it horizontally or vertically. Hiding a pane or closing the window keeps
the application and its processes alive. Explicit quit asks to stop local
sessions; an unresponsive process requires a separate force decision. Remote
tmux sessions survive detaching the desktop terminal.

Files dragged from Finder, Explorer or the desktop are inserted as shell-quoted
paths into the focused local Agent terminal and are never executed by the
desktop. Remote fclaude panes currently report that local file drops require an
identity-bound upload workflow; they do not receive misleading local paths.

Desktop preferences live in `desktop/desktop.sqlite3` under the selected CLI
home. The desktop never directly edits the CLI database or copies credentials.
Advanced launch arguments currently use a JSON string array and are passed as
argv, without shell interpolation.

Release artifacts include an `agent-remote-desktop-installer` helper. After
obtaining a complete release directory, it verifies the target manifest and
SHA-256 checksums before atomically installing or upgrading a user-writable
location; `--rollback` restores the previous version. It never downloads,
elevates, signs, or installs external Agent CLIs. See the [release build
guide](docs/release-build.md). The offline
[`scripts/verify-release.sh`](scripts/verify-release.sh) also rechecks the
artifact set, binds the manifest to `Agent Remote Desktop` and
`dev.agent-remote.desktop`, and requires each target's desktop executable and installer helper,
and enforces the community signing policy before distribution.
The bundled installer repeats that identity check on hosts without Bash.
Use the helper's read-only `--status` with the destination to inspect the
installed version and rollback availability before presenting an upgrade.
Linux Debian packages additionally declare an AppIndicator runtime alternative
(`libayatana-appindicator3-1 | libappindicator3-1`) for the tray entry that
keeps the hidden workbench reachable; the native platform and tray probes still
verify the installed desktop at runtime.

## Workspaces and recovery

The workbench is a full-window Zen workspace: one to four terminal panes and a
breathing bar at the bottom, with no sidebar, toolbar or tab strip. Everything
else is a layer over it (redesign doc §3):

- **⌥A / Alt+A — Agents dock.** A card per session of the project: state
  (new output, idle, connected, not connected, exited — facts the PTY shows,
  never "thinking"), the last lines of output, split position and last activity.
  Hold ⌥ and press A again to cycle, release ⌥ to switch; a quick tap keeps it
  open for the arrows and Enter. Drag a card onto another to reorder tabs.
- **⌥C — New Agent wheel.** Claude and Codex start at once in the project;
  fclaude and **Advanced…** open the full new-session dialog (name, arguments,
  history, review). Hold Shift when releasing to split beside the focused pane.
- **⌥X — Project wheel.** "··· More" (search, or open a folder) and seven
  stable slots: a project keeps its sector until it is the least recently
  opened when an eighth arrives. The HUD shows path, Git branch (read from
  `.git/HEAD`, never by running git) and language.
- **⌥S, click the breathing bar, or pull it up — Control Center.** Every
  management page, grouped by project, remote, connections and system, with
  in-progress operations first. Wide tiles show only observed state ("Not
  checked" until read). The header holds the sign-in state as last read, a
  search over pages, commands, projects and sessions (the palette's index),
  and the appearance switch. A page pushes into the sheet from the right with
  Back, its title, Return to workbench and a context strip (CLI home; server
  and device on Remote management); Esc goes back, then closes. Pages opened
  from anywhere else (palette, menu, links) open there too. Drag the sheet
  down, or push the dock up, to close either.

Chords are taken by the focused terminal before it encodes the key, so the
Agent receives nothing (R0 evidence: a window shortcut alone leaks `å` on macOS
and enters the system menu on Windows). Escape leaves any layer. Hiding a pane
keeps its session in the dock.

The desktop saves tab order, split ratios, pane focus, project wheel slots and
session metadata. After an application restart, saved panes are shown as disconnected.
Choose **Continue a saved session** to open the Codex/Claude history picker or
provide a specific session ID. Remote recovery refreshes the current server and
session list, then confirms the saved home/server/device and exact running session.
Older history without this context requires selecting a session again; a saved UUID
is never silently attached to the current server. Successful recovery preserves the
original project, working directory and split position. Local recovery uses the
tool's current configuration. Desktop history stores public remote context, but never
original launch arguments, environment variables, credentials or terminal output.

New-session and recovery dialogs check whether the selected Codex, Claude or fclaude
tool is available. Missing tools disable **Start session** or **Continue**, with an
explanation, installation instructions and an explicit link to the official guide.
Use **Refresh tool detection** after installing a tool; settings also provides
installation guidance. Detection uses a cached snapshot and does not run version
commands on redraw. Launch checks the executable again, so a tool removed after
detection cannot start from a stale availability result. Codex/Claude recovery
continues to use their native history/resume commands; fclaude recovery always
requires a reviewed existing session and its exact remote context.

The desktop database reads versions 1/2/3 and writes version 3 with validated,
independently cloned public remote context. Older apps cannot overwrite version 3.
Incompatible future versions and invalid layout references are rejected while
preserving the existing data. Removing a disconnected history entry only removes
desktop metadata; it does not delete the Agent's own history or a remote session.

Use Command on macOS and Control elsewhere for `Mod` below:

| Action | Shortcut |
| --- | --- |
| Agents / New Agent / Project / Control Center | `⌥A` / `⌥C` / `⌥X` / `⌥S` (Alt on Windows/Linux) |
| Commands and navigation | `Mod+Shift+P` |
| Project search / New Agent wheel | `Mod+O` / `Mod+T` |
| Horizontal / Vertical split | `Mod+Shift+D` / `Mod+Shift+E` |
| Next / Previous tab | `Ctrl+Tab` / `Ctrl+Shift+Tab` |
| Next / Previous project | `Mod+Alt+Right` / `Mod+Alt+Left` |
| Focus next pane | `F6` |
| Hide active pane | `Mod+Shift+W` |
| Focus mode / Settings | `Mod+Shift+F` / `Mod+,` |
| Remote management | `Mod+Shift+R` |

mygo's pinned terminal library is downloaded and checksum-verified during local
development. Packaged applications will include it. macOS requires 13+; Linux
requires GTK 3 (and AppIndicator for tray support), while WebKitGTK is needed
only for page/WebView windows; Windows uses native ConPTY, while WebView2 is
needed only for page/WebView windows.

## Development requirements

Remote management requires an `agent-remote` build with the version-1
`--desktop-control` protocol on PATH. It reads the selected CLI configuration,
lists tool accounts and remote sessions, and supports server-side Agent/status
filters, text search over loaded results, refresh and query cancellation.
Opening this page or focusing its window refreshes the data.
Choose a local project before connecting a session; the confirmation identifies
the full session UUID, server, device, home, local project/directory, account and
node, and explains terminal takeover. The CLI must advertise `sessions.attach` v1;
the desktop passes paired `--expected-server`/`--expected-device` arguments without
a fallback to legacy attachment. Live reuse requires the same home, server, device,
session and local project. The local directory is the terminal working directory,
not proof of remote workspace identity. Text copy/paste remains available; image
and file paste are not supported in this exact connection path yet. See the
[attachment and history design](docs/remote-session-attach.md).

The capability catalog routes `agent-remote node install` to a dedicated Node
picker. It loads a bounded `nodes.list` projection, freezes the selected
home/server/device, Node UUID and optional ego-browser intent, then opens an
exact review before the managed Agent Remote terminal performs SSH, release
verification, join-code exchange and installation. Desktop never stores the
join code or credentials; restart recovery keeps only the safe operation
summary and never replays the install automatically.

The capability catalog routes fclaude's default invocation, `run` and `list` to
this remote session inventory. `new` opens the structured caller-key creation
review, with account, workspace, project and synchronization evidence bound to
the current home/server/device context. The workbench still provides a managed
native fclaude terminal for Claude argument passthrough and first-time remote
authorization; terminal output is never treated as a desktop receipt.

The same remote page can request an exact stop for a running Claude session.
It shows `accepted`, `stopping`, `pending`, `unknown`, `saved`, `failed` and `deleted`
outcomes from `sessions.stop`/`sessions.stop.request` without submitting a second
stop. `stop-status` remains an owner-scoped query of the original caller key.

Caller-key remote creation is also queryable after a desktop restart. Once the
CLI returns the original request UUID, desktop persists only that UUID and its
home/server/device scope; the recovery dialog is query-only, freezes the current
context, and calls `sessions.create.request` for the original key. It never
stores the receipt, argv, synchronization plan or credentials and never submits
a replacement create request.

**Default account** reads, selects or clears the shared CLI default for each tool.
Review the configuration directory, server/device, previous default and new choice
before confirming. Concurrent CLI changes require a refresh instead of being
overwritten. Defaults belong to the selected home and tool and are shared across
servers. Reading and clearing remain available without login; clearing does not
disable or delete an account. Submitted results survive page navigation, but not
application restart, and uncertain outcomes are never automatically retried. See
the [account-default contract](../agent-remote-cli/docs/desktop-account-defaults.md).

**Developer credentials** has a secret-free inventory plus reviewed create, bind
and unbind actions. The desktop submits profile metadata and exact IDs through
the CLI control channel; device tokens stay in the CLI secure store. Unknown
fields such as tokens and private keys are rejected before network access, and
the receipt projects metadata only. Generic CLI forms never expose these actions.

Queries reuse CLI device credentials; the desktop never receives tokens. Errors
are translated from structured codes. **Sign in to server** accepts a password
and optional MFA code, device name, SSH public key file and optional WireGuard
public key. Review the exact details before confirming; user-only login can skip
device registration. The desktop clears password/MFA input after submission and
never stores it. Stopping the local wait can leave the remote result unknown;
the application reports that state and does not retry automatically. Device
registration uses a CLI-owned UUID and Server idempotency record; a lost
registration response is recovered only with that original key, and the key is
cleared only after local credential publication succeeds.

**Sign out** reviews the shared CLI home, server and device before confirmation.
Remote revocation is selected by default; deselect it for offline local-only cleanup.
The result separates local credential cleanup from remote user/device revocation,
including partial failures. Stopping the wait leaves the result unknown and never
retries automatically. Logout affects the CLI's shared login; it does not stop local
Agents or send a stop request to remote sessions. Existing connections may lose
authorization.

Choose browser approval on the sign-in page to authorize with a short code.
Review the target and device settings, then explicitly open the approval page or
copy the code. The CLI owns polling and credential storage; the desktop displays
only the user code, URL, remaining lifetime and final receipt. Cancellation stops
local waiting and may leave the result unknown. The CLI persists a caller-owned
completion key and, after a transport or timeout failure, queries only that exact
key; missing, expired or invalid receipts remain unknown and are never replayed.

Sign-in and sign-out keep running when you navigate to another page. Return via
their entry in the Control Center's In progress group or command search. Results remain available
until you acknowledge them with **Back to remote management**; reopening a status
page does not submit the operation again. The operation center restores only a
non-sensitive lifecycle summary after restart; exact request keys, receipts and
context remain owned by the CLI journal.

**Diagnostics** inspects the selected CLI configuration and dependencies. The
`status`, `doctor`, `deps status`, and `ssh check` commands now open strict
structured pages backed by versioned desktop-control methods. Their responses
are bound to the current home and platform; unknown fields or context drift fail
closed. Doctor and dependency repair require an explicit confirmation.
The Diagnostics page also opens a dedicated **Dependencies** page, which calls
`dependencies.inspect` and shows the CLI platform, manifest source, per-artifact
status/version result, and license. It never treats `credential: none` as an
administrator grant.
Opening
the page is passive; **Run checks** performs deeper local checks and optional server
health checks, while **Run version checks** explicitly runs bounded dependency
version commands. The page reports missing files, access failures, versions,
licenses and next actions. **Local initialization and repair** reviews the selected
configuration directory before creating missing directories, database tables and
the dependency manifest. Its eight-step receipt distinguishes existing data,
applied changes, failures and skipped prerequisites. Existing valid content is
preserved; damaged or newer database schemas and invalid manifests are not replaced.
This action does not sign in, install dependencies or start a tunnel. It also works
when the saved server configuration cannot be read.

Local repair runs in the background when you navigate away. Return through its
status entry or command search; dismissing the receipt refreshes diagnostics.
Stopping the wait or exiting may leave an unknown result and does not undo completed
steps. Inspect diagnostics before explicitly running repair again. The operation
center persists only a safe lifecycle summary; detailed receipts and exact
request context remain in the CLI journal, and the desktop never replays repair
automatically. Dependency installation remains in development; this is partial
P2 delivery.

**Complete initialization** is available from the Control Center and command palette. It
orders local setup, password or browser/device-code sign-in, optional device
registration, optional WireGuard publication, and final diagnostics plus account/
session readiness. WireGuard always pauses after its read-only plan for a separate
confirmation. The wizard uses the CLI desktop-control boundary for every step;
passwords, device-code polling credentials, access tokens and private keys never
enter the desktop UI or its receipts. A canceled or lost mutation remains an
unknown outcome and is never replayed automatically.

**WireGuard configuration** is available from Diagnostics and command search.
Choose the default location or an absolute output path, then review the server,
device and exact file creation/replacement before confirming. The CLI shares the
same configuration service with its normal command and optional `init` step.
The desktop receives only the plan and a receipt for key selection/storage,
registration and file publication; private keys remain with the CLI.

Partial results retain completed stages. A lost registration reply remains unknown
and is never automatically retried. Navigation preserves the operation, while
stopping the wait or quitting does not roll back saved keys or server changes.
Publishing a file does not start a tunnel. The network page now offers an explicit reviewed
tunnel action; Rust sends the user-side validated configuration bytes and digests through a fixed
platform broker client and returns success only after a broker receipt and second native
observation. It also exposes a read-only permission-bridge status query; without a protected
broker service the result stays unsupported/unavailable and never claims execution authority.
The macOS LaunchDaemon, Linux systemd/D-Bus/Polkit and Windows SCM LocalSystem broker
implementations and installation materials are in the repository. Persistent recovery,
real host authorization and Windows/macOS/Linux native release validation remain in development.

The desktop also derives a local lifecycle gate from the bridge facts in a fixed order: installation,
system approval, protocol compatibility, caller authentication and execution authorization. Only a
fully observed `ready` state is considered executable; intermediate or unknown observations remain
blocked and do not grant permission.

**WireGuard network** provides explicit configuration checks, runtime status and permission-bridge
status queries from Diagnostics or command search, without requiring login. Entering the
page does not run queries. Configuration checks return only a validated public
summary; unsupported hooks are identified by name and never executed. Status uses
one bounded machine-format query, strips private keys and preshared keys in Rust,
and distinguishes an empty successful observation from a failed query. It never
requests administrator access. Interface/peer pages use localized exact counters
and handshake times. Queries can be canceled independently; publishing configuration
invalidates their prior results. See the [inspection contract](docs/wireguard-inspection-contract.md).

**Tunnel plan** lets you choose a start/stop direction and configuration on the
network page, then explicitly preview configuration/tool snapshots, native
interface or service observations, and unmet prerequisites. Details expose object
identities; changing the action or configuration invalidates the old plan. Results
survive navigation and the query can be canceled. When a plan has no unmet prerequisites, the page enables an explicit Apply action. The CLI
rechecks the frozen plan, runs the packaged helper with platform permission handling, and
confirms the resulting target state before returning. See the [plan contract](docs/wireguard-tunnel-contract.md).

**Capabilities** reads the CLI's declared desktop operations, selected configuration
context and command catalogs for `agent-remote` and `fclaude`. It supports refresh,
query cancellation and command filtering without executing listed commands or
probing server permissions. Commands without a platform declaration are shown as
unknown, rather than assumed to work on every platform. Permission information
uses only already observed query results for the same configuration context;
other permissions remain unverified. This read-only inventory describes CLI
support and current prerequisites; it does not represent complete GUI coverage.

`agent-remote node install` and the `agent-remote ego-browser` command family
use a dedicated terminal workflow. The CLI advertises each exact command path
as a versioned terminal capability; the desktop checks that declaration and
the host platform, freezes the schema-generated argv, forces the CLI's
non-interactive `--yes` flag for mutating actions, and requires an explicit
review before starting a managed terminal. Final device/server authorization
and command errors remain owned by the CLI; the operation center shows safe
lifecycle summaries (running, completed, failed or needs attention) and restores
those summaries after restart without replaying commands.

All desktop-owned user-facing text must use mygo's official i18n catalogs and
generated Go functions, including menus, notifications, validation and errors.
The initial release languages are English (`en`, source/fallback) and Simplified
Chinese (`zh-Hans`), with system-language selection and a language setting.
Agent terminal output, user content and protocol identifiers remain unchanged.

All desktop tests, test-only helpers, fixtures and native test programs must live
under **this repository's root `tests/`**, separate from production code. Tests
use the root Go module and import production packages through their public APIs.

The current workbench ships both catalogs. Choose a language in Settings;
switching languages preserves project input and running terminals. Existing tests
and the native probe now live under `tests/`. See the
[development standards](docs/development-standards.md) for the target layout,
commands and required CI checks.

## Current checks

```sh
bash scripts/check.sh
DESKTOP_TERMINAL_TESTS=1 go test -race ./tests/session ./tests/terminalhost ./tests/storage ./tests/app
go run ./tests/native-probe
go run -race ./tests/workbench-probe
# Go/mygo view-build performance probe (host-only; no GPU/PTY claim)
go run ./tests/performance-probe -frames 120
scripts/native-acceptance.sh
# Protocol and remote-chain contracts (fixtures; no real remote claim)
scripts/remote-interop-acceptance.sh --contracts
# Host-only Go→Rust capability preflight (remote_execution=not_run)
scripts/remote-interop-acceptance.sh --host-only
```

The check script validates translations, generated Go accessors, test placement,
formatting, vet and tests. The native probe opens a real window, switches languages,
hides it, checks a harmless shell's PTY roundtrip, shows it again, and exits.
All probe files use temporary directories; its language override is not persisted.
The workbench probe uses a temporary test Agent with real PTYs and UI events under
the native app loop. It verifies focus routing, tab reordering, shutdown and
explicit history recovery, and confirmed remote attachment with the exact ID,
configuration directory and project. It verifies legacy and exact history
replacement within the original split, and reuses live connections only when
home/server/device/session/project all match. The probe never launches installed Agents or uses accounts.
The current full probe passes its remote-scope phase with the temporary account
binding and session fixtures; this still does not replace real server, Node or
Agent account validation.
For the explicit Server/Node/SSH and Mutagen boundary, see the
[remote interop acceptance](docs/remote-interop-acceptance.md). Its `--live`
mode requires an authenticated disposable session and never treats host-only
or fixture results as a real remote success.
For a real disposable synchronization chain, run
`scripts/remote-interop-acceptance.sh --sync-chain` with a reviewed Mutagen
v0.18.1 checkout, the CLI binary and adjacent SSH/SCP siblings. It exercises
the Server/Node/SSH/Mutagen Docker path and fails closed when prerequisites are
missing.
`native-acceptance.sh` also runs the temporary-directory installer probe
(install, upgrade, downgrade rejection and rollback) and records host metadata
and command output under `/tmp` by default. It never sets `GOOS` or `GOARCH`;
cross-compilation must be reported separately. On headless Linux it uses
`xvfb-run` when available and records that the display was virtual.
On Windows without Git Bash, use `pwsh -File scripts/native-acceptance.ps1`;
both entries preserve a `summary.txt` failure record naming the failed step.
If a coordinator has no target host yet, pass `--target windows|linux|darwin`
to the Bash entry (or `-Target` to PowerShell); it records `status=not_run`
and exits non-zero, and the evidence joiner rejects it. No cross-compiled
binary is executed.
For a built release directory, `scripts/release-native-smoke.sh dist/release`
also verifies, installs into a temporary user directory, checks installer
status, and launches the installed executable with `--version`.
On Windows without Git Bash, use
`pwsh -File scripts/release-native-smoke.ps1 -ReleaseDir dist/release` for the
same native packaged smoke; the shipped installer still performs offline
manifest/checksum validation when Bash is unavailable.
Pass `-AllowAdhoc` only when deliberately reviewing a macOS ad-hoc smoke
directory.
Once the three default hosts have produced evidence, run
`scripts/verify-native-evidence.sh <macos-dir> <windows-dir> <linux-dir>`;
it requires the formal `darwin/arm64`, `windows/amd64` and `linux/amd64`
targets, host-only macOS, Windows and Linux records plus all five probe logs,
including the real tray probe. Linux metadata must identify
`display_protocol=x11` or `display_protocol=wayland`. For the formal Linux
display gate, collect a second Linux host, pass `--require-linux-display-pair`
and provide separate X11 and Wayland evidence directories. The joiner also
parses each `platform.log` report
and rejects records without `host_runtime=true`, matching host target,
`supported=true` and passing required checks; Linux must report its required
display, GTK 3 and AppIndicator checks plus an optional WebKitGTK 4.1/4.0
observation, and Windows must report an optional WebView2 observation,
installer logs must contain structured install/upgrade/rollback/running-lock
results, tray logs must carry a host-only passed report, and native-window and
workbench logs must carry their probe success markers. Use
`--allow-virtual-display` only to review Linux Xvfb evidence; that mode is not
formal desktop proof.
The first platform probe step also allocates and closes the host's native PTY
(ConPTY on Windows) and records its initial size; this is a prerequisite check,
not proof of IME, full-screen TUI, or Agent-specific behavior.
Set `DESKTOP_LOCAL_AGENT_TESTS=1` to additionally run installed Agents' version
commands through the managed PTY; this is not a complete Agent TUI validation.

See [implementation status](docs/implementation-status.md),
[platform validation](docs/platform-validation.md),
[CLI coverage](docs/cli-coverage.md), and the
[product plan](../agent-remote/docs/agent-remote-desktop-design.zh-CN.md).

## License

GPL-3.0-only. See [third-party notices](THIRD_PARTY_NOTICES.md).
