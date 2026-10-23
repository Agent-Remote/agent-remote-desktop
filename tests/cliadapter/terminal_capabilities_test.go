package cliadapter_test

import (
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

func TestTerminalCapabilityIsSeparateAndVersioned(t *testing.T) {
	const method = "accounts.binding.attach"
	c := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: runtime.GOOS,
		Operations: []cliadapter.Capability{{Method: "accounts.status", Effect: "read"}}}
	if c.SupportsTerminal(method) {
		t.Fatal("legacy CLI claimed terminal support")
	}
	c.TerminalOperations = []cliadapter.TerminalCapability{{Method: method, Version: 1}}
	if !c.SupportsTerminal(method) || c.Supports(method) || c.SupportsEffect(method, "write") {
		t.Fatal("terminal and control contracts conflated")
	}
	c.TerminalOperations[0].Version = 2
	if c.SupportsTerminal(method) {
		t.Fatal("future terminal protocol accepted")
	}
	c.TerminalOperations[0].Version = 1
	c.Platform = "unsupported"
	if c.SupportsTerminal(method) {
		t.Fatal("foreign platform accepted")
	}
	c.Platform = runtime.GOOS
	c.TerminalOperations = append(c.TerminalOperations, c.TerminalOperations[0])
	if c.Validate() == nil || c.SupportsTerminal(method) {
		t.Fatal("duplicate terminal capability accepted")
	}
	c.TerminalOperations = c.TerminalOperations[:1]
	c.Operations = append(c.Operations, cliadapter.Capability{Method: method, Effect: "write"})
	if c.Validate() == nil || c.SupportsTerminal(method) {
		t.Fatal("ambiguous transport accepted")
	}
}
