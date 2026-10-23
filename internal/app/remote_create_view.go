package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) remoteCreateDialog(c *ui.Context) {
	ui.Modal(c, &a.remote.createOpen, func() {
		_, height := c.Size()
		ui.Column(c).Width(660).Height(min(560, height-60)).Gap(12).Children(func() {
			ui.Text(c, locales.CreateRemoteSession()).FontSize(22).Bold()
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Gap(10).Children(func() {
				ui.Text(c, locales.CreateRemoteSessionHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
				if a.remote.createRecovery {
					ui.Text(c, locales.SessionCreateRecoveryHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
					server, device := "", ""
					if a.remote.createContext.ServerURL != nil {
						server = *a.remote.createContext.ServerURL
					}
					if a.remote.createContext.DeviceID != nil {
						device = *a.remote.createContext.DeviceID
					}
					ui.Text(c, locales.SessionCreateRecoveryContext(server, device)).FontSize(12).TextColor(c.Theme().TextMuted)
				}
				if !remote.SessionCreateSupported(a.remote.snapshot.Capabilities) && !remote.SessionCreateQuerySupported(a.remote.snapshot.Capabilities) {
					ui.Text(c, locales.SessionCreateUnsupported()).TextColor(c.Theme().Danger)
				}
				if !a.remote.createRecovery {
					ui.TextInput(c, &a.remote.createTool).Label(locales.SessionCreateTool()).Placeholder(locales.SessionCreateToolPlaceholder())
					ui.TextInput(c, &a.remote.createAccount).Label(locales.SessionCreateAccount()).Placeholder(locales.SessionCreateAccountPlaceholder())
					ui.TextInput(c, &a.remote.createWorkspace).Label(locales.SessionCreateWorkspace()).Placeholder(locales.SessionCreateWorkspacePlaceholder())
					ui.TextInput(c, &a.remote.createProject).Label(locales.SessionCreateProject()).Placeholder(locales.SessionCreateProjectPlaceholder())
					ui.TextInput(c, &a.remote.createDigest).Label(locales.SessionCreateArgvDigest()).Placeholder(locales.SessionCreateDigestPlaceholder())
					ui.TextInput(c, &a.remote.createSyncTarget).Label(locales.SessionCreateSyncTarget()).Placeholder(locales.SessionCreateTargetPlaceholder())
					ui.TextInput(c, &a.remote.createReplace).Label(locales.SessionCreateReplacement()).Placeholder(locales.SessionCreateReplacementPlaceholder())
					ui.Text(c, locales.SessionCreateReplacementContext()).FontSize(12).TextColor(c.Theme().TextMuted)
					ui.TextInput(c, &a.remote.createReplaceAccount).Label(locales.SessionCreateReplacementAccount()).Placeholder(locales.SessionCreateAccountPlaceholder())
					ui.TextInput(c, &a.remote.createReplaceWorkspace).Label(locales.SessionCreateReplacementWorkspace()).Placeholder(locales.SessionCreateWorkspacePlaceholder())
					ui.TextInput(c, &a.remote.createReplaceProject).Label(locales.SessionCreateReplacementProject()).Placeholder(locales.SessionCreateProjectPlaceholder())
					ui.TextInput(c, &a.remote.createReplaceNode).Label(locales.SessionCreateReplacementNode()).Placeholder(locales.SessionCreateNodePlaceholder())
					ui.TextInput(c, &a.remote.createReplaceBackend).Label(locales.SessionCreateReplacementBackend()).Placeholder(locales.SessionCreateBackendPlaceholder())
				}
				ui.TextInput(c, &a.remote.createQueryKey).Label(locales.SessionCreateQueryKey()).Placeholder(locales.SessionCreateQueryPlaceholder()).Disabled(a.remote.createRecovery)
				if !a.remote.createRecovery {
					ui.Text(c, locales.SessionCreateReviewHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
				}
				if a.remote.createBusy {
					ui.Text(c, locales.SessionCreatePending()).TextColor(c.Theme().TextMuted)
				}
				if !a.remote.createRecovery && !a.remote.createBusy && !remoteCreateInputValid(a.remote.createTool, a.remote.createAccount, a.remote.createWorkspace, a.remote.createProject, a.remote.createDigest, a.remote.createSyncTarget) {
					ui.Text(c, locales.SessionCreateInputInvalid()).TextColor(c.Theme().Danger)
				}
				if a.remote.createErr != nil {
					a.errorText(c, a.remote.createErr, presentation.Error(a.remote.createErr))
				}
				if a.remote.createReceipt != nil {
					a.remoteCreateReceiptView(c, *a.remote.createReceipt)
				}
			})
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.SessionCreateCancel()).Disabled(a.remote.createBusy).OnClick(func() { a.remote.createOpen = false })
				ui.PrimaryButton(c, locales.SessionCreateSubmit()).Disabled(!a.remoteCreateConfirmationCurrent()).OnClick(a.submitRemoteCreate)
				ui.Button(c, locales.SessionCreateQuery()).Disabled(!a.remoteCreateQueryCurrent()).OnClick(a.queryRemoteCreate)
			})
		})
	})
}

func (a *App) remoteCreateReceiptView(c *ui.Context, receipt remote.SessionCreateReceipt) {
	a.card(c, ui.Column(c).FillWidth().Padding(12).Gap(6)).Children(func() {
		ui.Text(c, locales.SessionCreatePhase(receipt.Phase)).Bold()
		ui.Text(c, locales.SessionCreateRequest(receipt.RequestID)).FontSize(12)
		if receipt.SessionID != nil && receipt.TaskID != nil {
			ui.Text(c, locales.SessionCreateTask(*receipt.SessionID, *receipt.TaskID)).FontSize(12)
		} else if receipt.SessionID == nil {
			ui.Text(c, locales.SessionCreateNoSession()).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if receipt.NodeID != "" {
			ui.Text(c, locales.RemoteAttachNode(receipt.NodeID)).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if receipt.ErrorCode != nil {
			ui.Text(c, presentation.Error(&cliadapter.OperationError{Code: *receipt.ErrorCode})).FontSize(12).TextColor(c.Theme().Danger)
		}
		ui.Text(c, locales.SessionCreateReceiptHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	})
}
