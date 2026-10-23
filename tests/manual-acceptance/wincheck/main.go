//go:build windows

// wincheck is the Windows chord check (the Linux one is
// ../linux/chord-check.sh): with the desktop running on the acceptance home
// and AGENT_REMOTE_ACCEPTANCE_LOG set, it brings the window forward, starts
// the Codex stand-in from the create wheel and sends key events through
// SendInput, which takes the same path as a keyboard. It passes when the
// Agent received exactly the letters typed — nothing from the chords, and
// Alt did not leave the window in its menu loop.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	procSendInput     = user32.NewProc("SendInput")
	procFindWindow    = user32.NewProc("FindWindowW")
	procSetForeground = user32.NewProc("SetForegroundWindow")
	procGetForeground = user32.NewProc("GetForegroundWindow")
	procShowWindow    = user32.NewProc("ShowWindow")
	procMapVirtualKey = user32.NewProc("MapVirtualKeyW")
	procSendMessage   = user32.NewProc("SendMessageW")
	imm32             = windows.NewLazySystemDLL("imm32.dll")
	procDefaultIMEWnd = imm32.NewProc("ImmGetDefaultIMEWnd")
)

// The input method's open state, read and set from outside the window's
// thread through its default IME window (WM_IME_CONTROL).
const (
	wmIMEControl     = 0x0283
	imcGetOpenStatus = 0x0005
	imcSetOpenStatus = 0x0006
	imcGetConversion = 0x0001
	imcSetConversion = 0x0002
	imeCModeNative   = 0x0001 // IME_CMODE_NATIVE: Pinyin converts to Chinese
)

func imeOpen(hwnd uintptr) (bool, bool) {
	ime, _, _ := procDefaultIMEWnd.Call(hwnd)
	if ime == 0 {
		return false, false
	}
	st, _, _ := procSendMessage.Call(ime, wmIMEControl, imcGetOpenStatus, 0)
	return st != 0, true
}

func setIME(hwnd uintptr, open bool) {
	ime, _, _ := procDefaultIMEWnd.Call(hwnd)
	v := uintptr(0)
	if open {
		v = 1
	}
	procSendMessage.Call(ime, wmIMEControl, imcSetOpenStatus, v)
	if open {
		mode, _, _ := procSendMessage.Call(ime, wmIMEControl, imcGetConversion, 0)
		procSendMessage.Call(ime, wmIMEControl, imcSetConversion, mode|imeCModeNative)
	}
	pause(200)
}

// imeNative reports whether the input method converts to Chinese.
func imeNative(hwnd uintptr) bool {
	ime, _, _ := procDefaultIMEWnd.Call(hwnd)
	mode, _, _ := procSendMessage.Call(ime, wmIMEControl, imcGetConversion, 0)
	return mode&imeCModeNative != 0
}

// keyboardInput is INPUT{type=INPUT_KEYBOARD} padded to sizeof(INPUT).
type keyboardInput struct {
	kind      uint32
	_         uint32
	vk        uint16
	scan      uint16
	flags     uint32
	time      uint32
	extraInfo uintptr
	_         [8]byte
}

const (
	vkMenu, vkControl, vkReturn, vkEscape = 0x12, 0x11, 0x0D, 0x1B
	keyUpFlag                             = 0x0002
)

