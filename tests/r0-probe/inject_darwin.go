package main

import (
	"errors"
	"os"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

const platformInjector = "CGEventPostToPid"

// ANSI virtual key codes (HIToolbox/Events.h).
var macKeys = map[rune]uint16{
	'a': 0x00, 's': 0x01, 'd': 0x02, 'h': 0x04, 'z': 0x06, 'x': 0x07, 'c': 0x08,
	'q': 0x0C, 'w': 0x0D, 'e': 0x0E, 'r': 0x0F, 'y': 0x10, 't': 0x11,
	'o': 0x1F, 'l': 0x25, ' ': 0x31, altKey: 0x3A,
}

const (
	maskAlt      = 0x00080000 // kCGEventFlagMaskAlternate
	maskNonCoal  = 0x00000100 // kCGEventFlagMaskNonCoalesced
	statePrivate = -1         // kCGEventSourceStatePrivate: flags never leak from the real keyboard
)

type macInjector struct {
	pid     int
	source  uintptr
	alt     bool
	inApp   bool
	nsEvent objc.ID
	nsApp   objc.ID
	create  func(source uintptr, key uint16, down bool) uintptr
	flags   func(ev uintptr, flags uint64)
	setType func(ev uintptr, kind uint32)
	post    func(pid int32, ev uintptr)
	free    func(ref uintptr)
}

const eventFlagsChanged = 12 // kCGEventFlagsChanged

func newInjector(_ *mygo.Window) (injector, error) {
	cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, err
	}
	m := &macInjector{pid: os.Getpid()}
	var newSource func(state int32) uintptr
	var preflight func() bool
	purego.RegisterLibFunc(&newSource, cg, "CGEventSourceCreate")
	purego.RegisterLibFunc(&m.create, cg, "CGEventCreateKeyboardEvent")
	purego.RegisterLibFunc(&m.flags, cg, "CGEventSetFlags")
	purego.RegisterLibFunc(&m.setType, cg, "CGEventSetType")
	purego.RegisterLibFunc(&m.post, cg, "CGEventPostToPid")
	purego.RegisterLibFunc(&m.free, cf, "CFRelease")
	purego.RegisterLibFunc(&preflight, cg, "CGPreflightPostEventAccess")
	if m.source = newSource(statePrivate); m.source == 0 {
		return nil, errors.New("CGEventSourceCreate failed")
	}
	// Without the Accessibility/"post events" grant, CGEventPostToPid is
	// silently dropped. Fall back to posting the same CGEvent as an NSEvent
	// into our own AppKit queue: it skips the window server but takes the
	// rest of the hardware path (NSApplication → key window → keyDown:/
	// flagsChanged: → interpretKeyEvents:), and needs no permission.
	m.inApp = !preflight()
	if m.inApp {
		m.nsEvent = objc.ID(objc.GetClass("NSEvent"))
		m.nsApp = objc.ID(objc.GetClass("NSApplication")).Send(objc.RegisterName("sharedApplication"))
	}
	return m, nil
}

func (m *macInjector) name() string {
	if m.inApp {
		return "NSApp postEvent (CGEvent; no Accessibility grant)"
	}
	return platformInjector
}

func (m *macInjector) send(k rune, down bool) {
	code, ok := macKeys[k]
	if !ok {
		return
	}
	if k == altKey {
		m.alt = down
	}
	ev := m.create(m.source, code, down)
	if ev == 0 {
		return
	}
	var f uint64 = maskNonCoal
	if m.alt {
		f |= maskAlt
	}
	m.flags(ev, f)
	if k == altKey {
		// A modifier key arrives as flagsChanged:, never keyDown:/keyUp:.
		m.setType(ev, eventFlagsChanged)
	}
	if m.inApp {
		ns := m.nsEvent.Send(objc.RegisterName("eventWithCGEvent:"), ev)
		m.nsApp.Send(objc.RegisterName("postEvent:atStart:"), ns, false)
	} else {
		m.post(int32(m.pid), ev)
	}
	m.free(ev)
}

func (m *macInjector) down(k rune) { m.send(k, true) }
func (m *macInjector) up(k rune)   { m.send(k, false) }
