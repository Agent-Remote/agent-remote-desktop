package remote_test

import (
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestExactSessionAttachmentStatusIsTextOnlyAndValid(t *testing.T) {
	status := remote.ExactSessionAttachmentStatus()
	if err := status.Validate(); err != nil {
		t.Fatal(err)
	}
	if status.Version != 1 || status.Mode != remote.AttachmentTextOnly || !status.TextClipboard || status.Files || status.Images {
		t.Fatalf("unexpected exact attachment status: %+v", status)
	}
}

func TestAttachmentStatusRejectsCapabilityWidening(t *testing.T) {
	cases := []remote.AttachmentStatus{
		{Version: 2, Mode: remote.AttachmentTextOnly, TextClipboard: true, Reason: "future"},
		{Version: 1, Mode: remote.AttachmentTextOnly, TextClipboard: true, Files: true, Reason: "legacy"},
		{Version: 1, Mode: remote.AttachmentTextOnly, TextClipboard: true, Images: true, Reason: "legacy"},
		{Version: 1, Mode: "all", TextClipboard: true, Reason: "legacy"},
	}
	for _, status := range cases {
		if status.Validate() == nil {
			t.Fatalf("capability widening accepted: %+v", status)
		}
	}
}

func TestExactSessionAttachmentStatusNeedsCLIAndTerminalDeclarations(t *testing.T) {
	base := cliadapter.Capabilities{
		Version: 1, CLIVersion: "test", Platform: runtime.GOOS,
		Operations:         []cliadapter.Capability{{Method: "context", Effect: "read"}},
		TerminalOperations: []cliadapter.TerminalCapability{{Method: "sessions.attach", Version: 1}},
		AttachmentTransfer: cliadapter.AttachmentTransferCapability{Supported: true, ProtocolVersion: 1, MaxBytes: 256 * 1024 * 1024, MaxEntries: 32},
	}
	status := remote.ExactSessionAttachmentStatusForCapabilities(base)
	if err := status.Validate(); err != nil || !status.Files || !status.Images || status.Mode != remote.AttachmentExactTransfer {
		t.Fatalf("status = %+v, err = %v", status, err)
	}
	limits, err := status.Limits()
	if err != nil || limits.MaxBytes != base.AttachmentTransfer.MaxBytes || limits.MaxEntries != base.AttachmentTransfer.MaxEntries {
		t.Fatalf("status limits = %+v, err = %v", limits, err)
	}
	base.AttachmentTransfer.ProtocolVersion = 2
	status = remote.ExactSessionAttachmentStatusForCapabilities(base)
	if err := status.Validate(); err != nil || status.Files || status.Images || status.Mode != remote.AttachmentTextOnly {
		t.Fatalf("invalid declaration widened status: %+v, err = %v", status, err)
	}
	if _, err := status.Limits(); err == nil {
		t.Fatal("text-only attachment status exposed transfer limits")
	}
}
