package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type egoBrowserPage struct {
	snapshot          remote.EgoBrowserStatus
	candidates        []remote.EgoBrowserCandidate
	candidatesLoading bool
	candidatesErr     error
	candidatesResults chan egoBrowserCandidatesResult
	requests          []remote.EgoBrowserRequest
	selectedBinding   string
	loaded            bool
	loading           bool
	requestsLoading   bool
	requestBusy       bool
	err               error
	requestErr        error
	cancel            context.CancelFunc
	results           chan egoBrowserResult
	requestResults    chan egoBrowserRequestsResult
	bindingPlan       *remote.EgoBrowserBindingPlan
	bindingReceipt    *remote.EgoBrowserBindingControlReceipt
	bindingBusy       bool
	bindingErr        error
	bindingResults    chan egoBrowserBindingResult
	deletePlan        *remote.EgoBrowserDeletePlan
	deleteReceipt     *remote.EgoBrowserDeleteReceipt
	deleteBusy        bool
	deleteErr         error
	deleteResults     chan egoBrowserDeleteResult
	claimPlan         *remote.EgoBrowserClaimPlan
	claimReceipt      *remote.EgoBrowserClaimReceipt
	claimBusy         bool
	claimErr          error
	claimResults      chan egoBrowserClaimResult
}

type egoBrowserResult struct {
	snapshot remote.EgoBrowserStatus
	err      error
}

type egoBrowserRequestsResult struct {
	items []remote.EgoBrowserRequest
	err   error
}

type egoBrowserClaimResult struct {
	plan    remote.EgoBrowserClaimPlan
	receipt remote.EgoBrowserClaimReceipt
	err     error
	control bool
}

type egoBrowserCandidatesResult struct {
	items []remote.EgoBrowserCandidate
	err   error
}

type egoBrowserBindingResult struct {
	plan    remote.EgoBrowserBindingPlan
	receipt remote.EgoBrowserBindingControlReceipt
	err     error
	control bool
}

func (a *App) openEgoBrowser() {
	a.page = "ego-browser"
	if !a.egoBrowser.loading {
		a.loadEgoBrowser()
	}
}

func (a *App) loadEgoBrowser() {
	p := &a.egoBrowser
	if a.closing || a.quitting.Load() || p.loading {
		return
	}
	p.snapshot, p.err, p.loaded = remote.EgoBrowserStatus{}, nil, false
	p.candidates, p.candidatesErr, p.claimPlan, p.claimReceipt = nil, nil, nil, nil
	p.candidatesLoading = true
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.results, p.loading = cancel, make(chan egoBrowserResult, 1), true
	results, home, find := p.results, a.home, a.findExecutable
	go func() {
		binary, err := find("agent-remote")
		var snapshot remote.EgoBrowserStatus
		if err == nil {
			snapshot, err = remote.LoadEgoBrowserStatus(ctx, cliadapter.Client{Binary: binary, Home: home})
		}
		results <- egoBrowserResult{snapshot: snapshot, err: err}
	}()
}

