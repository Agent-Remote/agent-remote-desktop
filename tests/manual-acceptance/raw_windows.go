package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// rawMode turns off line input and echo and asks for VT input, so the
// ConPTY delivers every byte as typed, as an Agent's TUI sees it.
func rawMode() func() {
	h := windows.Handle(os.Stdin.Fd())
	var old uint32
	if windows.GetConsoleMode(h, &old) != nil {
		return func() {}
	}
	mode := old &^ (windows.ENABLE_LINE_INPUT | windows.ENABLE_ECHO_INPUT | windows.ENABLE_PROCESSED_INPUT)
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	if windows.SetConsoleMode(h, mode) != nil {
		return func() {}
	}
	return func() { _ = windows.SetConsoleMode(h, old) }
}
