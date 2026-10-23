package presentation_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

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