func (a *App) pollEgoBrowser(c *ui.Context) {
	p := &a.egoBrowser
	if p.candidatesLoading && p.candidatesResults != nil {
		select {
		case result := <-p.candidatesResults:
			p.candidatesLoading, p.candidatesErr, p.candidates = false, result.err, result.items
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if !p.loading && !p.requestsLoading && !p.requestBusy && !p.bindingBusy && !p.deleteBusy {
		if !p.claimBusy {
			return
		}
	}
	if p.requestsLoading {
		select {
		case result := <-p.requestResults:
			p.requestsLoading, p.requestErr = false, result.err
			if result.err == nil {
				p.requests = result.items
			}
		default:
			c.After(50 * time.Millisecond)
		}
		return
	}
	if p.requestBusy {
		select {
		case result := <-p.requestResults:
			p.requestBusy, p.requestErr = false, result.err
			if result.err == nil {
				p.requests = nil
			}
		default:
			c.After(50 * time.Millisecond)
		}
		return
	}
	if p.bindingBusy {
		select {
		case result := <-p.bindingResults:
			p.bindingBusy, p.bindingErr = false, result.err
			if result.err == nil {
				if result.control {
					p.bindingReceipt = &result.receipt
					p.bindingPlan = nil
				} else {
					p.bindingPlan = &result.plan
					p.bindingReceipt = nil
				}
			}
		default:
			c.After(50 * time.Millisecond)
		}
		return
	}
	if p.deleteBusy {
		select {
		case result := <-p.deleteResults:
			p.deleteBusy, p.deleteErr = false, result.err
			if result.err == nil {
				if result.control {
					p.deleteReceipt = &result.receipt
					p.deletePlan = nil
				} else {
					p.deletePlan = &result.plan
					p.deleteReceipt = nil
				}
			}
		default:
			c.After(50 * time.Millisecond)
		}
		return
	}
	if p.claimBusy {
		select {
		case result := <-p.claimResults:
			p.claimBusy, p.claimErr = false, result.err
			if result.err == nil {
				if result.control {
					p.claimReceipt = &result.receipt
					p.claimPlan = nil
				} else {
					p.claimPlan = &result.plan
					p.claimReceipt = nil
				}
			}
		default:
			c.After(50 * time.Millisecond)
		}
		return
	}
	select {
	case result := <-p.results:
		p.loading, p.loaded, p.err = false, true, result.err
		if result.err == nil {
			p.snapshot = result.snapshot
			p.candidatesLoading = true
			p.candidatesErr = nil
			p.candidates = nil
			p.candidatesResults = make(chan egoBrowserCandidatesResult, 1)
			results, expected, home, find := p.candidatesResults, p.snapshot.Context, a.home, a.findExecutable
			go func() {
				binary, err := find("agent-remote")
				var candidates remote.EgoBrowserCandidates
				if err == nil {
					candidates, err = remote.LoadEgoBrowserCandidates(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserCandidatesInput{Expected: expected})
				}
				results <- egoBrowserCandidatesResult{items: candidates.Items, err: err}
			}()
		}
		if p.cancel != nil {
			p.cancel()
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) planEgoBrowserClaim(candidate remote.EgoBrowserCandidate) {
	p := &a.egoBrowser
	if p.claimBusy || p.loading || !p.loaded || !candidate.Controllable || candidate.BindingID != nil || candidate.CurrentEgoBrowserDeviceID != nil {
		return
	}
	p.claimPlan, p.claimReceipt, p.claimErr = nil, nil, nil
	p.claimBusy, p.claimResults = true, make(chan egoBrowserClaimResult, 1)
	results, home, find, expected := p.claimResults, a.home, a.findExecutable, p.snapshot.Context
	go func() {
		binary, err := find("agent-remote")
		var plan remote.EgoBrowserClaimPlan
		if err == nil {
			plan, err = remote.LoadEgoBrowserClaimPlan(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserClaimPlanInput{Expected: expected, ToolSessionID: candidate.ToolSessionID, CandidateFingerprint: egoBrowserCandidateFingerprint(candidate)})
		}
		results <- egoBrowserClaimResult{plan: plan, err: err}
	}()
}

func (a *App) confirmEgoBrowserClaim() {
	p := &a.egoBrowser
	if p.claimPlan == nil || p.claimBusy || !p.claimPlan.Allowed || a.closing || a.quitting.Load() {
		return
	}
	plan := *p.claimPlan
	p.claimBusy, p.claimErr = true, nil
	p.claimResults = make(chan egoBrowserClaimResult, 1)
	results, home, find := p.claimResults, a.home, a.findExecutable
	go func() {
		binary, err := find("agent-remote")
		var receipt remote.EgoBrowserClaimReceipt
		if err == nil {
			receipt, err = remote.ControlEgoBrowserClaim(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserClaimControlInput{Confirmed: true, Expected: plan.Context, ToolSessionID: plan.Candidate.ToolSessionID, CandidateFingerprint: plan.CandidateFingerprint})
		}
		results <- egoBrowserClaimResult{receipt: receipt, err: err, control: true}
	}()
}

func (a *App) cancelEgoBrowserClaimReview() {
	a.egoBrowser.claimPlan = nil
	a.egoBrowser.claimErr = nil
}

func egoBrowserCandidateFingerprint(candidate remote.EgoBrowserCandidate) string {
	var deviceID, deviceName, binding string
	if candidate.CurrentEgoBrowserDeviceID != nil {
		deviceID = *candidate.CurrentEgoBrowserDeviceID
	}
	if candidate.CurrentEgoBrowserDeviceName != nil {
		deviceName = *candidate.CurrentEgoBrowserDeviceName
	}
	if candidate.BindingID != nil {
		binding = *candidate.BindingID
	}
	return strings.Join([]string{candidate.ToolSessionID, candidate.ToolType, candidate.ToolAccountID, candidate.WorkspaceID, candidate.NodeID, candidate.Status, candidate.RuntimeBackend, candidate.ProjectKey, candidate.DisplayName, deviceID, deviceName, binding, strconv.FormatBool(candidate.Controllable)}, "|")
}

func (a *App) planEgoBrowserBindingAction(binding remote.EgoBrowserBinding, action string) {
	p := &a.egoBrowser
	if p.bindingBusy || p.loading || !p.loaded || a.closing || a.quitting.Load() || p.snapshot.Context.ServerURL == nil {
		return
	}
	p.bindingPlan, p.bindingReceipt, p.bindingErr = nil, nil, nil
	p.bindingBusy, p.bindingResults = true, make(chan egoBrowserBindingResult, 1)
	results, home, find, expected := p.bindingResults, a.home, a.findExecutable, p.snapshot.Context
	go func() {
		binary, err := find("agent-remote")
		var plan remote.EgoBrowserBindingPlan
		if err == nil {
			if binding.BindingGeneration == nil || *binding.BindingGeneration == 0 {
				err = fault.CLIProtocol
			} else {
				plan, err = remote.LoadEgoBrowserBindingPlan(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserBindingPlanInput{Expected: expected, BindingID: binding.ID, BindingGeneration: *binding.BindingGeneration, Generation: binding.Generation, Action: action})
			}
		}
		results <- egoBrowserBindingResult{plan: plan, err: err}
	}()
}

func (a *App) confirmEgoBrowserBindingAction() {
	p := &a.egoBrowser
	if p.bindingPlan == nil || p.bindingBusy || !p.bindingPlan.Allowed || a.closing || a.quitting.Load() {
		return
	}
	plan := *p.bindingPlan
	p.bindingBusy, p.bindingErr = true, nil
	p.bindingResults = make(chan egoBrowserBindingResult, 1)
	results, home, find, expected := p.bindingResults, a.home, a.findExecutable, plan.Context
	go func() {
		binary, err := find("agent-remote")
		var receipt remote.EgoBrowserBindingControlReceipt
		if err == nil {
			receipt, err = remote.ControlEgoBrowserBinding(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserBindingControlInput{Confirmed: true, Expected: expected, BindingID: plan.Binding.ID, BindingGeneration: plan.BindingGeneration, Generation: plan.Generation, Action: plan.Action})
		}
		results <- egoBrowserBindingResult{receipt: receipt, err: err, control: true}
	}()
}

func (a *App) cancelEgoBrowserBindingReview() {
	a.egoBrowser.bindingPlan = nil
	a.egoBrowser.bindingErr = nil
}

func (a *App) loadEgoBrowserRequests(bindingID string) {
	p := &a.egoBrowser
	if p.loading || p.requestsLoading || p.requestBusy || a.closing || a.quitting.Load() || !p.loaded {
		return
	}
	p.selectedBinding, p.requests, p.requestErr = bindingID, nil, nil
	p.requestsLoading, p.requestResults = true, make(chan egoBrowserRequestsResult, 1)
	results, home, find, expected := p.requestResults, a.home, a.findExecutable, p.snapshot.Context
	go func() {
		binary, err := find("agent-remote")
		var list remote.EgoBrowserRequests
		if err == nil {
			list, err = remote.LoadEgoBrowserRequests(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserRequestListInput{Expected: expected, BindingID: bindingID})
		}
		results <- egoBrowserRequestsResult{items: list.Items, err: err}
	}()
}

func (a *App) cancelEgoBrowserRequest(item remote.EgoBrowserRequest) {
	p := &a.egoBrowser
	if p.requestBusy || p.requestsLoading || p.loading || !p.loaded || p.snapshot.Context.ServerURL == nil {
		return
	}
	p.requestBusy, p.requestErr, p.requestResults = true, nil, make(chan egoBrowserRequestsResult, 1)
	results, home, find, expected, bindingID := p.requestResults, a.home, a.findExecutable, p.snapshot.Context, p.selectedBinding
	go func() {
		binary, err := find("agent-remote")
		if err == nil {
			_, err = remote.CancelEgoBrowserRequest(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserRequestCancelInput{Confirmed: true, Expected: expected, BindingID: bindingID, RequestID: item.ID, BindingGeneration: item.BindingGeneration, Generation: item.Generation, Sequence: item.Sequence})
		}
		results <- egoBrowserRequestsResult{err: err}
	}()
}

func (a *App) cancelEgoBrowserWait() {
	if a.egoBrowser.cancel != nil {
		a.egoBrowser.cancel()
	}
}

func (a *App) egoBrowserView(c *ui.Context) {
	p := &a.egoBrowser
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, locales.EgoBrowserTitle()).FontSize(24).Bold().Grow(1)
			ui.Button(c, locales.ReturnWorkbench()).OnClick(func() { a.page = "" })
		})
		ui.Text(c, locales.EgoBrowserHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			switch {
			case p.loading:
				ui.Text(c, locales.EgoBrowserLoading())
			case p.err != nil:
				ui.Text(c, presentation.Error(p.err)).TextColor(c.Theme().Danger)
			case p.loaded:
				a.egoBrowserDetails(c, &p.snapshot)
				a.egoBrowserDeletePanel(c)
			}
		})
		ui.Row(c).Justify(ui.End).Children(func() {
			if p.loading {
				ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.cancelEgoBrowserWait)
			} else {
				ui.Button(c, locales.EgoBrowserRefresh()).OnClick(a.loadEgoBrowser)
			}
		})
	})
}

