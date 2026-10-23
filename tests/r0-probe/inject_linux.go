package main

import (
	"errors"
	"os/exec"
	"strings"
	"time"

	"github.com/egoist/mygo"
)

const platformInjector = "xdotool (XTEST)"

type xInjector struct{ id string }

// newInjector finds the probe window by title and gives it the X input focus.
// XTEST events (xdotool keydown/keyup without --window) are indistinguishable
// from hardware input for the client; --window would use XSendEvent instead.
func newInjector(_ *mygo.Window) (injector, error) {
	if _, err := exec.LookPath("xdotool"); err != nil {
		return nil, errors.New("xdotool not installed")
	}
	var id string
	for i := 0; i < 30 && id == ""; i++ {
		out, _ := exec.Command("xdotool", "search", "--onlyvisible", "--name", "Agent Remote R0 Probe").Output()
		id = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
		if id == "" {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if id == "" {
		return nil, errors.New("probe window not found on the X display")
	}
	if err := exec.Command("xdotool", "windowfocus", "--sync", id).Run(); err != nil {
		return nil, err
	}
	return &xInjector{id: id}, nil
}

func keysym(k rune) string {
	switch k {
	case altKey:
		return "Alt_L"
	case ' ':
		return "space"
	}
	return string(k)
}

func (x *xInjector) down(k rune) { _ = exec.Command("xdotool", "keydown", keysym(k)).Run() }
func (x *xInjector) up(k rune)   { _ = exec.Command("xdotool", "keyup", keysym(k)).Run() }
