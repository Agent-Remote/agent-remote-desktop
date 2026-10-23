# Redesign acceptance on real windows

Redesign §10 checks that need a real window system, a real keyboard path or
a real CLI. Every check uses a disposable home and stand-in Agents. These
are copies of `go run ./tests/manual-acceptance` named codex, claude and
fclaude, which print every byte the terminal sends, so a leaked chord shows
up as bytes (`c3 a5` is å). No real Agent starts, and your own desktop
state is not touched.

## Builds

```sh
O=/tmp/ara-build; mkdir -p $O
go build -o $O/desktop-darwin . && go build -o $O/acceptance-darwin ./tests/manual-acceptance
for a in arm64 amd64; do
  GOOS=linux   GOARCH=$a go build -o $O/desktop-linux-$a .
  GOOS=linux   GOARCH=$a go build -o $O/acceptance-linux-$a ./tests/manual-acceptance
  GOOS=windows GOARCH=$a go build -ldflags=-H=windowsgui -o $O/desktop-windows-$a.exe .
  GOOS=windows GOARCH=$a go build -o $O/acceptance-windows-$a.exe ./tests/manual-acceptance
  GOOS=windows GOARCH=$a go build -o $O/wincheck-$a.exe ./tests/manual-acceptance/wincheck
done
```

## Scripted chord checks (OS-level key events into the real app)

Each check starts the Codex stand-in, then types `hi`, taps Alt+A (the dock
stays open), Esc, holds Alt+X and releases it (commits), taps Alt+C (the
wheel stays open), Esc, and types `ok`. It passes when the Agent received
exactly `68 69 6f 6b`.

- **Linux X11** (Xvfb + openbox + fcitx5; xdotool XTEST, which a client
  cannot tell from hardware). Use `--platform linux/amd64` for x86_64, run
  under emulation.
  ```sh
  docker build --platform linux/arm64 -t ara-linux:arm64 tests/manual-acceptance/linux
  docker run -d --name ara-linux --platform linux/arm64 -p 127.0.0.1:5901:5900 -v /tmp/ara-build:/work:ro ara-linux:arm64
  docker exec ara-linux /chord-check.sh
  ```
- **Linux Wayland**: headless sway, the GTK Wayland backend, and wtype on
  the virtual-keyboard protocol. wtype has to send the Alt_L key and the
  modifier state together, as a keyboard does; `-M alt` alone never lets go
  of Alt.
  ```sh
  docker build --platform linux/arm64 -t ara-wayland:arm64 tests/manual-acceptance/wayland
  docker run -d --name ara-wl --platform linux/arm64 -p 127.0.0.1:5903:5900 -v /tmp/ara-build:/work:ro ara-wayland:arm64
  docker exec ara-wl /chord-check.sh
  ```
- **Windows** (SendInput, in the signed-in session). Start the desktop with
  `AGENT_REMOTE_ACCEPTANCE_LOG` set, then run `wincheck-<arch>.exe` in the
  same session. It closes the input method for the plain run, then repeats
  Alt+A with Microsoft Pinyin open, both idle and mid-composition, and
  reports those bytes separately.

To watch any of these, connect to `vnc://127.0.0.1:5901` (X11, password
`agent`) or `:5903` (Wayland) with macOS Screen Sharing.

## Real CLI and server: every Control Center page

`tests/live-pages-probe` opens all tiles headlessly against a real
`agent-remote` and the server its home is signed in to, and reports
protocol or contract errors. It only reads. Use a disposable server,
for example the Server repository's image under its own compose project
with only `127.0.0.1` ports, and a disposable home:

```sh
agent-remote login --home /tmp/cli-home --server-url http://127.0.0.1:18000 --username … --skip-device-registration
go run ./tests/live-pages-probe -home /tmp/cli-home -cli "$(command -v agent-remote)"
```

## By hand (a person at the keyboard)

On each platform, start the desktop on the acceptance home. On macOS:
`go run ./tests/manual-acceptance`. Then go through `checklist.md` beside
this file.
