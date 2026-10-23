package remote

import (
	"fmt"

	"github.com/Agent-Remote/agent-remote-desktop/internal/attachment"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

// AttachmentMode describes the payloads that the exact remote-session
// connection is allowed to transfer. It is deliberately independent of the
// generic fclaude attach bridge: that bridge does not prove the expected
// device and SSH key used by sessions.attach.
type AttachmentMode string

const (
	AttachmentTextOnly      AttachmentMode = "text_only"
	AttachmentExactTransfer AttachmentMode = "exact_transfer"
)

// AttachmentStatus is the desktop-side projection of the exact attachment
// boundary. The transfer variant reflects only the CLI's declared v1 bridge;
// upload authority still comes from the identity-bound Server/Node lease.
type AttachmentStatus struct {
	Version       int
	Mode          AttachmentMode
	TextClipboard bool
	Files         bool
	Images        bool
	MaxBytes      int64
	MaxEntries    int
	Reason        string
}

// ExactSessionAttachmentStatus returns the fail-closed status used when the
// required capability declaration is absent. Text copy/paste remains
// available through the terminal; file and image payloads stay disabled.
func ExactSessionAttachmentStatus() AttachmentStatus {
	return AttachmentStatus{
		Version:       1,
		Mode:          AttachmentTextOnly,
		TextClipboard: true,
		Files:         false,
		Images:        false,
		Reason:        "expected_device_and_ssh_key_transfer_binding_unavailable",
	}
}

// ExactSessionAttachmentStatusForCapabilities enables payloads only when the
// same CLI capability response declares both the exact terminal and the v1
// transfer bridge. The CLI still obtains a fresh lease from Server/Node for
// every transfer; this function never grants upload authority itself.
func ExactSessionAttachmentStatusForCapabilities(capabilities cliadapter.Capabilities) AttachmentStatus {
	if capabilities.SupportsExactAttachment() {
		return AttachmentStatus{
			Version:       1,
			Mode:          AttachmentExactTransfer,
			TextClipboard: true,
			Files:         true,
			Images:        true,
			MaxBytes:      capabilities.AttachmentTransfer.MaxBytes,
			MaxEntries:    capabilities.AttachmentTransfer.MaxEntries,
			Reason:        "cli_and_exact_session_transfer_v1_declared",
		}
	}
	return ExactSessionAttachmentStatus()
}

// Validate rejects an accidental capability widening or malformed transfer
// limits. Keep this strict so a caller cannot mark files or images as
// available without the separately reviewed protocol declaration.
func (s AttachmentStatus) Validate() error {
	if s.Version != 1 || !s.TextClipboard || s.Reason == "" {
		return fmt.Errorf("invalid exact remote attachment status")
	}
	switch s.Mode {
	case AttachmentTextOnly:
		if s.Files || s.Images || s.MaxBytes != 0 || s.MaxEntries != 0 {
			return fmt.Errorf("invalid exact remote attachment status")
		}
	case AttachmentExactTransfer:
		if !s.Files || !s.Images || s.MaxBytes <= 0 || s.MaxBytes > 256*1024*1024 || s.MaxEntries <= 0 || s.MaxEntries > 32 {
			return fmt.Errorf("invalid exact remote attachment status")
		}
	default:
		return fmt.Errorf("invalid exact remote attachment status")
	}
	return nil
}

// Limits returns the exact-transfer limits after validating the status.  A
// text-only status deliberately returns invalid limits so callers cannot
// accidentally interpret a disabled bridge as unlimited.
func (s AttachmentStatus) Limits() (attachment.Limits, error) {
	if err := s.Validate(); err != nil || s.Mode != AttachmentExactTransfer {
		if err != nil {
			return attachment.Limits{}, err
		}
		return attachment.Limits{}, fmt.Errorf("exact attachment transfer is unavailable")
	}
	limits := attachment.Limits{MaxBytes: s.MaxBytes, MaxEntries: s.MaxEntries}
	if err := limits.Validate(); err != nil {
		return attachment.Limits{}, err
	}
	return limits, nil
}