func send(vk uint16, up bool) {
	in := keyboardInput{kind: 1, vk: vk}
	scan, _, _ := procMapVirtualKey.Call(uintptr(vk), 0)
	in.scan = uint16(scan)
	if up {
		in.flags = keyUpFlag
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	time.Sleep(25 * time.Millisecond)
}

func tap(vk uint16)        { send(vk, false); send(vk, true) }
func letter(r rune) uint16 { return uint16(strings.ToUpper(string(r))[0]) }
func typeText(s string) {
	for _, r := range s {
		tap(letter(r))
		time.Sleep(40 * time.Millisecond)
	}
}
func pause(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }
func chord(key rune, hold int) {
	send(vkMenu, false)
	send(letter(key), false)
	send(letter(key), true)
	pause(hold)
	send(vkMenu, true)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "step" {
		step(os.Args[2:])
		return
	}
	log := os.Getenv("AGENT_REMOTE_ACCEPTANCE_LOG")
	report := map[string]any{"probe": "windows-chord-check", "goarch": runtime.GOARCH, "injection": "SendInput"}
	err := run(log, report)
	report["status"] = "passed"
	if err != nil {
		report["status"], report["reason"] = "failed", err.Error()
	}
	out, _ := json.Marshal(report)
	fmt.Println(string(out))
	if err != nil {
		os.Exit(1)
	}
}

func run(log string, report map[string]any) error {
	if log == "" {
		return errors.New("AGENT_REMOTE_ACCEPTANCE_LOG is not set")
	}
	title, _ := windows.UTF16PtrFromString("Agent Remote Desktop")
	var hwnd uintptr
	for i := 0; i < 50 && hwnd == 0; i++ {
		hwnd, _, _ = procFindWindow.Call(0, uintptr(unsafe.Pointer(title)))
		pause(100)
	}
	if hwnd == 0 {
		return errors.New("desktop window not found")
	}
	procShowWindow.Call(hwnd, 9) // SW_RESTORE
	// Foreground rules: a tap of Alt lets this process bring a window forward.
	tap(vkMenu)
	procSetForeground.Call(hwnd)
	pause(500)
	if fg, _, _ := procGetForeground.Call(); fg != hwnd {
		return errors.New("desktop window did not come to the foreground")
	}
	send(vkControl, false)
	tap('T')
	send(vkControl, true) // Ctrl+T: the create wheel
	pause(1000)
	_ = os.Remove(log + ".ready")
	tap('2')
	tap(vkReturn)              // the Codex stand-in
	for i := 0; i < 100; i++ { // emulated x64 starts slowly
		if _, err := os.Stat(log + ".ready"); err == nil {
			break
		}
		pause(100)
	}
	if _, err := os.Stat(log + ".ready"); err != nil {
		return errors.New("the Codex stand-in did not start")
	}
	pause(800)
	// The plain chord path runs with the input method closed (English), as
	// on a keyboard without one; its state is restored afterwards.
	was, known := imeOpen(hwnd)
	report["ime_initially_open"], report["ime_control"] = was, known
	setIME(hwnd, false)
	defer setIME(hwnd, was)
	start := size(log)
	typeText("hi")
	pause(300)
	chord('a', 0) // tap: the dock stays open
	pause(500)
	tap(vkEscape)
	pause(400)
	chord('x', 400) // hold, then release: commits
	pause(500)
	chord('c', 0) // tap: the wheel stays open
	pause(500)
	tap(vkEscape)
	pause(400)
	typeText("ok") // reaches the Agent only if Alt left no menu loop behind
	pause(800)
	got, err := since(log, start)
	if err != nil {
		return err
	}
	report["agent_bytes"] = fmt.Sprintf("% x", got)
	if string(got) != "hiok" {
		return fmt.Errorf("the Agent received %q, want \"hiok\"", got)
	}
	if known {
		imeChord(hwnd, log, report)
	}
	return nil
}

// imeChord observes, without failing the check, what a chord does with
// the input method open: idle, and in the middle of a composition.
func imeChord(hwnd uintptr, log string, report map[string]any) {
	setIME(hwnd, true)
	start := size(log)
	chord('a', 0) // idle input method
	pause(500)
	tap(vkEscape)
	pause(400)
	idle, _ := since(log, start)
	report["ime_native"] = imeNative(hwnd)
	typeText("ni") // composing 你: the letters stay in the input method
	pause(300)
	chord('a', 0)
	pause(500)
	tap(vkEscape) // closes the dock
	pause(400)
	composing, _ := since(log, start)
	report["ime_open_idle_chord_bytes"] = fmt.Sprintf("% x", idle)
	report["ime_composing_chord_bytes"] = fmt.Sprintf("% x", composing[len(idle):])
	setIME(hwnd, false)
}

// step sends one action to the desktop window, for stepping through a case
// with screenshots between: ime-on, ime-off, type <text>, chord <key>, esc.
func step(args []string) {
	title, _ := windows.UTF16PtrFromString("Agent Remote Desktop")
	hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		fmt.Println("window not found")
		os.Exit(1)
	}
	tap(vkMenu)
	procSetForeground.Call(hwnd)
	pause(300)
	switch args[0] {
	case "ime-on":
		setIME(hwnd, true)
	case "ime-off":
		setIME(hwnd, false)
	case "type":
		typeText(args[1])
	case "chord":
		chord(rune(args[1][0]), 0)
	case "esc":
		tap(vkEscape)
	}
	open, _ := imeOpen(hwnd)
	fmt.Println("ok; ime open:", open, "native:", imeNative(hwnd))
}

// The log only grows (the stand-in appends): a step reads what came after
// its start, so no one truncates a file another process is writing.
func size(path string) int64 {
	if st, err := os.Stat(path); err == nil {
		return st.Size()
	}
	return 0
}

func since(path string, start int64) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return b[min(int(start), len(b)):], nil
}
