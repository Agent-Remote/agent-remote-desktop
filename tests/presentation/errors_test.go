package presentation_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/egoist/mygo/i18n"
)

func TestExistingErrorChangesLanguageWithoutChangingIdentity(t *testing.T) {
	locale := "en"
	i18n.SetLocaleFunc(func() string { return locale })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	err := fmt.Errorf("context: %w", fault.StopTimeout)
	notice := presentation.Notice(err)
	if got := notice(); !strings.Contains(got, "force stop") {
		t.Fatal(got)
	}
	locale = "zh-Hans"
	if got := notice(); !strings.Contains(got, "强制结束") {
		t.Fatal(got)
	}
	if !errors.Is(err, fault.StopTimeout) {
		t.Fatal("localization changed error identity")
	}
}

// Found against a real CLI and server (R5 live pages): a home signed in
// without a device, and a Mac without the Device app, are states, not an
// incompatible CLI.
func TestDeviceStatesAreNotProtocolErrors(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	protocol := presentation.Error(&cliadapter.OperationError{Code: "some_new_code"})
	for _, code := range []string{"device_required", "local_status_unavailable"} {
		if got := presentation.Error(&cliadapter.OperationError{Code: code}); got == protocol {
			t.Errorf("%s reads as an incompatible CLI: %q", code, got)
		}
	}
}
