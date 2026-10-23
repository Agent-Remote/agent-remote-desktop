package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

type accountPage struct {
	expected                    cliadapter.ConfigContext
	capabilities                cliadapter.Capabilities
	accountID                   string
	form                        remote.AccountCreateInput
	tags                        string
	action                      string
	review                      bool
	bindingReview               bool
	bindingProject              string
	bindingProjectID, bindingID string
	loading, busy, submitted    bool
	generation                  uint64
	status                      *remote.AccountStatus
	receipt                     *remote.AccountReceipt
	err, queryErr               error
	cancel                      context.CancelFunc
	done                        chan struct{}
	results                     chan accountResult
}
type accountResult struct {
	generation   uint64
	capabilities cliadapter.Capabilities
	status       remote.AccountStatus
	receipt      remote.AccountReceipt
	err          error
}

func (a *App) accountBlocked() bool {
	return a.runtimeRecovery.busy || a.accountImport.busy || a.synchronizationBusy() || a.login.busy || a.logout.busy || a.setup.busy || a.wireguard.busy || a.accountDefault.busy || a.closing || a.quitting.Load()
}
func (a *App) accountOperationAvailable(method string) bool {
	return !a.accountBlocked() && !a.account.loading && !a.account.busy && remote.AccountOperationSupported(a.account.capabilities, method)
}
func (a *App) openAccountOperation() { a.page = "account" }
func (a *App) openAccountCreate() {
	a.page = "account"
	if a.account.submitted || a.account.loading || a.account.busy {
		return
	}
	if !a.remoteWorkflowAvailable("accounts.create") || a.accountBlocked() {
		return
	}
	a.account = accountPage{expected: a.remote.snapshot.Config, capabilities: a.remote.snapshot.Capabilities, action: "create", tags: "[]", form: remote.AccountCreateInput{Tool: "claude", Locale: "en_US.UTF-8", Tags: []string{}}}
}

// Saved binding history resolves the current account status; it never reconnects
// an old binding session automatically.
func (a *App) openAccountStatus(accountID string) {
	a.page = "account"
	if a.account.submitted || a.account.loading || a.account.busy {
		return
	}
	a.account = accountPage{accountID: accountID, action: "status"}
	a.refreshAccountStatus()
}
func (a *App) refreshAccountStatus() {
	p := &a.account
	if p.loading || p.busy || a.accountBlocked() || p.accountID == "" {
		return
	}
	p.review, p.bindingReview = false, false
	p.status, p.queryErr = nil, nil
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.loading = cancel, make(chan struct{}), make(chan accountResult, 1), true
	results, done, home, expected, id, generation, find := p.results, p.done, a.home, p.expected, p.accountID, p.generation, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		result := accountResult{generation: generation}
		binary, err := find("agent-remote")
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			err = client.Call(ctx, "capabilities", struct{}{}, &result.capabilities)
			if err == nil && !remote.AccountOperationSupported(result.capabilities, "accounts.status") {
				err = &cliadapter.OperationError{Code: "unsupported_method"}
			}
			if err == nil && expected.Home == "" {
				err = client.Call(ctx, "context", struct{}{}, &expected)
			}
			if err == nil {
				result.status, err = remote.GetAccountStatus(ctx, client, remote.AccountStatusInput{Expected: expected, AccountID: id})
			}
		}
		result.err = err
		results <- result
	}()
}
func (a *App) reviewAccountAction(action string) {
	p := &a.account
	if p.submitted || p.status == nil || !a.accountOperationAvailable("accounts."+action) {
		return
	}
	p.action, p.review = action, true
}
func (a *App) submitAccount() {
	p := &a.account
	if !p.review || p.submitted || !a.accountOperationAvailable("accounts."+p.action) {
		return
	}
	form := p.form
	form.Confirmed = true
	form.Expected = p.expected
	input := remote.AccountActionInput{Confirmed: true, Expected: p.expected, AccountID: p.accountID}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.busy, p.submitted = cancel, make(chan struct{}), make(chan accountResult, 1), true, true
	p.err = nil
	results, done, home, action, find := p.results, p.done, a.home, p.action, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.AccountReceipt
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			switch action {
			case "create":
				receipt, err = remote.CreateAccount(ctx, client, form)
			case "bind":
				receipt, err = remote.BindAccount(ctx, client, input)
			case "verify":
				receipt, err = remote.VerifyAccount(ctx, client, input)
			case "disable":
				receipt, err = remote.DisableAccount(ctx, client, input)
			}
		}
		results <- accountResult{receipt: receipt, err: err}
	}()
}
func (a *App) pollAccount(c *ui.Context) {
	p := &a.account
	if !p.loading && !p.busy {
		return
	}
	select {
	case result := <-p.results:
		mutation := p.busy
		p.loading, p.busy = false, false
		if !mutation && result.generation != p.generation {
			return
		}
		if mutation {
			p.err = result.err
			p.review = false
			if result.err == nil {
				p.receipt = &result.receipt
				if result.receipt.Account != nil {
					p.accountID = result.receipt.Account.ID
				}
			}
			p.status = nil
			a.invalidateRemoteAccountList()
			a.invalidateCapabilities()
		} else {
			p.queryErr = result.err
			if result.err == nil {
				p.status = &result.status
				p.expected = result.status.Context
				p.capabilities = result.capabilities
			}
		}
	default:
		c.After(50 * time.Millisecond)
	}
}
func (a *App) invalidateAccountContext() {
	p := &a.account
	p.bindingReview = false
	p.generation++
	p.status = nil
	p.review = false
	p.queryErr = fault.CLIContextChanged
	if p.loading {
		p.cancel()
	}
}
func (a *App) invalidateRemoteAccountList() {
	a.remote.generation++
	a.remote.snapshot.Accounts = nil
	a.remote.snapshot.AccountsError = fault.AccountListStale
	if a.remote.busy {
		a.remote.cancel()
	}
	a.invalidateAccountDefault()
}
func (a *App) finishAccountOperation() {
	if a.account.busy || a.account.loading {
		return
	}
	a.account = accountPage{}
	a.remote.tab = "accounts"
	a.openRemote()
}
