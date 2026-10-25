# Community release build

`agent-remote-desktop` publishes community builds. The release policy is:

- macOS uses the project's persistent self-signed `community-local-trust`
  identity (the ad-hoc identity is reserved for explicit smoke builds);
- Windows and Linux do not require publisher code signing;
- Apple Developer ID and notarization are not part of the release path; the
  workflow uploads ordinary GitHub release assets only after offline
  verification;
- macOS provisioning profiles are not used; the community certificate is
  checked directly by the build and bundle verification steps;
- FreeBSD is not a release target.

`community-local-trust` is the release profile name; the current certificate
subject used by the example environment is `Agent Remote Community Code
Signing`. Both refer to the same project-controlled community self-signing
identity, rather than an Apple Developer ID certificate.

## Tagged GitHub release workflow

The repository's [community release workflow](../.github/workflows/release.yml)
is the only automated publication path. It accepts a `v*` tag (including a
manual run selected on a tag), checks that the tag exactly matches the formal
version in `mygo.json`, runs the complete desktop check, and then builds the
default `darwin/arm64`, `windows/amd64` and `linux/amd64` set on a macOS
runner. The build imports the protected project community certificate into a
temporary keychain, runs `build-release.sh`, verifies the resulting bundle and
manifest offline, installs the macOS package into a temporary user directory
and runs `release-native-smoke.sh` against the copied bundle, retains the
archive as workflow evidence, and publishes the tagged GitHub release only
after those checks succeed.

The protected `community-release` environment supplies
`COMMUNITY_MACOS_CERTIFICATE_P12_B64`,
`COMMUNITY_MACOS_CERTIFICATE_PASSWORD`,
`COMMUNITY_MACOS_SIGNING_IDENTITY`, and
`COMMUNITY_MACOS_CERTIFICATE_SHA1`. The keychain is removed in an `always()`
cleanup step. The workflow never enables ad-hoc mode, does not publish a
Windows/Linux publisher signature, and does not make an Apple notarization
request. Keep the P12 and password in the environment secret store; they must
not be committed or attached to the release.

The workflow's successful build and offline verification prove that the tagged
source was packaged under the community policy. They do not prove native
Windows/Linux windowing, tray, PTY, permission broker or Agent/Server/Node
integration. Those gates still require the host evidence described in the
[platform validation matrix](platform-validation.md).

The tagged workflow passes the first formal target set explicitly:
`darwin/arm64,windows/amd64,linux/amd64`. It also invokes the offline verifier
with `--require-targets` for that exact set, so a changed build default or an
inherited runner variable cannot silently publish a partial release. Local
smoke builds may still select a subset with `--platforms` and verify it without
`--require-targets`.

## Prepare a component release

The repository follows the same two-step component release flow as the other
agent-remote repositories. `scripts/prepare-release.sh` updates `mygo.json`,
regenerates and checks the i18n catalog, and prepends the commit range to
`CHANGELOG.md` through `scripts/update-changelog.sh`:

```sh
scripts/prepare-release.sh 0.1.0
git diff -- mygo.json CHANGELOG.md
```

The protected `prepare-release` workflow accepts a semantic version without
the leading `v`, rejects an existing tag, runs the complete desktop checks,
commits `chore: release vX.Y.Z`, pushes `main`, and pushes `vX.Y.Z`. The tag
then starts the tagged release workflow. Review the open host, broker, remote
integration and performance gates in `implementation-status.md` before running
the workflow; preparing source metadata does not close those gates.

The reproducible packaging entry point is:

```sh
DESKTOP_MACOS_SIGNING_IDENTITY="Agent Remote Community Code Signing" \
DESKTOP_MACOS_SIGNER_CERTIFICATE_SHA1="<40 uppercase hex characters>" \
scripts/build-release.sh --out dist/release --clean
```

The identity and certificate fingerprint come from the protected community
release environment; private keys and P12 files never belong in Git or release
assets. For a local cross-build that intentionally does not claim community
signature evidence, set `DESKTOP_ALLOW_ADHOC=1`; its manifest records
`mode=ad-hoc/smoke`, identity `-` and no certificate fingerprint. It must not
be treated as a formal macOS release.

