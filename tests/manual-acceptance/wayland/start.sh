#!/bin/sh
# Headless sway + wayvnc on :5900 (no password; bind to 127.0.0.1 only),
# then the desktop on GTK's Wayland backend with a disposable home.
set -e
export XDG_RUNTIME_DIR=/tmp/xdg-tester WLR_BACKENDS=headless WLR_LIBINPUT_NO_DEVICES=1 WLR_RENDERER=pixman
mkdir -p "$XDG_RUNTIME_DIR" && chmod 700 "$XDG_RUNTIME_DIR"
export AGENT_REMOTE_ACCEPTANCE_LOG=/tmp/agent-bytes.log
printf 'output HEADLESS-1 resolution 1440x900\ndefault_border none\n' > /tmp/sway.conf
eval "$(dbus-launch --sh-syntax)"
sway -c /tmp/sway.conf >/tmp/sway.log 2>&1 &
for i in $(seq 1 50); do ls "$XDG_RUNTIME_DIR"/wayland-* >/dev/null 2>&1 && break; sleep 0.1; done
export WAYLAND_DISPLAY=$(basename "$(ls "$XDG_RUNTIME_DIR"/wayland-? | head -1)")
echo "$WAYLAND_DISPLAY" > /tmp/wayland-display
wayvnc 0.0.0.0 5900 >/tmp/wayvnc.log 2>&1 &
arch=$(uname -m); [ "$arch" = x86_64 ] && arch=amd64; [ "$arch" = aarch64 ] && arch=arm64
/work/acceptance-linux-$arch -prepare -root /tmp/ara >/tmp/ara.txt
export PATH=/tmp/ara/bin:$PATH GDK_BACKEND=wayland
while true; do /work/desktop-linux-$arch -home /tmp/ara/home >/tmp/desktop.log 2>&1 || true; sleep 1; done
