package app

import (
	"encoding/json"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func (a *App) launchAccountBinding(accountID, bindingID string, expected cliadapter.ConfigContext) {
	project := a.project()
	if project == nil || a.starting || a.accountBlocked() || !a.accountCanConnectBinding() {
		return
	}
	if expected.Home != a.home || expected.ServerURL == nil || expected.DeviceID == nil || !matchingCapabilityContext(expected, a.account.expected) || a.account.status.Account.ID != accountID || *a.account.status.Binding.BindingSessionID != bindingID {
		a.message = presentation.Notice(fault.CLIContextChanged)
		return
	}
	args, err := session.BindingArguments(*expected.ServerURL, *expected.DeviceID, accountID, bindingID)
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	reference, err := session.BindingReference(accountID, bindingID)
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	// Live reuse also requires the confirmed server/device. History deliberately
	// does not persist this connection authority or attempt an automatic resume.
	keyBytes, _ := json.Marshal([]string{expected.Home, *expected.ServerURL, *expected.DeviceID, reference})
	key := string(keyBytes)
	for _, record := range a.sessions.Records() {
		if record.Tool != session.AccountBinding || record.Exited || record.ProjectID != project.ID || a.bindingConnections[record.ID] != key {
			continue
		}
		if saved, ok := a.savedSession(record.ID); ok && saved.Reference == reference {
			a.page = ""
			a.focusSession(record.ID)
			return
		}
	}
	a.launch(launchRequest{projectID: project.ID, directory: project.Path, tool: session.AccountBinding, args: args, reference: reference, name: locales.AccountBindingConnect(), bindingKey: key})
}