The default targets are `darwin/arm64`, `windows/amd64` and `linux/amd64`.
Use `--platforms` (or `DESKTOP_RELEASE_PLATFORMS`) to select a subset or an
additional supported `amd64`/`arm64` target. The script accepts `--skip-dmg`
for a cross-build smoke check; a macOS `.dmg` requires running mygo on macOS.
Target entries must be unique; a duplicate target is rejected before any
build or output-directory mutation.
The normal command builds with `CGO_ENABLED=0`, delegates packaging to the
mygo version pinned in `go.mod`, verifies that the configured macOS certificate
is the expected fingerprint and is self-signed, verifies the bundle signature,
and never passes `-upload`. An explicit `DESKTOP_MACOS_SIGNING_IDENTITY=-` is
treated as ad-hoc signing and is rejected unless `DESKTOP_ALLOW_ADHOC=1` is
also set.

The completed directory is then checked offline by
[`scripts/verify-release.sh`](../scripts/verify-release.sh). The verifier
first binds the manifest application identity to `Agent Remote Desktop` and
`dev.agent-remote.desktop`; a correctly checksummed archive for another
application cannot pass by changing only its metadata. It then requires the
manifest and `SHA256SUMS` artifact sets to match byte-for-byte,
rejects symlinks and unlisted files, and requires every declared target to
contain its desktop executable and `agent-remote-desktop-installer` helper
(`.exe` on Windows). A target directory containing an arbitrary placeholder
file therefore cannot pass release verification. It also enforces the
community policy fields:
macOS uses `project-self-signed`, Windows/Linux set publisher signing to
`false`, and notarization is `false`. On a non-macOS host it reports the native
macOS signature check as deferred; run the same verifier on macOS to inspect
the bundle with `codesign`. A directory produced with
`DESKTOP_ALLOW_ADHOC=1` is accepted only with the explicit
`--allow-adhoc` verifier flag and is smoke evidence, not a formal release.
The bundled installer repeats the product identity check before copying files,
so Windows hosts without Bash cannot bypass this binding.

To re-check a copied release without source files or network access:

```sh
scripts/verify-release.sh dist/release
```

For a formal three-platform directory, require the exact target set explicitly:

```sh
scripts/verify-release.sh dist/release \
  --require-targets darwin/arm64,windows/amd64,linux/amd64
```

The output directory contains one subdirectory per target under `artifacts/`:

- macOS: a project-self-signed `.app`; on macOS, a `.dmg` containing the app;
- Windows: the GUI `.exe` and bundled `ghostty-vt.dll`; an NSIS installer is
  produced only when `makensis` is installed;
- Linux: the executable, desktop entry, install script, tar archive and Debian
  package where mygo supports that architecture. The Debian package declares
  `libayatana-appindicator3-1 | libappindicator3-1` in addition to mygo's GTK
  dependency because the workbench keeps a tray entry while its window is
  hidden. The pinned mygo Debian generator may also retain its conservative
  WebKitGTK package dependency even though this desktop uses native-content
  windows and loads WebKitGTK only for page windows. The runtime probe still
  verifies the GTK/AppIndicator ABI and the tray probe creates an actual icon
  on the target desktop; package metadata alone is not native runtime evidence.
- Every target also includes `agent-remote-desktop-installer` (with `.exe` on
  Windows). This small Go helper validates the local manifest/checksums and
  performs an atomic install or upgrade; it does not download, elevate, sign,
  or install external Agent CLIs.

Every packaged application includes mygo's terminal native library. The
script reads the exact `mygo-plugin.json` files from the resolved module graph
and records each native source URL, upstream checksum, shipped-file checksum
and license notice in `metadata/release-manifest.json`. A project-signed
macOS dylib has an embedded signature, so its shipped checksum can differ from
the unsigned upstream checksum; the manifest records that post-build
transformation explicitly. It also records the Go module list and hashes of
`go.mod`, `go.sum` and `THIRD_PARTY_NOTICES.md`.

