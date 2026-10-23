//go:build darwin || linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// rawMode turns off line editing, echo and signal keys, as an Agent's TUI
// does, so every byte arrives as typed.
func rawMode() func() {
	fd := int(os.Stdin.Fd())
	old, err := unix.IoctlGetTermios(fd, ioctlGet)
	if err != nil {
		return func() {}
	}
	raw := *old
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN], raw.Cc[unix.VTIME] = 1, 0
	if unix.IoctlSetTermios(fd, ioctlSet, &raw) != nil {
		return func() {}
	}
	return func() { _ = unix.IoctlSetTermios(fd, ioctlSet, old) }
}