func (a *App) egoBrowserDetails(c *ui.Context, status *remote.EgoBrowserStatus) {
	ui.Column(c).FillWidth().Padding(14).Gap(8).Background(c.Theme().Surface).Radius(8).Children(func() {
		ui.Text(c, locales.EgoBrowserLocalState(status.Local.Installation, status.Local.Store)).Bold()
		ui.Text(c, locales.EgoBrowserPolicy(status.Policy.Enrollment, status.Policy.Execution)).FontSize(12)
		if status.Context.ServerURL != nil {
			ui.Text(c, locales.EgoBrowserServer(*status.Context.ServerURL)).FontSize(12)
		}
		ui.Text(c, locales.EgoBrowserDeviceCount(strconv.Itoa(len(status.Devices)))).FontSize(12)
		for _, device := range status.Devices {
			device := device
			ui.Column(c.Key("ego-device-" + device.ID)).Gap(2).Padding(8).Background(c.Theme().SurfaceHover).Radius(6).Children(func() {
				ui.Text(c, locales.EgoBrowserDevice(device.ID, device.Status)).Bold()
				ui.Text(c, locales.EgoBrowserVersion(egoVersion(device.BridgeVersion), egoVersion(device.RuntimeVersion))).FontSize(12)
				if device.Status == "revoked" {
					ui.Button(c, locales.EgoBrowserDeleteDevice()).Disabled(a.egoBrowser.deleteBusy).OnClick(func() {
						a.planEgoBrowserDelete("device", device.ID, device.Generation, 0)
					})
				}
			})
		}
		ui.Text(c, locales.EgoBrowserBindingCount(strconv.Itoa(len(status.Bindings)))).FontSize(12)
		for _, binding := range status.Bindings {
			binding := binding
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, locales.EgoBrowserBinding(binding.ID, binding.Status, binding.LeaseHealth)).FontSize(12).Grow(1)
				ui.Button(c, locales.EgoBrowserRequestsOpen()).OnClick(func() { a.loadEgoBrowserRequests(binding.ID) })
				for _, action := range []string{"pause", "resume", "stop", "revoke"} {
					ui.Button(c, locales.EgoBrowserBindingAction(action)).Disabled(a.egoBrowser.bindingBusy).OnClick(func() { a.planEgoBrowserBindingAction(binding, action) })
				}
				if egoBrowserDeleteBindingAllowed(binding.Status) {
					ui.Button(c, locales.EgoBrowserDeleteBinding()).Disabled(a.egoBrowser.deleteBusy).OnClick(func() {
						generation := binding.Generation
						bindingGeneration := uint64(0)
						if binding.BindingGeneration != nil {
							bindingGeneration = *binding.BindingGeneration
						}
						a.planEgoBrowserDelete("binding", binding.ID, generation, bindingGeneration)
					})
				}
			})
		}
		if p := &a.egoBrowser; p.bindingBusy {
			ui.Text(c, locales.EgoBrowserBindingWorking()).TextColor(c.Theme().TextMuted)
		} else if p.bindingErr != nil {
			ui.Text(c, presentation.Error(p.bindingErr)).TextColor(c.Theme().Danger)
		} else if p.bindingPlan != nil {
			ui.Column(c).FillWidth().Padding(12).Gap(6).Background(c.Theme().SurfaceHover).Radius(8).Children(func() {
				ui.Text(c, locales.EgoBrowserBindingReview()).Bold()
				ui.Text(c, locales.EgoBrowserBindingReviewTarget(p.bindingPlan.Binding.ID, p.bindingPlan.Action))
				if p.bindingPlan.Allowed {
					ui.Text(c, locales.EgoBrowserBindingReviewAllowed()).TextColor(a.tokens().AccentInk)
					ui.Row(c).Gap(8).Children(func() {
						ui.Button(c, locales.Cancel()).OnClick(a.cancelEgoBrowserBindingReview)
						ui.PrimaryButton(c, locales.EgoBrowserBindingConfirm()).OnClick(a.confirmEgoBrowserBindingAction)
					})
				} else {
					ui.Text(c, locales.EgoBrowserBindingReviewDenied()).TextColor(c.Theme().Danger)
					ui.Button(c, locales.Cancel()).OnClick(a.cancelEgoBrowserBindingReview)
				}
			})
		}
		if p := &a.egoBrowser; p.bindingReceipt != nil {
			ui.Text(c, locales.EgoBrowserBindingAccepted(p.bindingReceipt.Action, p.bindingReceipt.Binding.Status)).TextColor(c.Theme().Success)
		}
		ui.Text(c, locales.EgoBrowserCandidatesTitle()).Bold()
		p := &a.egoBrowser
		switch {
		case p.candidatesLoading:
			ui.Text(c, locales.EgoBrowserCandidatesLoading()).TextColor(c.Theme().TextMuted)
		case p.candidatesErr != nil:
			ui.Text(c, presentation.Error(p.candidatesErr)).TextColor(c.Theme().Danger)
		case len(p.candidates) == 0:
			ui.Text(c, locales.EgoBrowserCandidatesEmpty()).TextColor(c.Theme().TextMuted)
		default:
			for _, candidate := range p.candidates {
				candidate := candidate
				ui.Row(c.Key("ego-candidate-" + candidate.ToolSessionID)).Gap(8).Children(func() {
					ui.Text(c, locales.EgoBrowserCandidate(candidate.DisplayName, candidate.Status, candidate.NodeID)).Grow(1)
					ui.Button(c, locales.EgoBrowserCandidateClaim()).Disabled(!candidate.Controllable || candidate.BindingID != nil || candidate.CurrentEgoBrowserDeviceID != nil || p.claimBusy).OnClick(func() { a.planEgoBrowserClaim(candidate) })
				})
			}
		}
		if p.claimBusy {
			ui.Text(c, locales.EgoBrowserClaimWorking()).TextColor(c.Theme().TextMuted)
		} else if p.claimErr != nil {
			ui.Text(c, presentation.Error(p.claimErr)).TextColor(c.Theme().Danger)
		} else if p.claimPlan != nil {
			ui.Column(c).FillWidth().Padding(12).Gap(6).Background(c.Theme().SurfaceHover).Radius(8).Children(func() {
				ui.Text(c, locales.EgoBrowserClaimReview()).Bold()
				ui.Text(c, locales.EgoBrowserClaimReviewTarget(p.claimPlan.Candidate.ToolSessionID, p.claimPlan.Candidate.DisplayName))
				ui.Text(c, locales.EgoBrowserClaimReviewWarning()).TextColor(c.Theme().Danger)
				ui.Row(c).Gap(8).Children(func() {
					ui.Button(c, locales.Cancel()).OnClick(a.cancelEgoBrowserClaimReview)
					ui.PrimaryButton(c, locales.EgoBrowserClaimConfirm()).OnClick(a.confirmEgoBrowserClaim)
				})
			})
		}
		if p.claimReceipt != nil {
			ui.Text(c, locales.EgoBrowserClaimAccepted(p.claimReceipt.Candidate.ToolSessionID)).TextColor(c.Theme().Success)
		}
		if p := &a.egoBrowser; p.selectedBinding != "" {
			ui.Text(c, locales.EgoBrowserRequestsTitle(p.selectedBinding)).Bold()
			switch {
			case p.requestsLoading:
				ui.Text(c, locales.EgoBrowserRequestsLoading())
			case p.requestErr != nil:
				ui.Text(c, presentation.Error(p.requestErr)).TextColor(c.Theme().Danger)
			case len(p.requests) == 0:
				ui.Text(c, locales.EgoBrowserRequestsEmpty()).TextColor(c.Theme().TextMuted)
			default:
				for _, item := range p.requests {
					request := item
					ui.Row(c.Key("ego-request-" + item.ID)).Gap(8).Children(func() {
						ui.Text(c, locales.EgoBrowserRequest(item.ID, item.Status, item.MessageType, strconv.FormatUint(item.PayloadBytes, 10))).Grow(1)
						ui.Button(c, locales.EgoBrowserRequestCancel()).Disabled(p.requestBusy).OnClick(func() { a.cancelEgoBrowserRequest(request) })
					})
				}
			}
		}
	})
}

func egoVersion(value *string) string {
	if value == nil || *value == "" {
		return locales.EgoBrowserUnknown()
	}
	return *value
}