The Rust CLI is an external runtime dependency and is never silently copied
into the desktop package. The manifest probes `agent-remote`, `fclaude`,
`codex` and `claude` on `PATH`; set `DESKTOP_CLI_AGENT_REMOTE_BIN`,
`DESKTOP_CLI_FCLAUDE_BIN`, `DESKTOP_CLI_CODEX_BIN` or
`DESKTOP_CLI_CLAUDE_BIN` to record a specific binary's version and SHA-256.
Missing tools are recorded as `not-found`, because the desktop can still be
installed and used for other workflows.

The helper can be used by a package integrator or a user who has already
obtained the complete release directory:

```sh
artifacts/linux-amd64/agent-remote-desktop-installer \
  --release . --destination "$HOME/.local/opt/agent-remote-desktop" \
  --platform linux --arch amd64
# Restore the previous installed tree after an upgrade:
artifacts/linux-amd64/agent-remote-desktop-installer \
  --rollback --destination "$HOME/.local/opt/agent-remote-desktop"
```

Before an upgrade, the same helper can report the managed installation and
whether a rollback tree is available without reading or changing application
files:

```sh
artifacts/linux-amd64/agent-remote-desktop-installer \
  --status --destination "$HOME/.local/opt/agent-remote-desktop"
```

An absent destination is returned as `installed:false`; an existing directory
without installer state is rejected as unmanaged. This makes upgrade prompts
safe to build around and avoids treating a partially moved installation as a
successful install.

Installation refuses a target not declared by `metadata/release-manifest.json`,
missing or changed SHA-256 entries, symlinked artifacts, unmanaged destination
directories, destinations inside the release tree, identifier/target changes,
and implicit downgrades. A successful upgrade keeps one previous tree and its
sidecar state. Rollback swaps that tree back atomically; the old current tree
is retained during the swap and cleaned up only after success.
The destination must be writable by the current user; system package-manager
installs remain the package manager's responsibility.

`SHA256SUMS` is relative to the output directory and covers every packaged
artifact. Verify it after copying the complete release directory:

```sh
cd dist/release
sha256sum -c SHA256SUMS       # or: shasum -a 256 -c SHA256SUMS
```

The manifest's `git_dirty` field is informational. A formal release should be
built from a clean, reviewable commit and should archive the manifest, checksums
and dependency notices with the release. Cross-compilation and checksum
verification prove build reproducibility only; native windows, terminal,
background-lifecycle, permission and Agent/CLI integration checks still belong
to the platform validation matrix.

After a release is copied to a target host, run the packaged smoke helper:

```sh
scripts/release-native-smoke.sh dist/release
```

It selects `GOHOSTOS/GOHOSTARCH`, runs the offline verifier, installs the exact
host target into a temporary directory, checks installer `--status`, and runs
the installed desktop executable with `--version`, matching the manifest
version exactly. On macOS it additionally walks from the installed executable
to the copied `.app` bundle and runs `codesign --verify --deep --strict` after
installation, so the smoke covers the installer output as well as the source
release directory. An inherited `GOOS` or
`GOARCH` is ignored, so this command cannot accidentally report a cross-build
as native execution evidence. The helper is user-scoped and does not elevate
or modify a system installation.

On a Windows host without Git Bash, use the PowerShell entry point. It selects
the current native Windows architecture, reads only the declared
`windows/<arch>` artifacts, runs the packaged installer in a temporary user
directory, checks `--status`, and launches the installed executable with
`--version`:

```powershell
pwsh -File scripts/release-native-smoke.ps1 -ReleaseDir dist/release
```

For a deliberately ad-hoc macOS smoke directory, add `-AllowAdhoc`; the
default remains the formal community policy.

When Bash is available the PowerShell entry point also invokes
`verify-release.sh`; the installer remains the required offline manifest and
checksum check when Bash is unavailable. Neither smoke entry point elevates,
installs a service, or claims WireGuard broker evidence.

