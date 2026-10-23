package main

// altKey stands for the platform's Alt/Option key in injector calls; every
// other rune is a lowercase ASCII letter or space.
const altKey rune = 0x2325 // ⌥

// injector sends OS-level key events to the probe's own window, so they take
// the same path as hardware input: backend → engine → shortcuts → terminal.
type injector interface {
	down(k rune)
	up(k rune)
}

// imeControl is implemented where the probe can open/close the platform
// input method (Windows IMM). The chord baseline runs with it closed; an
// extra observation repeats a chord with it open.
type imeControl interface {
	setIME(open bool) (was bool, err error)
}

// named lets an injector report a fallback path it chose at runtime.
type named interface{ name() string }

func injectorName(manual bool) string {
	if manual {
		return "manual"
	}
	return platformInjector
}

func describe(inj injector) string {
	if n, ok := inj.(named); ok {
		return n.name()
	}
	return platformInjector
}
