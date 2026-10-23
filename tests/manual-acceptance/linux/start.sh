#!/bin/sh
# Container entry: X (Xvfb) + openbox + fcitx5 Pinyin + VNC on :5900
# (password "agent", bind it to 127.0.0.1 only), then the desktop with a
# disposable acceptance home. /work holds desktop-linux-<arch> and
# acceptance-linux-<arch> (see README.md beside this file).
set -e
export DISPLAY=:1 XMODIFIERS=@im=fcitx GTK_IM_MODULE=fcitx
export AGENT_REMOTE_ACCEPTANCE_LOG=/tmp/agent-bytes.log
Xvfb :1 -screen 0 1440x900x24 &
sleep 1
eval "$(dbus-launch --sh-syntax)"
openbox &
fcitx5 -d >/dev/null 2>&1 || true
x11vnc -display :1 -forever -shared -passwd agent -rfbport 5900 -quiet &
arch=$(uname -m); [ "$arch" = x86_64 ] && arch=amd64; [ "$arch" = aarch64 ] && arch=arm64
/work/acceptance-linux-$arch -prepare -root /tmp/ara >/tmp/ara.txt
export PATH=/tmp/ara/bin:$PATH
while true; do /work/desktop-linux-$arch -home /tmp/ara/home || true; sleep 1; done