Before calling a release ready, collect the complete `native-acceptance.sh`
output from one macOS host, one Windows host and two Linux hosts (one X11 and
one Wayland), then run the evidence joiner from a review machine:

```sh
scripts/verify-native-evidence.sh --require-wireguard --require-linux-display-pair \
  /path/to/macos-evidence \
  /path/to/windows-evidence \
  /path/to/linux-x11-evidence \
  /path/to/linux-wayland-evidence
```

The joiner requires a passed platform, installer, native-window, workbench and
completed native WireGuard broker probe on every host, checks that `summary.txt`
and `metadata.txt` identify the same `GOOS/GOARCH`, and rejects evidence marked
as cross-compilation. The formal target architecture is
`darwin/arm64`, `windows/amd64` and `linux/amd64`. With
`--require-linux-display-pair`, the two Linux records must also carry
`display_protocol=x11` and `display_protocol=wayland` respectively. Formal
evidence must use a host display. CI runs that used Xvfb can be reviewed with
`--allow-virtual-display`, but that explicit result is not a physical desktop
acceptance result and cannot be used to close the release gate by itself.

Before recording host evidence, run the read-only platform probe from the same
display session as the UI probes:

```sh
go run ./tests/platform-probe
```

The JSON report is explicitly scoped to the process's current host
(`host_runtime=true`, `cross_compilation=false`). It reports required target,
shell and (on Linux) display, GTK 3 and AppIndicator checks. It also records
optional WebKitGTK 4.1/4.0 and Windows WebView2 observations; these runtimes
are loaded lazily by mygo only for page/WebView windows, while this desktop's
workbench uses native-content windows. Successful pkg-config observations
include the detected ABI package version in their detail. On Windows the
optional probe follows mygo's Evergreen WebView2 discovery in HKCU/HKLM (both registry views), reading the
`EdgeUpdate\ClientState\<channel>\EBWebView` directory and confirming
`msedgewebview2.exe` plus the architecture-specific
`EmbeddedBrowserWebView.dll`; it returns `fail` for a missing or stale runtime
and `unknown` when registry access is denied. macOS and Windows display
availability remains `unknown` until the native-window probe runs. Linux
runtime dependencies are fail-closed for GTK 3 and AppIndicator: a missing
`pkg-config`, GTK 3, or AppIndicator ABI makes the platform probe fail.
Missing optional WebKitGTK is recorded without blocking native-content startup.
The native acceptance also runs `tests/tray-probe`, which
creates and destroys a real mygo tray icon/menu; its host-only JSON is required
by the evidence joiner. A cross-build must never be used to fill these fields.

On Windows without Git Bash, use the PowerShell equivalent:

```powershell
pwsh -File scripts/native-acceptance.ps1 -EvidenceDir "$env:TEMP\agent-remote-desktop-native"
```

To record that a required host is unavailable, run the same entry point with
an explicit target on any coordinator host (for example,
`scripts/native-acceptance.sh --target windows`). It writes
`status=not_run`/`reason=target host is unavailable` and exits non-zero;
`verify-native-evidence.sh` rejects that record. The entry point never sets
`GOOS`/`GOARCH` or executes a cross-compiled binary. PowerShell accepts the
equivalent `-Target windows|linux|darwin` option.

Both native acceptance entries record the resolved Go and mygo module versions in `metadata.txt`; the evidence joiner rejects records missing a concrete mygo version. They preserve a failure record when a probe exits:
`summary.txt` names the failed step and exit code, while the corresponding
step log remains available for the platform validation report.

## Platform prerequisites

Go 1.27.1 (or the version required by `go.mod`), Python 3 for manifest
generation, and the network access needed for mygo's pinned native assets are
required. Linux runtime users need GTK 3 and an AppIndicator implementation
for the tray; WebKitGTK 4.1/4.0 is optional until a page/WebView window is
introduced. Windows runtime users need native ConPTY; WebView2 is optional for
the same reason. macOS runtime users need macOS 13 or newer. These runtime
components are OS dependencies and are not repackaged by this script.
