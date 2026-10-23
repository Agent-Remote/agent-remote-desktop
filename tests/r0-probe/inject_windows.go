package main

import (
	"errors"
	"time"
	"unsafe"

	"github.com/egoist/mygo"
	"golang.org/x/sys/windows"
)

const platformInjector = "SendInput"

var (
	user32                 = windows.NewLazySystemDLL("user32.dll")
	procSendInput          = user32.NewProc("SendInput")
	procSetForeground      = user32.NewProc("SetForegroundWindow")
	procMapVirtualKeyW     = user32.NewProc("MapVirtualKeyW")
	procGetForeground      = user32.NewProc("GetForegroundWindow")
	procAllowSetForeground = user32.NewProc("AllowSetForegroundWindow")
	procGetFocus           = user32.NewProc("GetFocus")

	imm32                 = windows.NewLazySystemDLL("imm32.dll")
	procImmGetContext     = imm32.NewProc("ImmGetContext")
	procImmReleaseContext = imm32.NewProc("ImmReleaseContext")
	procImmGetOpenStatus  = imm32.NewProc("ImmGetOpenStatus")
	procImmSetOpenStatus  = imm32.NewProc("ImmSetOpenStatus")
)

// setIME opens or closes the input method of the focused window (Microsoft
// Pinyin and other IMEs) and reports whether it was open. It runs on the UI
// thread, which owns the window's input context.
func (w *winInjector) setIME(open bool) (was bool, err error) {
	done := make(chan struct{})
	mygo.RunOnMain(func() {
		defer close(done)
		h, _, _ := procGetFocus.Call()
		if h == 0 {
			h = w.hwnd
		}
		himc, _, _ := procImmGetContext.Call(h)
		if himc == 0 {
			err = errors.New("window has no input context")
			return
		}
		st, _, _ := procImmGetOpenStatus.Call(himc)
		was = st != 0
		v := uintptr(0)
		if open {
			v = 1
		}
		procImmSetOpenStatus.Call(himc, v)
		procImmReleaseContext.Call(h, himc)
	})
	select {
	case <-done:
		return was, err
	case <-time.After(2 * time.Second):
		return false, errors.New("setIME: UI thread did not answer")
	}
}

// keyboardInput is INPUT{type=INPUT_KEYBOARD} with its KEYBDINPUT union
// member, padded to sizeof(INPUT) on 64-bit Windows (40 bytes).
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
	inputKeyboard = 1
	keyUp         = 0x0002
	vkMenu        = 0x12 // Alt
	vkSpace       = 0x20
)

type winInjector struct{ hwnd uintptr }

func newInjector(win *mygo.Window) (injector, error) {
	if unsafe.Sizeof(keyboardInput{}) != 40 {
		return nil, errors.New("INPUT layout mismatch")
	}
	hwnd := win.NativeHandle()
	procAllowSetForeground.Call(^uintptr(0)) // ASFW_ANY
	procSetForeground.Call(hwnd)
	if fg, _, _ := procGetForeground.Call(); fg != hwnd {
		return nil, errors.New("probe window is not the foreground window (SendInput would go elsewhere)")
	}
	return &winInjector{hwnd: hwnd}, nil
}

func vkOf(k rune) uint16 {
	switch {
	case k == altKey:
		return vkMenu
	case k == ' ':
		return vkSpace
	case k >= 'a' && k <= 'z':
		return uint16(k - 'a' + 'A')
	}
	return 0
}

func (w *winInjector) send(k rune, up bool) {
	vk := vkOf(k)
	if vk == 0 {
		return
	}
	scan, _, _ := procMapVirtualKeyW.Call(uintptr(vk), 0) // MAPVK_VK_TO_VSC
	in := keyboardInput{kind: inputKeyboard, vk: vk, scan: uint16(scan)}
	if up {
		in.flags = keyUp
	}
	procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func (w *winInjector) down(k rune) { w.send(k, false) }
func (w *winInjector) up(k rune)   { w.send(k, true) }
