package app

import (
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func (a *App) accountDetailsView(c *ui.Context, account remote.AccountDetails) {
	ui.Text(c, account.Name).Bold()
	ui.Text(c, locales.AccountExactID(account.ID)).FontSize(12)
	ui.Text(c, locales.AccountState(account.Status)).Bold()
	if a.account.status != nil && !a.account.review && !a.account.submitted {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			if account.Tool == "claude" && remote.AccountImportSupported(a.account.capabilities, "accounts.import.prepare") {
				ui.Button(c, locales.AccountImportTitle()).Disabled(a.accountBlocked()).OnClick(a.openAccountImport)
			}
			for _, action := range []string{"bind", "verify", "disable"} {
				ui.Button(c, locales.AccountActionTitle(action)).Disabled(!a.accountOperationAvailable("accounts."+action) || account.Status == "disabled" && action != "bind").OnClick(func() { a.reviewAccountAction(action) })
			}
		})
	}
	ui.Text(c, locales.AccountFieldValue(locales.AccountFieldTool(), account.Tool)).FontSize(12)
	ui.Text(c, locales.AccountFieldValue(locales.AccountFieldRegion(), account.Region)).FontSize(12)
	ui.Text(c, locales.AccountFieldValue(locales.AccountFieldTimezone(), account.Timezone)).FontSize(12)
	ui.Text(c, locales.AccountFieldValue(locales.AccountFieldLocale(), account.Locale)).FontSize(12)
	if len(account.Tags) > 0 {
		ui.Text(c, locales.AccountFieldValue(locales.AccountFieldTags(), strings.Join(account.Tags, ", "))).FontSize(12)
	}
	if account.AffinityNodeID != nil {
		ui.Text(c, locales.AccountNode(*account.AffinityNodeID)).FontSize(12)
	}
	if account.RuntimeBackend != nil {
		ui.Text(c, locales.RemoteBackend(*account.RuntimeBackend)).FontSize(12)
	}
	ui.Text(c, locales.AccountCreatedAt(accountDate(account.CreatedAt))).FontSize(12)
	ui.Text(c, locales.AccountUpdatedAt(accountDate(account.UpdatedAt))).FontSize(12)
}
func (a *App) accountBindingDetails(c *ui.Context, binding remote.AccountBinding) {
	ui.Text(c, locales.AccountBindingStatus(locales.AccountState(binding.Status))).Bold()
	if binding.NodeID != nil {
		ui.Text(c, locales.AccountNode(*binding.NodeID)).FontSize(12)
	}
	if binding.BindingSessionID != nil {
		ui.Text(c, locales.AccountBindingSession(*binding.BindingSessionID)).FontSize(12)
	}
	if binding.TmuxSessionName != nil {
		ui.Text(c, locales.AccountBindingTmux(*binding.TmuxSessionName)).FontSize(12)
	}
	if binding.RemotePath != nil {
		ui.Text(c, *binding.RemotePath).FontSize(12)
	}
	if binding.TaskID != nil {
		ui.Text(c, locales.AccountTask(*binding.TaskID)).FontSize(12)
	}
	if binding.ErrorCode != nil {
		ui.Text(c, locales.AccountBindingFailed()).TextColor(c.Theme().Danger)
	}

}
func accountDate(raw string) string {
	if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return i18n.FormatDate(i18n.Locale(), parsed.In(time.Local), i18n.DateOptions{Date: i18n.MediumStyle, Time: i18n.ShortStyle})
	}
	return raw
}
