package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

type credentialsPage struct {
	items        []cliadapter.CredentialProfile
	expected     cliadapter.ConfigContext
	err          error
	busy, loaded bool
	action       string
	review       bool
	accountID    string
	profileID    string
	form         remote.CredentialCreateInput
	gitUserName  string
	gitUserEmail string
	receipt      *remote.CredentialMutationReceipt
	mutationBusy bool
	generation   uint64
	cancel       context.CancelFunc
	done         chan struct{}
	results      chan credentialsResult
}

func (a *App) credentialsBlocked() bool {
	return a.credentials.busy || a.credentials.mutationBusy || a.closing || a.quitting.Load() || a.login.busy || a.logout.busy || a.setup.busy || a.wireguard.busy
}

func (a *App) credentialsMutationAvailable(method string) bool {
	caps, _, ok := a.credentialsSource()
	return ok && remote.CredentialMutationSupported(caps, method)
}

func (a *App) reviewCredentialCreate() {
	p := &a.credentials
	caps, _, ok := a.credentialsSource()
	if a.credentialsBlocked() || !ok || !remote.CredentialMutationSupported(caps, remote.CredentialsCreateMethod) {
		return
	}
	p.form.Expected = p.expected
	p.form.GitUserName, p.form.GitUserEmail = optionalCredentialText(p.gitUserName), optionalCredentialText(p.gitUserEmail)
	p.form.Confirmed = true
	if err := remote.ValidateCredentialCreateInput(p.form); err != nil {
		p.err = err
		return
	}
	p.err, p.action, p.review = nil, "create", true
}

func optionalCredentialText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func (a *App) submitCredentialMutation() {
	p := &a.credentials
	if !p.review || a.credentialsBlocked() {
		return
	}
	form := p.form
	form.Confirmed = true
	form.Expected = p.expected
	accountID, profileID, action := p.accountID, p.profileID, p.action
	p.review, p.busy, p.mutationBusy, p.err = false, true, true, nil
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done, p.results = make(chan struct{}), make(chan credentialsResult, 1)
	home, expected, generation, find := a.home, p.expected, p.generation, a.findExecutable
	go func() {
		defer close(p.done)
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.CredentialMutationReceipt
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			switch action {
			case "create":
				receipt, err = remote.CreateCredentialProfile(ctx, client, form)
			case "bind":
				receipt, err = remote.BindCredentialProfile(ctx, client, remote.CredentialBindingInput{Confirmed: true, Expected: expected, AccountID: accountID, ProfileID: profileID})
			case "unbind":
				receipt, err = remote.UnbindCredentialProfile(ctx, client, remote.CredentialUnbindInput{Confirmed: true, Expected: expected, AccountID: accountID})
			}
		}
		p.results <- credentialsResult{generation: generation, mutation: true, receipt: receipt, err: err}
	}()
}

type credentialsResult struct {
	generation uint64
	profiles   remote.CredentialProfiles
	receipt    remote.CredentialMutationReceipt
	mutation   bool
	err        error
}

func (a *App) openCredentials() {
	a.page = "credentials"
	if a.credentials.busy || a.credentials.loaded {
		return
	}
	a.refreshCredentials()
}

func (a *App) refreshCredentials() {
	p := &a.credentials
	if p.busy || a.closing || a.quitting.Load() {
		return
	}
	caps, expected, ok := a.credentialsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.CredentialsListMethod, false, nil).CanStart() {
		p.err = &cliadapter.OperationError{Code: "unsupported_method"}
		return
	}
	p.busy, p.err = true, nil
	p.expected = expected
	p.done = make(chan struct{})
	p.results = make(chan credentialsResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	results, done, home, expected, generation, find := p.results, p.done, a.home, p.expected, p.generation, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var profiles remote.CredentialProfiles
		if err == nil {
			profiles, err = remote.ListCredentialProfiles(ctx, cliadapter.Client{Binary: binary, Home: home}, expected)
		}
		results <- credentialsResult{generation: generation, profiles: profiles, err: err}
	}()
}

func (a *App) credentialsSource() (cliadapter.Capabilities, cliadapter.ConfigContext, bool) {
	if a.remote.loaded && !a.remote.busy && a.remote.err == nil {
		return a.remote.snapshot.Capabilities, a.remote.snapshot.Config, true
	}
	if a.capabilities.loaded && !a.capabilities.busy && a.capabilities.snapshot.ProtocolError == nil {
		return a.capabilities.snapshot.Capabilities, a.capabilities.snapshot.Config, true
	}
	return cliadapter.Capabilities{}, cliadapter.ConfigContext{}, false
}

func (a *App) pollCredentials(c *ui.Context) {
	p := &a.credentials
	if (!p.busy && !p.mutationBusy) || p.results == nil {
		return
	}
	select {
	case result := <-p.results:
		p.busy = false
		if result.mutation {
			p.mutationBusy = false
			if result.generation != p.generation {
				return
			}
			// Drop all editable credential metadata as soon as the CLI returns.
			// The CLI owns device credentials; the desktop keeps no form copy.
			p.form = remote.CredentialCreateInput{}
			p.accountID, p.profileID, p.action = "", "", ""
			p.gitUserName, p.gitUserEmail = "", ""
			p.err = result.err
			if result.err == nil {
				p.receipt = &result.receipt
				p.review = false
				p.loaded = false
				a.refreshCredentials()
			}
			return
		}
		if result.generation != p.generation {
			return
		}
		p.err = result.err
		if result.err == nil {
			p.items, p.expected, p.loaded = result.profiles.Items, result.profiles.Context, true
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) invalidateCredentials() {
	p := &a.credentials
	p.generation++
	p.items, p.expected, p.err, p.loaded = nil, cliadapter.ConfigContext{}, fault.CLIContextChanged, false
	p.form = remote.CredentialCreateInput{}
	p.accountID, p.profileID, p.action = "", "", ""
	p.gitUserName, p.gitUserEmail = "", ""
	if (p.busy || p.mutationBusy) && p.cancel != nil {
		p.cancel()
	}
}

func (a *App) cancelCredentialMutation() {
	p := &a.credentials
	if p.cancel != nil {
		p.cancel()
	}
	p.form.GitUserName, p.form.GitUserEmail = nil, nil
	p.gitUserName, p.gitUserEmail = "", ""
}
