package cliadapter_test

import (
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

func TestExactAttachmentRequiresTerminalAndTransferDeclarations(t *testing.T) {
	base := cliadapter.Capabilities{
		Version: 1, CLIVersion: "test", Platform: runtime.GOOS,
		Operations:         []cliadapter.Capability{{Method: "context", Effect: "read"}},
		TerminalOperations: []cliadapter.TerminalCapability{{Method: "sessions.attach", Version: 1}},
		AttachmentTransfer: cliadapter.AttachmentTransferCapability{Supported: true, ProtocolVersion: 1, MaxBytes: 256 * 1024 * 1024, MaxEntries: 32},
	}
	if !base.SupportsExactAttachment() {
		t.Fatal("v1 exact session and transfer declarations should enable payload bridge")
	}
	for name, mutate := range map[string]func(*cliadapter.Capabilities){
		"missing transfer":       func(c *cliadapter.Capabilities) { c.AttachmentTransfer = cliadapter.AttachmentTransferCapability{} },
		"wrong transfer version": func(c *cliadapter.Capabilities) { c.AttachmentTransfer.ProtocolVersion = 2 },
		"oversized limits":       func(c *cliadapter.Capabilities) { c.AttachmentTransfer.MaxBytes = 256*1024*1024 + 1 },
		"missing terminal":       func(c *cliadapter.Capabilities) { c.TerminalOperations = nil },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := base
			mutate(&candidate)
			if candidate.SupportsExactAttachment() {
				t.Fatal("invalid capability declaration enabled payload transfer")
			}
		})
	}
}
