package app

import (
	"context"
	"strconv"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type egoBrowserDeleteResult struct {
	plan    remote.EgoBrowserDeletePlan
	receipt remote.EgoBrowserDeleteReceipt
	err     error
	control bool
}

func egoBrowserDeleteBindingAllowed(status string) bool {
	switch status {
	case "stopped", "expired", "failed", "revoked":
		return true
	default:
		return false
	}
}

func (a *App) planEgoBrowserDelete(target, id string, generation, bindingGeneration uint64) {
	p := &a.egoBrowser
	if p.deleteBusy || p.loading || !p.loaded || a.closing || a.quitting.Load() || p.snapshot.Context.ServerURL == nil {
		return
	}
	p.deletePlan, p.deleteReceipt, p.deleteErr = nil, nil, nil
	p.deleteBusy, p.deleteResults = true, make(chan egoBrowserDeleteResult, 1)
	results, home, find, expected := p.deleteResults, a.home, a.findExecutable, p.snapshot.Context
	go func() {
		binary, err := find("agent-remote")
		var plan remote.EgoBrowserDeletePlan
		if err == nil {
			plan, err = remote.LoadEgoBrowserDeletePlan(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserDeletePlanInput{
				Expected: expected, Target: target, ID: id, Generation: generation, BindingGeneration: bindingGeneration,
			})
		}
		results <- egoBrowserDeleteResult{plan: plan, err: err}
	}()
}

func (a *App) confirmEgoBrowserDelete() {
	p := &a.egoBrowser
	if p.deletePlan == nil || p.deleteBusy || !p.deletePlan.Allowed || a.closing || a.quitting.Load() {
		return
	}
	plan := *p.deletePlan
	p.deleteBusy, p.deleteErr = true, nil
	p.deleteResults = make(chan egoBrowserDeleteResult, 1)
	results, home, find := p.deleteResults, a.home, a.findExecutable
	go func() {
		binary, err := find("agent-remote")
		var receipt remote.EgoBrowserDeleteReceipt
		if err == nil {
			receipt, err = remote.ControlEgoBrowserDelete(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.EgoBrowserDeleteControlInput{
				Confirmed: true, Expected: plan.Context, Target: plan.Target, ID: plan.ID, Generation: plan.Generation, BindingGeneration: plan.BindingGeneration,
			})
		}
		results <- egoBrowserDeleteResult{receipt: receipt, err: err, control: true}
	}()
}

func (a *App) cancelEgoBrowserDeleteReview() {
	a.egoBrowser.deletePlan = nil
	a.egoBrowser.deleteErr = nil
}

func (a *App) egoBrowserDeletePanel(c *ui.Context) {
	p := &a.egoBrowser
	if p.deleteBusy {
		ui.Text(c, locales.EgoBrowserDeleteWorking()).TextColor(c.Theme().TextMuted)
	} else if p.deleteErr != nil {
		a.errorText(c, p.deleteErr, presentation.Error(p.deleteErr))
	} else if p.deletePlan != nil {
		a.card(c, ui.Column(c).FillWidth().Padding(12).Gap(6)).Children(func() {
			ui.Text(c, locales.EgoBrowserDeleteReview()).Bold()
			ui.Text(c, locales.EgoBrowserDeleteReviewTarget(p.deletePlan.Target, p.deletePlan.ID, strconv.FormatUint(p.deletePlan.Generation, 10)))
			if p.deletePlan.Allowed {
				ui.Text(c, locales.EgoBrowserDeleteReviewAllowed()).TextColor(c.Theme().Danger)
				ui.Row(c).Gap(8).Children(func() {
					ui.Button(c, locales.Cancel()).OnClick(a.cancelEgoBrowserDeleteReview)
					ui.Button(c, locales.EgoBrowserDeleteConfirm()).OnClick(a.confirmEgoBrowserDelete)
				})
			} else {
				ui.Text(c, locales.EgoBrowserDeleteReviewDenied()).TextColor(c.Theme().Danger)
				ui.Button(c, locales.Cancel()).OnClick(a.cancelEgoBrowserDeleteReview)
			}
		})
	} else if p.deleteReceipt != nil {
		ui.Text(c, locales.EgoBrowserDeleteAccepted(p.deleteReceipt.Target, p.deleteReceipt.ID)).TextColor(c.Theme().Success)
		if p.deleteReceipt.LocalCleanupComplete != nil {
			ui.Text(c, locales.DeviceLocalCleanup(boolText(*p.deleteReceipt.LocalCleanupComplete))).FontSize(12)
		}
	}
}
