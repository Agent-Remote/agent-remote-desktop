#!/bin/sh
# Scripted chord check inside the container (docker exec ... /chord-check.sh):
# real X server, XTEST key events (as hardware), the real desktop and the
# stand-in Agent. Passes when the Agent received exactly the plain letters
# typed and nothing from Alt+A (tap), Alt+X (hold) or Alt+C (tap).
set -eu
export DISPLAY=:1
log=/tmp/agent-bytes.log
w=$(xdotool search --sync --onlyvisible --name "Agent Remote Desktop" | head -1)
xdotool windowactivate --sync "$w"; sleep 0.5
xdotool key ctrl+t; sleep 1.2            # ⌘T/Ctrl+T: the create wheel
rm -f "$log.ready"
xdotool key 2 Return                     # sector 2: the Codex stand-in
i=0; while [ ! -e "$log.ready" ] && [ $i -lt 100 ]; do sleep 0.1; i=$((i+1)); done
[ -e "$log.ready" ] || { echo '{"probe":"linux-chord-check","status":"failed","reason":"stand-in did not start"}'; exit 1; }
sleep 0.8
start=$(stat -c %s "$log" 2>/dev/null || echo 0)
xdotool type --delay 60 "hi"; sleep 0.4
xdotool keydown alt key a keyup alt; sleep 0.6   # tap: dock stays open
xdotool key Escape; sleep 0.5
xdotool keydown alt key x; sleep 0.4; xdotool keyup alt; sleep 0.6   # hold, release
xdotool keydown alt key c keyup alt; sleep 0.6   # tap: wheel stays open
xdotool key Escape; sleep 0.5
xdotool type --delay 60 "ok"; sleep 0.6
got=$(tail -c +$((start+1)) "$log" | od -An -tx1 | tr -s ' \n' ' ' | sed 's/^ //;s/ $//')
want="68 69 6f 6b"
arch=$(uname -m)
if [ "$got" = "$want" ]; then
  printf '{"probe":"linux-chord-check","arch":"%s","display":"x11-xvfb","injection":"xdotool XTEST","agent_bytes":"%s","status":"passed"}\n' "$arch" "$got"
else
  printf '{"probe":"linux-chord-check","arch":"%s","display":"x11-xvfb","injection":"xdotool XTEST","agent_bytes":"%s","want":"%s","status":"failed"}\n' "$arch" "$got" "$want"
  exit 1
fi
