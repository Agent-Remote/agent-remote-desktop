# Hand checklist (redesign §10)

Mark each line ✅ / ❌, and add what you saw if it failed. In the Agent
pane, "nothing" means the stand-in printed no new `received` line.

## A. Keyboard (physical keyboard; ⌥ = Option on macOS, Alt elsewhere)

1. Ctrl+T (⌘T on macOS), then 2, then Enter starts **codex 1**. Type `abc`:
   three `received` lines show 61, 62, 63.
2. Tap ⌥A: the dock drops and stays. Nothing reaches the Agent. Esc closes it.
3. Hold ⌥ and press A twice: the selection moves. Let go of ⌥: it switches.
4. Tap ⌥C: the wheel stays. Esc. Hold ⌥, press C, point at Codex with the
   mouse, then let go: a second Codex starts.
5. Hold ⌥ and press X, then let go at once on nothing: the wheel closes and
   nothing changes.
6. Tap ⌥S: the Control Center rises. Tab to a tile, then Enter: its page
   pushes in. Esc goes back, and Esc again closes.
7. Windows/Linux only: tap Alt alone, then type `q`. Note what happens (a
   menu bar may show, and `q` may go to it). This is mygo's
   AutoHideMenuBar; report it, it is not a chord.
8. Settings → Layer shortcut → ⌃⌥ / Ctrl+Alt. Now ⌥A types into the Agent
   (macOS: `c3 a5`), and ⌃⌥A opens the dock. Set it back to ⌥.

## B. Input method (macOS: Pinyin – Simplified; Windows: Microsoft Pinyin)

1. Switch to Pinyin and click the Agent pane. Type `ni`: candidates show,
   and nothing reaches the Agent yet.
2. With the candidates still showing, press ⌥A: note whether the dock opens,
   and whether the composition is kept, committed or dropped. Esc.
3. Press Space or 1 to commit 你: the Agent receives `e4 bd a0`.
4. In the ⌥X search field, type a project name in Pinyin: it composes as
   usual.

## C. Screen reader

- macOS: VoiceOver (⌘F5). Windows: Narrator (Ctrl+Win+Enter).

1. Tap ⌥C, then use the arrows. Each sector is read by name (Claude, Codex,
   fclaude, Advanced…).
2. Tap ⌥A. Each dock card is read with its name and state, not colour alone.
3. ⌥S, then Tab. Tiles and the appearance switch are read, and the switch
   is announced as a group of choices.
4. On a page, Back to Control Center and Back to workbench are read.

## D. Look and feel (trackpad or mouse)

1. Drag the Control Center sheet down halfway and back up: the workspace
   behind scales with it and does not jump.
2. Pull the breathing bar up: the sheet follows the finger. Let go past
   half and it opens; let go short and it falls back.
3. Push the dock up: it closes the way it came.
4. Switch Light/Dark/Kawaii in the Control Center: no flash, and the
   terminal keeps its contents.
5. Settings → Reduce transparency: surfaces become solid with no blur.
   Reduce motion: no movement, short fades only.
6. Resize the window to its minimum (900×600): nothing overlaps or is cut
   off in any layer.

## E. Design sign-off

Approve or change these R4/R5 values: the PillText label colour, the
kawaii Success `#059669`, the kawaii Solid `#FFF8FB`, the dock status pill
max width 132, and the palette sitting a third of the way down.
