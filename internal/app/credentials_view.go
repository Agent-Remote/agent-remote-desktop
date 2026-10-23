package app

import (
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) credentialsView(c *ui.Context) {
	p := &a.credentials
	createAvailable := a.credentialsMutationAvailable(remote.CredentialsCreateMethod)
	bindAvailable := a.credentialsMutationAvailable(remote.CredentialsBindMethod)
	unbindAvailable := a.credentialsMutationAvailable(remote.CredentialsUnbindMethod)
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(14).Children(func() {
		ui.Row(c).Gap(12).Wrap().Children(func() {
			ui.Button(c, locales.RefreshRemote()).Disabled(p.busy || p.mutationBusy).OnClick(func() {
				p.loaded = false
				a.refreshCredentials()
			})
		})
		ui.Text(c, locales.CredentialsHelp()).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.CredentialsPrivacy()).FontSize(12).TextColor(c.Theme().TextMuted)
		if p.receipt != nil {
			ui.Text(c, locales.CredentialsMutationAccepted(p.receipt.Action)).Bold()
			ui.Text(c, p.receipt.OperationID).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if p.action == "create" && !p.review && !p.mutationBusy {
			ui.Text(c, locales.CredentialsCreateTitle()).Bold()
			ui.TextInput(c, &p.form.DisplayName).Label(locales.CredentialsDisplayName()).Placeholder(locales.CredentialsDisplayName()).FillWidth()
			ui.TextInput(c, &p.gitUserName).Label(locales.CredentialsGitUserName()).Placeholder(locales.CredentialsGitUserName()).FillWidth()
			ui.TextInput(c, &p.gitUserEmail).Label(locales.CredentialsGitUserEmail()).Placeholder(locales.CredentialsGitUserEmail()).FillWidth()
			ui.TextInput(c, &p.form.GHMode).Label(locales.CredentialsGitHubMode()).Placeholder(locales.CredentialsGitHubModeDefault()).FillWidth()
			ui.TextInput(c, &p.form.SSHMode).Label(locales.CredentialsSSHMode()).Placeholder(locales.CredentialsSSHModeDefault()).FillWidth()
		}
		if p.review {
			ui.Text(c, locales.CredentialsReviewHelp()).TextColor(c.Theme().TextMuted)
			if p.action == "create" {
				ui.Text(c, p.form.DisplayName)
			} else {
				ui.Text(c, locales.CredentialsReviewAccount(p.accountID))
				if p.action == "bind" {
					ui.Text(c, locales.CredentialsReviewProfile(p.profileID))
				}
			}
		} else if p.action == "bind" || p.action == "unbind" {
			ui.TextInput(c, &p.accountID).Label(locales.CredentialsAccountID()).Placeholder(locales.CredentialsAccountID()).FillWidth()
			if p.action == "bind" {
				ui.TextInput(c, &p.profileID).Label(locales.CredentialsProfileID()).Placeholder(locales.CredentialsProfileID()).FillWidth()
			}
		}
		if p.busy {
			ui.Text(c, locales.CredentialsLoading()).TextColor(c.Theme().TextMuted)
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(p.cancel)
			return
		}
		if p.mutationBusy {
			ui.Text(c, locales.CredentialsMutationWorking()).TextColor(c.Theme().TextMuted)
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.cancelCredentialMutation)
		}
		if p.err != nil {
			text := presentation.Error(p.err)
			var operation *cliadapter.OperationError
			if errors.As(p.err, &operation) && operation.Code == "unsupported_method" {
				text = locales.CredentialsUnsupported()
			}
			ui.Text(c, text).TextColor(c.Theme().Danger)
			return
		}
		if !p.loaded {
			return
		}
		ui.Text(c, p.expected.Home).FontSize(12).TextColor(c.Theme().TextMuted)
		if p.expected.ServerURL != nil {
			ui.Text(c, *p.expected.ServerURL).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			if len(p.items) == 0 {
				ui.Text(c, locales.CredentialsEmpty()).TextColor(c.Theme().TextMuted)
				return
			}
			for _, profile := range p.items {
				a.card(c, ui.Column(c.Key(profile.ID)).FillWidth().Padding(14).Gap(5)).Children(func() {
					ui.Row(c).Gap(10).Wrap().Children(func() {
						ui.Text(c, profile.DisplayName).Bold().Grow(1)
						ui.Text(c, locales.CredentialsStatus(profile.Status)).TextColor(c.Theme().TextMuted)
					})
					ui.Text(c, profile.ID).FontSize(12).TextColor(c.Theme().TextMuted)
					ui.Text(c, locales.CredentialsModes(profile.GitHubCLIMode, profile.SSHMode)).FontSize(12)
					ui.Text(c, locales.CredentialsUpdated(profile.UpdatedAt)).FontSize(12).TextColor(c.Theme().TextMuted)
				})
			}
		})
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Button(c, locales.CredentialsCreateTitle()).Disabled(!createAvailable || a.credentialsBlocked() || p.review || p.action == "create").OnClick(func() {
				p.action = "create"
				p.form = remote.CredentialCreateInput{Expected: p.expected, GHMode: "remote_login", SSHMode: "agent_forwarding"}
				p.gitUserName, p.gitUserEmail = "", ""
			})
			ui.Button(c, locales.CredentialsBindTitle()).Disabled(!bindAvailable || a.credentialsBlocked() || p.review).OnClick(func() { p.action = "bind" })
			ui.Button(c, locales.CredentialsUnbindTitle()).Disabled(!unbindAvailable || a.credentialsBlocked() || p.review).OnClick(func() { p.action = "unbind" })
			if p.review {
				ui.Button(c, locales.Cancel()).OnClick(func() { p.review = false })
				ui.PrimaryButton(c, locales.CredentialsConfirm()).Disabled(a.credentialsBlocked()).OnClick(a.submitCredentialMutation)
			} else if p.action == "create" {
				ui.PrimaryButton(c, locales.CredentialsReview()).Disabled(a.credentialsBlocked()).OnClick(a.reviewCredentialCreate)
			} else if p.action == "bind" || p.action == "unbind" {
				available := p.action == "bind" && bindAvailable || p.action == "unbind" && unbindAvailable
				ui.PrimaryButton(c, locales.CredentialsConfirm()).Disabled(!available || a.credentialsBlocked() || p.accountID == "" || p.action == "bind" && p.profileID == "").OnClick(func() { p.review = true })
			}
		})
	})
}
