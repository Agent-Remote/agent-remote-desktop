#!/bin/sh
# Scripted chord check on Wayland (docker exec ... /chord-check.sh): wtype
# sends through the virtual-keyboard protocol to the focused surface. Passes
# when the stand-in Agent received exactly the plain letters typed.
set -eu
export XDG_RUNTIME_DIR=/tmp/xdg-tester WAYLAND_DISPLAY=$(cat /tmp/wayland-display)
log=/tmp/agent-bytes.log
# As a keyboard does: the Alt_L key event together with the modifier state.
# (wtype -M alone sets the state only; GTK then never sees Alt let go.)
chord() {
  if [ -n "${2:-}" ]; then wtype -M alt -P Alt_L -k "$1" -s "$2" -p Alt_L -m alt
  else wtype -M alt -P Alt_L -k "$1" -p Alt_L -m alt; fi
}
sleep 1
wtype -M ctrl t -m ctrl; sleep 1.2
rm -f "$log.ready"
wtype 2; sleep 0.2; wtype -k Return
i=0; while [ ! -e "$log.ready" ] && [ $i -lt 100 ]; do sleep 0.1; i=$((i+1)); done
[ -e "$log.ready" ] || { echo '{"probe":"wayland-chord-check","status":"failed","reason":"stand-in did not start"}'; exit 1; }
sleep 0.8
start=$(stat -c %s "$log" 2>/dev/null || echo 0)
wtype hi; sleep 0.4
chord a; sleep 0.6; wtype -k Escape; sleep 0.5      # tap: dock stays open
chord x 500; sleep 0.6                                # hold, release: commits
chord c; sleep 0.6; wtype -k Escape; sleep 0.5      # tap: wheel stays open
wtype ok; sleep 0.6
got=$(tail -c +$((start+1)) "$log" | od -An -tx1 | tr -s ' \n' ' ' | sed 's/^ //;s/ $//')
want="68 69 6f 6b"
if [ "$got" = "$want" ]; then
  printf '{"probe":"wayland-chord-check","arch":"%s","display":"wayland-sway-headless","injection":"wtype virtual-keyboard","agent_bytes":"%s","status":"passed"}\n' "$(uname -m)" "$got"
else
  printf '{"probe":"wayland-chord-check","arch":"%s","display":"wayland-sway-headless","injection":"wtype virtual-keyboard","agent_bytes":"%s","want":"%s","status":"failed"}\n' "$(uname -m)" "$got" "$want"
  exit 1
fi
