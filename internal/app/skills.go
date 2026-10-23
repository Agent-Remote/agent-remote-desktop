package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type skillsPage struct {
	snapshot            remote.SkillCatalog
	err                 error
	busy                bool
	loaded              bool
	filter              string
	generation          uint64
	cancel              context.CancelFunc
	results             chan skillsResult
	review              *skillsChangeReview
	receipt             *remote.SkillChangeReceipt
	changeErr           error
	changeBusy          bool
	changeCancel        context.CancelFunc
	changeResults       chan skillsChangeResult
	conflictAccount     string
	conflicts           remote.SkillStateConflicts
	conflictsLoaded     bool
	conflictsBusy       bool
	conflictsErr        error
	conflictsCancel     context.CancelFunc
	conflictsResults    chan skillsConflictsResult
	historyAccount      string
	historySkill        string
	historyScope        string
	history             remote.SkillStateList
	historyLoaded       bool
	historyBusy         bool
	historyErr          error
	historyCancel       context.CancelFunc
	historyResults      chan skillsHistoryResult
	stateInfo           *remote.SkillStateInfo
	stateInfoSelector   string
	stateInfoBusy       bool
	stateInfoErr        error
	stateInfoCancel     context.CancelFunc
	stateInfoResults    chan skillsStateInfoResult
	diff                *remote.SkillStateDiff
	diffCheckpoint      string
	diffBusy            bool
	diffErr             error
	diffCancel          context.CancelFunc
	diffResults         chan skillsDiffResult
	exportCheckpoint    *remote.SkillStateCheckpoint
	info                *remote.SkillInfo
	infoSelector        string
	infoBusy            bool
	infoErr             error
	infoCancel          context.CancelFunc
	infoResults         chan skillsInfoResult
	retryOperationID    string
	retryPlan           *remote.SkillRetryPlan
	retryReceipt        *remote.SkillRetryReceipt
	retryBusy           bool
	retryErr            error
	retryCancel         context.CancelFunc
	retryResults        chan skillsRetryResult
	retryControlBusy    bool
	retryControlCancel  context.CancelFunc
	retryControlResults chan skillsRetryControlResult
}

type skillsChangeReview struct {
	Skill      string
	Name       string
	Action     string
	Expected   cliadapter.ConfigContext
	Generation int64
}

type skillsChangeResult struct {
	receipt remote.SkillChangeReceipt
	err     error
}

type skillsResult struct {
	generation uint64
	catalog    remote.SkillCatalog
	err        error
}

type skillsConflictsResult struct {
	conflicts remote.SkillStateConflicts
	err       error
}

type skillsHistoryResult struct {
	history remote.SkillStateList
	err     error
}

type skillsStateInfoResult struct {
	selector string
	info     remote.SkillStateInfo
	err      error
}

type skillsDiffResult struct {
	diff remote.SkillStateDiff
	err  error
}

type skillsInfoResult struct {
	selector string
	info     remote.SkillInfo
	err      error
}

type skillsRetryResult struct {
	plan remote.SkillRetryPlan
	err  error
}

type skillsRetryControlResult struct {
	receipt remote.SkillRetryReceipt
	err     error
}

func (a *App) openSkills() {
	a.page = "skills"
	if a.skills.historyScope == "" {
		a.skills.historyScope = "account-directory"
	}
	if !a.skills.loaded && !a.skills.busy {
		if _, _, ok := a.skillsSource(); ok {
			a.refreshSkills()
		} else {
			a.refreshCapabilities()
		}
	}
}

func (a *App) refreshSkills() {
	p := &a.skills
	if p.busy || a.closing || a.quitting.Load() {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsListMethod, false, nil).CanStart() {
		p.err = &cliadapter.OperationError{Code: "unsupported_method"}
		p.loaded = false
		return
	}
	p.busy, p.err = true, nil
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.results = make(chan skillsResult, 1)
	home, generation, find := a.home, p.generation, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var catalog remote.SkillCatalog
		if err == nil {
			catalog, err = remote.ListSkills(ctx, cliadapter.Client{Binary: binary, Home: home}, expected, remote.SkillListOptions{Effective: true, IncludeSystem: true})
		}
		p.results <- skillsResult{generation: generation, catalog: catalog, err: err}
	}()
}

func (a *App) skillsSource() (cliadapter.Capabilities, cliadapter.ConfigContext, bool) {
	if a.remote.loaded && !a.remote.busy && a.remote.err == nil {
		return a.remote.snapshot.Capabilities, a.remote.snapshot.Config, true
	}
	if a.capabilities.loaded && !a.capabilities.busy && a.capabilities.snapshot.ProtocolError == nil {
		return a.capabilities.snapshot.Capabilities, a.capabilities.snapshot.Config, true
	}
	return cliadapter.Capabilities{}, cliadapter.ConfigContext{}, false
}

func (a *App) pollSkills(c *ui.Context) {
	p := &a.skills
	if p.busy && p.results != nil {
		select {
		case result := <-p.results:
			p.busy = false
			if result.generation == p.generation {
				p.snapshot, p.err, p.loaded = result.catalog, result.err, result.err == nil
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if p.changeBusy && p.changeResults != nil {
		select {
		case result := <-p.changeResults:
			p.changeBusy = false
			p.changeCancel = nil
			p.changeErr = result.err
			if result.err == nil {
				p.receipt = &result.receipt
				p.review = nil
			} else {
				p.receipt = nil
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if p.conflictsBusy && p.conflictsResults != nil {
		select {
		case result := <-p.conflictsResults:
			p.conflictsBusy = false
			p.conflictsCancel = nil
			p.conflictsErr = result.err
			if result.err == nil {
				p.conflicts = result.conflicts
				p.conflictsLoaded = true
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if p.historyBusy && p.historyResults != nil {
		select {
		case result := <-p.historyResults:
			p.historyBusy = false
			p.historyCancel = nil
			p.historyErr = result.err
			if result.err == nil {
				p.history = result.history
				p.historyLoaded = true
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if p.infoBusy && p.infoResults != nil {
		select {
		case result := <-p.infoResults:
			p.infoBusy = false
			p.infoCancel = nil
			p.infoErr = result.err
			if result.err == nil && result.selector == p.infoSelector {
				p.info = &result.info
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if p.stateInfoBusy && p.stateInfoResults != nil {
		select {
		case result := <-p.stateInfoResults:
			p.stateInfoBusy = false
			p.stateInfoCancel = nil
			p.stateInfoErr = result.err
			if result.err == nil && result.selector == p.stateInfoSelector {
				p.stateInfo = &result.info
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if p.diffBusy && p.diffResults != nil {
		select {
		case result := <-p.diffResults:
			p.diffBusy = false
			p.diffCancel = nil
			p.diffErr = result.err
			if result.err == nil {
				p.diff = &result.diff
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if p.retryBusy && p.retryResults != nil {
		select {
		case result := <-p.retryResults:
			p.retryBusy = false
			p.retryCancel = nil
			p.retryErr = result.err
			if result.err == nil {
				p.retryPlan = &result.plan
				p.retryReceipt = nil
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
	if p.retryControlBusy && p.retryControlResults != nil {
		select {
		case result := <-p.retryControlResults:
			p.retryControlBusy = false
			p.retryControlCancel = nil
			p.retryErr = result.err
			if result.err == nil {
				p.retryReceipt = &result.receipt
				p.retryPlan = nil
			}
		default:
			c.After(50 * time.Millisecond)
		}
	}
}

func (a *App) invalidateSkills() {
	p := &a.skills
	p.generation++
	p.snapshot = remote.SkillCatalog{}
	p.err, p.loaded = nil, false
	if p.busy && p.cancel != nil {
		p.cancel()
	}
	if p.changeBusy && p.changeCancel != nil {
		p.changeCancel()
	}
	if p.conflictsBusy && p.conflictsCancel != nil {
		p.conflictsCancel()
	}
	if p.historyBusy && p.historyCancel != nil {
		p.historyCancel()
	}
	if p.infoBusy && p.infoCancel != nil {
		p.infoCancel()
	}
	if p.stateInfoBusy && p.stateInfoCancel != nil {
		p.stateInfoCancel()
	}
	if p.diffBusy && p.diffCancel != nil {
		p.diffCancel()
	}
	if p.retryBusy && p.retryCancel != nil {
		p.retryCancel()
	}
	if p.retryControlBusy && p.retryControlCancel != nil {
		p.retryControlCancel()
	}
	p.conflicts = remote.SkillStateConflicts{}
	p.conflictsLoaded, p.conflictsErr = false, nil
	p.history = remote.SkillStateList{}
	p.historyLoaded, p.historyErr = false, nil
	p.info, p.infoErr, p.infoSelector = nil, nil, ""
	p.stateInfo, p.stateInfoErr, p.stateInfoSelector = nil, nil, ""
	p.diff, p.diffErr, p.diffCheckpoint = nil, nil, ""
	p.exportCheckpoint = nil
	p.retryPlan, p.retryReceipt, p.retryErr = nil, nil, nil
}

// inspectSkillDiff loads a bounded, metadata-only diff. The CLI remains the
// authority for manifests and file contents; the desktop only renders changed
// paths, sizes and digests after strict context validation.
func (a *App) inspectSkillDiff(checkpointID string) {
	a.inspectSkillDiffPage(checkpointID, nil)
}

func (a *App) inspectCurrentSkillDiff() {
	p := &a.skills
	if p.diffBusy || a.closing || a.quitting.Load() {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsStateDiffMethod, false, nil).CanStart() {
		p.diffErr = &cliadapter.OperationError{Code: "unsupported_method"}
		p.diff = nil
		return
	}
	account := strings.TrimSpace(p.historyAccount)
	scope := p.historyScope
	if scope == "" {
		scope = "account-directory"
	}
	var skill *string
	if scope == "item" {
		value := strings.TrimSpace(p.historySkill)
		if !canonicalSkillUUID(value) {
			p.diffErr = &cliadapter.OperationError{Code: "invalid_params"}
			p.diff = nil
			return
		}
		skill = &value
	}
	if !canonicalSkillUUID(account) {
		p.diffErr = &cliadapter.OperationError{Code: "invalid_params"}
		p.diff = nil
		return
	}
	p.diffCheckpoint, p.diffBusy, p.diffErr = "", true, nil
	p.diff = nil
	p.diffResults = make(chan skillsDiffResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.diffCancel = cancel
	home, find := a.home, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var diff remote.SkillStateDiff
		if err == nil {
			diff, err = remote.DiffSkillState(ctx, cliadapter.Client{Binary: binary, Home: home}, expected, remote.SkillStateDiffInput{
				AccountID: account, Scope: scope, Skill: skill, Limit: 100,
			})
		}
		p.diffResults <- skillsDiffResult{diff: diff, err: err}
	}()
}

func (a *App) inspectSkillDiffPage(checkpointID string, cursor *string) {
	p := &a.skills
	if p.diffBusy || a.closing || a.quitting.Load() || !canonicalSkillUUID(strings.TrimSpace(checkpointID)) {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsStateDiffMethod, false, nil).CanStart() {
		p.diffErr = &cliadapter.OperationError{Code: "unsupported_method"}
		p.diff = nil
		return
	}
	selector := strings.TrimSpace(checkpointID)
	p.diffCheckpoint, p.diffBusy, p.diffErr = selector, true, nil
	p.diff = nil
	p.diffResults = make(chan skillsDiffResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.diffCancel = cancel
	home, find := a.home, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var diff remote.SkillStateDiff
		if err == nil {
			diff, err = remote.DiffSkillState(ctx, cliadapter.Client{Binary: binary, Home: home}, expected, remote.SkillStateDiffInput{
				CheckpointID: &selector, Limit: 100, Cursor: cursor,
			})
		}
		p.diffResults <- skillsDiffResult{diff: diff, err: err}
	}()
}

func (a *App) inspectSkill(item remote.SkillSummary) {
	a.inspectSkillSelector(item.ID, "")
}

func (a *App) inspectLocalSkill(item remote.LocalSkillSummary) {
	if !canonicalSkillUUID(strings.TrimSpace(item.ID)) || !canonicalSkillUUID(strings.TrimSpace(item.AccountID)) {
		return
	}
	a.inspectSkillSelector(item.ID, item.AccountID)
}

func (a *App) inspectSkillSelector(skillID, accountID string) {
	p := &a.skills
	selector := strings.TrimSpace(skillID)
	if p.infoBusy || a.closing || a.quitting.Load() || !canonicalSkillUUID(selector) {
		return
	}
	accountID = strings.TrimSpace(accountID)
	if accountID != "" && !canonicalSkillUUID(accountID) {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsInfoMethod, false, nil).CanStart() {
		p.infoErr = &cliadapter.OperationError{Code: "unsupported_method"}
		p.info = nil
		return
	}
	p.infoSelector, p.infoBusy, p.infoErr = selector, true, nil
	p.info = nil
	p.infoResults = make(chan skillsInfoResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.infoCancel = cancel
	home, find := a.home, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var info remote.SkillInfo
		if err == nil {
			info, err = remote.InfoSkill(ctx, cliadapter.Client{Binary: binary, Home: home}, expected, remote.SkillInfoInput{Skill: selector, AccountID: accountID})
		}
		p.infoResults <- skillsInfoResult{selector: selector, info: info, err: err}
	}()
}

// inspectSkillState loads one exact checkpoint projection through the
// versioned desktop-control contract. The checkpoint ID is immutable and the
// CLI remains authoritative for retention/expiry semantics.
func (a *App) inspectSkillState(checkpointID string) {
	p := &a.skills
	selector := strings.TrimSpace(checkpointID)
	if p.stateInfoBusy || a.closing || a.quitting.Load() || !canonicalSkillUUID(selector) {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsStateInfoMethod, false, nil).CanStart() {
		p.stateInfoErr = &cliadapter.OperationError{Code: "unsupported_method"}
		p.stateInfo = nil
		return
	}
	p.stateInfoSelector, p.stateInfoBusy, p.stateInfoErr = selector, true, nil
	p.stateInfo = nil
	p.stateInfoResults = make(chan skillsStateInfoResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.stateInfoCancel = cancel
	home, find := a.home, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var info remote.SkillStateInfo
		if err == nil {
			info, err = remote.InfoSkillState(ctx, cliadapter.Client{Binary: binary, Home: home}, expected, selector)
		}
		p.stateInfoResults <- skillsStateInfoResult{selector: selector, info: info, err: err}
	}()
}

func (a *App) refreshSkillHistory() {
	a.loadSkillHistory(nil, nil)
}

// loadSkillHistory keeps the checkpoint and pending-finalization cursors
// independent. Advancing one page carries the other cursor forward so the
// two bounded projections remain stable while the user browses them.
func (a *App) loadSkillHistory(cursor, pendingCursor *string) {
	p := &a.skills
	if p.historyBusy || a.closing || a.quitting.Load() {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsStateListMethod, false, nil).CanStart() {
		p.historyErr = &cliadapter.OperationError{Code: "unsupported_method"}
		p.historyLoaded = false
		return
	}
	account := strings.TrimSpace(p.historyAccount)
	scope := p.historyScope
	if scope == "" {
		scope = "account-directory"
	}
	var skill *string
	if scope == "item" {
		value := strings.TrimSpace(p.historySkill)
		if !canonicalSkillUUID(value) {
			p.historyErr = &cliadapter.OperationError{Code: "invalid_params"}
			p.historyLoaded = false
			return
		}
		skill = &value
	}
	if !canonicalSkillUUID(account) {
		p.historyErr = &cliadapter.OperationError{Code: "invalid_params"}
		p.historyLoaded = false
		return
	}
	p.historyBusy, p.historyErr = true, nil
	p.historyResults = make(chan skillsHistoryResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.historyCancel = cancel
	home, find := a.home, a.findExecutable
	cursor = cloneSkillCursor(cursor)
	pendingCursor = cloneSkillCursor(pendingCursor)
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var history remote.SkillStateList
		if err == nil {
			history, err = remote.ListSkillState(ctx, cliadapter.Client{Binary: binary, Home: home}, expected, remote.SkillStateListInput{AccountID: account, Scope: scope, Skill: skill, Limit: 100, Cursor: cursor, PendingCursor: pendingCursor})
		}
		p.historyResults <- skillsHistoryResult{history: history, err: err}
	}()
}

func cloneSkillCursor(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (a *App) refreshSkillConflicts() {
	p := &a.skills
	if p.conflictsBusy || a.closing || a.quitting.Load() {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsStateConflictsMethod, false, nil).CanStart() {
		p.conflictsErr = &cliadapter.OperationError{Code: "unsupported_method"}
		p.conflictsLoaded = false
		return
	}
	if !canonicalSkillUUID(strings.TrimSpace(p.conflictAccount)) {
		p.conflictsErr = &cliadapter.OperationError{Code: "invalid_params"}
		p.conflictsLoaded = false
		return
	}
	account := strings.TrimSpace(p.conflictAccount)
	p.conflictsBusy, p.conflictsErr = true, nil
	p.conflictsResults = make(chan skillsConflictsResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.conflictsCancel = cancel
	home, find := a.home, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var conflicts remote.SkillStateConflicts
		if err == nil {
			conflicts, err = remote.ListSkillConflicts(ctx, cliadapter.Client{Binary: binary, Home: home}, expected, remote.SkillStateConflictsInput{AccountID: account, Scope: "account-directory", Limit: 100})
		}
		p.conflictsResults <- skillsConflictsResult{conflicts: conflicts, err: err}
	}()
}

func canonicalSkillUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func boolLabel(value bool, yes, no string) string {
	if value {
		return yes
	}
	return no
}

func (a *App) reviewSkillChange(item remote.SkillSummary, enable bool) {
	a.skills.review = &skillsChangeReview{
		Skill: item.ID, Name: item.Name, Action: "disable", Expected: a.skills.snapshot.Context,
		Generation: a.skills.snapshot.Generation,
	}
	if enable {
		a.skills.review.Action = "enable"
	}
	a.skills.receipt, a.skills.changeErr = nil, nil
}

func (a *App) submitSkillChange() {
	p := &a.skills
	if p.review == nil || p.changeBusy || a.closing || a.quitting.Load() {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsChangeMethod, true, nil).CanStart() {
		p.changeErr = &cliadapter.OperationError{Code: "unsupported_method"}
		return
	}
	review := *p.review
	ctx, cancel := context.WithCancel(context.Background())
	p.changeBusy, p.changeCancel = true, cancel
	p.changeResults = make(chan skillsChangeResult, 1)
	home, find := a.home, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.SkillChangeReceipt
		if err == nil {
			receipt, err = remote.ChangeSkill(ctx, cliadapter.Client{Binary: binary, Home: home}, remote.SkillChangeInput{
				Confirmed: true, Expected: review.Expected, ExpectedGeneration: review.Generation,
				Skill: review.Skill, Action: review.Action, Scope: remote.SkillScope{Tools: []string{}},
			})
		}
		p.changeResults <- skillsChangeResult{receipt: receipt, err: err}
	}()
}

func (a *App) planSkillRetry() {
	p := &a.skills
	if p.retryBusy || a.closing || a.quitting.Load() || !canonicalSkillUUID(strings.TrimSpace(p.retryOperationID)) {
		p.retryErr = &cliadapter.OperationError{Code: "invalid_params"}
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsRetryPlanMethod, false, nil).CanStart() {
		p.retryErr = &cliadapter.OperationError{Code: "unsupported_method"}
		return
	}
	p.retryBusy, p.retryErr = true, nil
	p.retryReceipt, p.retryPlan = nil, nil
	p.retryResults = make(chan skillsRetryResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.retryCancel = cancel
	home, operationID, find := a.home, strings.TrimSpace(p.retryOperationID), a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var plan remote.SkillRetryPlan
		if err == nil {
			plan, err = remote.PlanSkillRetry(ctx, cliadapter.Client{Binary: binary, Home: home}, expected, remote.SkillRetryPlanInput{Expected: expected, OperationID: operationID})
		}
		p.retryResults <- skillsRetryResult{plan: plan, err: err}
	}()
}

func (a *App) submitSkillRetry() {
	p := &a.skills
	if p.retryPlan == nil || p.retryControlBusy || a.closing || a.quitting.Load() {
		return
	}
	caps, expected, ok := a.skillsSource()
	if !ok || !capability.Assess(caps, expected, nil, remote.SkillsRetryControlMethod, true, nil).CanStart() {
		p.retryErr = &cliadapter.OperationError{Code: "unsupported_method"}
		return
	}
	plan := *p.retryPlan
	p.retryControlBusy, p.retryErr = true, nil
	p.retryControlResults = make(chan skillsRetryControlResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	p.retryControlCancel = cancel
	home, find := a.home, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.SkillRetryReceipt
		if err == nil {
			receipt, err = remote.ControlSkillRetry(ctx, cliadapter.Client{Binary: binary, Home: home}, remote.SkillRetryControlInput{
				Confirmed: true, Expected: expected, OperationID: plan.OperationID,
				ExpectedGeneration: plan.Generation, ExpectedPlanFingerprint: plan.PlanFingerprint,
			})
		}
		p.retryControlResults <- skillsRetryControlResult{receipt: receipt, err: err}
	}()
}

func skillMatches(item remote.SkillSummary, filter string) bool {
	filter = strings.TrimSpace(strings.ToLower(filter))
	return filter == "" || strings.Contains(strings.ToLower(item.Name), filter) || strings.Contains(strings.ToLower(item.ID), filter)
}

// openSkillStateWorkflow keeps the conflict identity in the reviewed argv
// while leaving branch inputs, paths and materialized content to the CLI. The
// CLI schema remains the source of truth for all argument names and validation.
func (a *App) openSkillStateWorkflow(action string, item remote.SkillConflictSummary) {
	if !a.capabilities.loaded {
		return
	}
	path := []string{"skill", "state", action}
	accountID := strings.TrimSpace(a.skills.conflictAccount)
	for _, catalog := range a.capabilities.snapshot.Catalogs {
		if catalog.Executable != "agent-remote" || catalog.Err != nil {
			continue
		}
		for _, command := range catalog.Schema.Commands {
			if command.HasSubcommands || len(command.Path) != len(path) {
				continue
			}
			match := true
			for i := range path {
				if command.Path[i] != path[i] {
					match = false
					break
				}
			}
			if !match {
				continue
			}
			workflow := "skill_state_" + action
			if dedicated, ok := cliadapter.Dedicated("agent-remote", path); !ok || !a.capabilities.snapshot.Capabilities.SupportsTerminalWorkflow(dedicated.Method, workflow) {
				return
			}
			a.openGenericCLI(catalog.Executable, command)
			p := &a.genericCLI
			p.workflow = workflow
			p.workflowSummary = locales.SkillsConflictWorkflow(item.ID, action)
			if action == "resolve" {
				setSkillWorkflowArgument(p, "conflict", item.ID)
			} else if action == "migrate" || action == "restore" {
				setSkillWorkflowArgument(p, "skill", item.SkillID)
				setSkillWorkflowArgument(p, "account_id", accountID)
			}
			return
		}
	}
}

// openSkillStateExport opens the schema-driven export review from the exact
// retained checkpoint the user selected.  The CLI schema remains authoritative
// for required flags; when a current CLI exposes checkpoint/scope selectors we
// prefill them from the metadata-only checkpoint projection so the user cannot
// accidentally export a different checkpoint.  The output destination is
// intentionally left for the user to choose in the reviewed form.
func (a *App) openSkillStateExport(checkpoint remote.SkillStateCheckpoint) {
	if !checkpoint.Retained || !canonicalSkillUUID(strings.TrimSpace(checkpoint.ID)) {
		return
	}
	if !a.capabilities.loaded || a.capabilities.busy {
		// Skills can be opened from the already-loaded remote snapshot before
		// passive schema discovery has completed. Keep the exact checkpoint and
		// finish opening the review when the catalog arrives.
		copy := checkpoint
		a.skills.exportCheckpoint = &copy
		a.refreshCapabilities()
		return
	}
	for _, catalog := range a.capabilities.snapshot.Catalogs {
		if catalog.Executable != "agent-remote" || catalog.Err != nil {
			continue
		}
		for _, command := range catalog.Schema.Commands {
			if command.HasSubcommands || len(command.Path) != 3 || strings.Join(command.Path, " ") != "skill state export" {
				continue
			}
			if _, ok := cliadapter.Dedicated("agent-remote", command.Path); !ok {
				return
			}
			a.openGenericCLI(catalog.Executable, command)
			p := &a.genericCLI
			p.workflow = "skill_state_export"
			p.workflowSummary = locales.SkillsStateExportReview()
			setSkillWorkflowArgument(p, "checkpoint", checkpoint.ID)
			setSkillWorkflowArgument(p, "account_id", checkpoint.AccountID)
			if checkpoint.Scope == "account-directory" {
				setSkillWorkflowArgument(p, "scope", checkpoint.Scope)
			} else if checkpoint.SkillID != nil {
				setSkillWorkflowArgument(p, "skill", *checkpoint.SkillID)
			}
			return
		}
	}
}

func setSkillWorkflowArgument(page *genericCLIPage, name, value string) {
	if value == "" {
		return
	}
	for _, argument := range page.command.Arguments {
		if argument.ID != name && argument.Long != name {
			continue
		}
		page.values[argument.ID] = []string{value}
		page.text[argument.ID] = value
		if len(argument.Choices) != 0 {
			page.choice[argument.ID] = value
		}
		return
	}
}

func (a *App) skillsView(c *ui.Context) {
	p := &a.skills
	t := c.Theme()
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Text(c, locales.SkillsTitle()).FontSize(24).Bold().Grow(1)
			ui.Button(c, locales.ReturnWorkbench()).OnClick(func() { a.page = "" })
			ui.Button(c, locales.SkillsRefresh()).Disabled(p.busy).OnClick(a.refreshSkills)
		})
		ui.Text(c, locales.SkillsHelp()).TextColor(t.TextMuted)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Text(c, locales.SkillsRetryOperation()).FontSize(12)
			ui.TextInput(c, &p.retryOperationID).Placeholder(locales.SkillsRetryOperationPlaceholder()).Width(360)
			ui.Button(c, locales.SkillsRetryPlan()).Disabled(p.retryBusy || p.retryControlBusy).OnClick(a.planSkillRetry)
		})
		if p.retryBusy {
			ui.Text(c, locales.SkillsRetryWorking()).TextColor(t.TextMuted)
		}
		if p.retryPlan != nil {
			ui.Column(c).FillWidth().Padding(12).Gap(5).Background(t.Surface).Radius(8).Children(func() {
				ui.Text(c, locales.SkillsRetryReview()).Bold()
				ui.Text(c, locales.SkillsRetryTargets(p.retryPlan.TargetCount)).TextColor(t.TextMuted)
				ui.Text(c, locales.SkillsGeneration(int(p.retryPlan.Generation))).FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					ui.Button(c, locales.Cancel()).Disabled(p.retryControlBusy).OnClick(func() { p.retryPlan = nil })
					ui.PrimaryButton(c, locales.SkillsRetryConfirm()).Disabled(p.retryControlBusy).OnClick(a.submitSkillRetry)
				})
			})
		}
		if p.retryControlBusy {
			ui.Text(c, locales.SkillsRetryWorking()).TextColor(t.TextMuted)
		}
		if p.retryReceipt != nil {
			ui.Text(c, locales.SkillsRetryAccepted(p.retryReceipt.TargetCount)).Bold().TextColor(t.Success)
			ui.Text(c, p.retryReceipt.SourceOperationID).FontSize(12).TextColor(t.TextMuted)
		}
		if p.retryErr != nil {
			ui.Text(c, locales.SkillsRetryUnknown()).TextColor(t.Danger)
		}
		if p.infoBusy {
			ui.Text(c, locales.SkillsInfoLoading()).TextColor(t.TextMuted)
		} else if p.infoErr != nil {
			if op, ok := p.infoErr.(*cliadapter.OperationError); ok && op.Code == "unsupported_method" {
				ui.Text(c, locales.SkillsInfoUnsupported()).TextColor(t.Danger)
			} else {
				ui.Text(c, locales.ErrorUnknown(p.infoErr.Error())).TextColor(t.Danger)
			}
		} else if p.info != nil {
			info := p.info
			ui.Column(c).FillWidth().Padding(12).Gap(5).Background(t.Surface).Radius(8).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, info.Name).Bold().Grow(1)
					ui.Button(c, locales.Cancel()).OnClick(func() { p.info = nil; p.infoErr = nil; p.infoSelector = "" })
				})
				ui.Text(c, info.ID).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, locales.SkillsInfoKind(info.Kind)).FontSize(12)
				if info.Status != "" {
					ui.Text(c, locales.SkillsInfoStatus(info.Status)).FontSize(12)
				}
				if info.AccountID != "" {
					ui.Text(c, locales.SkillsInfoAccount(info.AccountID)).FontSize(12)
				}
				if info.SourceCheckpoint != "" {
					ui.Text(c, locales.SkillsInfoSourceCheckpoint(info.SourceCheckpoint)).FontSize(12)
				}
				if info.Epoch > 0 {
					ui.Text(c, locales.SkillsInfoEpoch(strconv.FormatInt(info.Epoch, 10))).FontSize(12)
				}
				ui.Text(c, locales.SkillsInfoRevisions(strconv.Itoa(info.RevisionCount))).FontSize(12)
				for _, revision := range info.Revisions {
					retained := locales.SkillsInfoUnretained()
					if revision.Retained {
						retained = locales.SkillsInfoRetained()
					}
					ui.Text(c, locales.SkillsInfoRevision(strconv.FormatInt(revision.Number, 10), revision.ID, retained)).FontSize(12).TextColor(t.TextMuted)
				}
				if info.Effective != nil {
					included := locales.SkillsInfoExcluded()
					if info.Effective.Included {
						included = locales.SkillsInfoIncluded()
					}
					enabled := locales.SkillsDisabled()
					if info.Effective.Enabled {
						enabled = locales.SkillsEnabled()
					}
					ui.Text(c, locales.SkillsInfoEffective(enabled, info.Effective.RevisionID, included)).FontSize(12)
				}
			})
		}
		if p.stateInfoBusy {
			ui.Text(c, locales.SkillsStateInfoLoading()).TextColor(t.TextMuted)
		} else if p.stateInfoErr != nil {
			if op, ok := p.stateInfoErr.(*cliadapter.OperationError); ok && op.Code == "unsupported_method" {
				ui.Text(c, locales.SkillsStateInfoUnsupported()).TextColor(t.Danger)
			} else {
				ui.Text(c, locales.ErrorUnknown(p.stateInfoErr.Error())).TextColor(t.Danger)
			}
		} else if p.stateInfo != nil {
			ui.Column(c).FillWidth().Padding(12).Gap(5).Background(t.Surface).Radius(8).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, locales.SkillsStateInfoTitle()).Bold().Grow(1)
					ui.Button(c, locales.Cancel()).OnClick(func() { p.stateInfo = nil; p.stateInfoErr = nil; p.stateInfoSelector = "" })
				})
				ui.Text(c, locales.SkillsStateInfoStatus(p.stateInfo.Status)).FontSize(12)
				if checkpoint := p.stateInfo.Checkpoint; checkpoint != nil {
					ui.Text(c, locales.SkillsStateCheckpoint(checkpoint.ID)).FontSize(12)
					ui.Text(c, locales.SkillsStateCheckpointStatus(checkpoint.StorageLocation, boolLabel(checkpoint.Retained, locales.SkillsStateCheckpointRetained(), locales.SkillsStateCheckpointUnretained()), boolLabel(checkpoint.IsHead, locales.SkillsStateCheckpointHead(), locales.SkillsStateCheckpointNotHead()))).FontSize(12)
					ui.Text(c, locales.SkillsStateCheckpointCreated(checkpoint.CreatedAt)).FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, locales.SkillsStateInfoDigest(checkpoint.ContentDigest)).FontSize(12).TextColor(t.TextMuted)
				}
			})
		}
		if p.diffBusy {
			ui.Text(c, locales.SkillsStateDiffLoading()).TextColor(t.TextMuted)
		} else if p.diffErr != nil {
			if op, ok := p.diffErr.(*cliadapter.OperationError); ok && op.Code == "unsupported_method" {
				ui.Text(c, locales.SkillsStateDiffUnsupported()).TextColor(t.Danger)
			} else {
				ui.Text(c, locales.ErrorUnknown(p.diffErr.Error())).TextColor(t.Danger)
			}
		} else if p.diff != nil {
			diff := p.diff
			ui.Column(c).FillWidth().Padding(12).Gap(5).Background(t.Surface).Radius(8).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, locales.SkillsStateDiffTitle()).Bold().Grow(1)
					ui.Button(c, locales.Cancel()).Disabled(p.diffBusy).OnClick(func() { p.diff = nil; p.diffErr = nil; p.diffCheckpoint = "" })
				})
				ui.Text(c, locales.SkillsStateDiffSummary(diff.Mode, strconv.Itoa(len(diff.Items)))).FontSize(12)
				ui.Text(c, locales.SkillsStateCheckpoint(diff.CheckpointID)).FontSize(12).TextColor(t.TextMuted)
				ui.Text(c, locales.SkillsStateDiffDigest(diff.CurrentTreeDigest)).FontSize(12).TextColor(t.TextMuted)
				if len(diff.Items) == 0 {
					ui.Text(c, locales.SkillsStateEmpty()).FontSize(12).TextColor(t.TextMuted)
				}
				for _, item := range diff.Items {
					item := item
					entry := item.Current
					if entry == nil {
						entry = item.Base
					}
					if entry == nil {
						continue
					}
					ui.Column(c.Key("skill-diff-" + item.Path)).FillWidth().Padding(8).Background(t.Surface.Alpha(.65)).Radius(6).Children(func() {
						ui.Text(c, locales.SkillsStateDiffPath(item.Path, entry.Kind, strconv.FormatUint(entry.Size, 10))).Bold()
						if item.Base != nil {
							ui.Text(c, locales.SkillsStateDiffDigest(item.Base.SHA256)).FontSize(12).TextColor(t.TextMuted)
						}
						if item.Current != nil {
							ui.Text(c, locales.SkillsStateDiffDigest(item.Current.SHA256)).FontSize(12).TextColor(t.TextMuted)
						}
					})
				}
				if next := diff.NextCursor; next != nil {
					next := next
					ui.Button(c, locales.SkillsStateDiffNext()).Disabled(p.diffBusy).OnClick(func() { a.inspectSkillDiffPage(p.diffCheckpoint, next) })
				}
			})
		}
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, &p.conflictAccount).Label(locales.SkillsConflictAccountPlaceholder()).Placeholder(locales.SkillsConflictAccountPlaceholder()).FillWidth()
			ui.Button(c, locales.SkillsConflictsRefresh()).Disabled(p.conflictsBusy || !canonicalSkillUUID(strings.TrimSpace(p.conflictAccount))).OnClick(a.refreshSkillConflicts)
		})
		ui.Text(c, locales.SkillsConflictAccountHelp()).FontSize(12).TextColor(t.TextMuted)
		ui.Column(c).FillWidth().Padding(12).Gap(8).Background(t.Surface.Alpha(.65)).Radius(8).Children(func() {
			ui.Text(c, locales.SkillsStateTitle()).Bold()
			ui.Text(c, locales.SkillsStateHelp()).FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				ui.TextInput(c, &p.historyAccount).Label(locales.SkillsStateAccountPlaceholder()).Placeholder(locales.SkillsStateAccountPlaceholder()).FillWidth()
				ui.Button(c, locales.SkillsStateScopeAccount()).Disabled(p.historyScope == "account-directory").OnClick(func() { p.historyScope = "account-directory"; p.historyLoaded = false })
				ui.Button(c, locales.SkillsStateScopeItem()).Disabled(p.historyScope == "item").OnClick(func() { p.historyScope = "item"; p.historyLoaded = false })
			})
			if p.historyScope == "item" {
				ui.TextInput(c, &p.historySkill).Label(locales.SkillsStateSkillPlaceholder()).Placeholder(locales.SkillsStateSkillPlaceholder()).FillWidth()
			}
			ui.Row(c).Gap(8).Wrap().Children(func() {
				ui.Button(c, locales.SkillsStateRefresh()).Disabled(p.historyBusy).OnClick(a.refreshSkillHistory)
				ui.Button(c, locales.SkillsStateCurrentDiff()).Disabled(p.diffBusy).OnClick(a.inspectCurrentSkillDiff)
			})
			if p.historyBusy {
				ui.Text(c, locales.SkillsStateLoading()).TextColor(t.TextMuted)
			} else if p.historyErr != nil {
				if op, ok := p.historyErr.(*cliadapter.OperationError); ok && op.Code == "unsupported_method" {
					ui.Text(c, locales.SkillsStateUnsupported()).TextColor(t.Danger)
				} else {
					ui.Text(c, locales.ErrorUnknown(p.historyErr.Error())).TextColor(t.Danger)
				}
			} else if p.historyLoaded {
				ui.Text(c, locales.SkillsStateCheckpoints()).Bold()
				if len(p.history.Checkpoints.Items) == 0 {
					ui.Text(c, locales.SkillsStateEmpty()).FontSize(12).TextColor(t.TextMuted)
				} else {
					for _, item := range p.history.Checkpoints.Items {
						item := item
						ui.Column(c.Key("skill-checkpoint-" + item.ID)).FillWidth().Padding(8).Background(t.Surface).Radius(6).Children(func() {
							ui.Text(c, locales.SkillsStateCheckpoint(item.ID)).Bold()
							retained := locales.SkillsStateCheckpointUnretained()
							if item.Retained {
								retained = locales.SkillsStateCheckpointRetained()
							}
							head := locales.SkillsStateCheckpointNotHead()
							if item.IsHead {
								head = locales.SkillsStateCheckpointHead()
							}
							ui.Text(c, locales.SkillsStateCheckpointStatus(item.StorageLocation, retained, head)).FontSize(12)
							ui.Text(c, locales.SkillsStateCheckpointCreated(item.CreatedAt)).FontSize(12).TextColor(t.TextMuted)
							ui.Button(c, locales.SkillsStateInfo()).Disabled(p.stateInfoBusy).OnClick(func() { a.inspectSkillState(item.ID) })
							ui.Button(c, locales.SkillsStateDiff()).Disabled(p.diffBusy).OnClick(func() { a.inspectSkillDiff(item.ID) })
							if item.Retained {
								ui.Button(c, locales.SkillsStateExport()).OnClick(func() { a.openSkillStateExport(item) })
							}
						})
					}
				}
				if next := p.history.Checkpoints.NextCursor; next != nil {
					next := next
					pending := p.history.Pending.NextCursor
					ui.Button(c, locales.SkillsStateNextCheckpoints()).Disabled(p.historyBusy).OnClick(func() { a.loadSkillHistory(next, pending) })
				}
				ui.Text(c, locales.SkillsStatePending()).Bold().Margin(8, 0, 0)
				if len(p.history.Pending.Items) == 0 {
					ui.Text(c, locales.SkillsStateEmpty()).FontSize(12).TextColor(t.TextMuted)
				} else {
					for _, item := range p.history.Pending.Items {
						ui.Text(c, locales.SkillsStatePendingItem(item.ID, item.NodeID, item.Status)).FontSize(12)
					}
				}
				if next := p.history.Pending.NextCursor; next != nil {
					next := next
					cursor := p.history.Checkpoints.NextCursor
					ui.Button(c, locales.SkillsStateNextPending()).Disabled(p.historyBusy).OnClick(func() { a.loadSkillHistory(cursor, next) })
				}
			}
		})
		if p.conflictsBusy {
			ui.Text(c, locales.SkillsConflictsLoading()).TextColor(t.TextMuted)
		} else if p.conflictsErr != nil {
			if op, ok := p.conflictsErr.(*cliadapter.OperationError); ok && op.Code == "unsupported_method" {
				ui.Text(c, locales.SkillsConflictsUnsupported()).TextColor(t.Danger)
			} else if op, ok := p.conflictsErr.(*cliadapter.OperationError); ok && op.Code == "invalid_params" {
				ui.Text(c, locales.SkillsConflictAccountHelp()).TextColor(t.Danger)
			} else {
				ui.Text(c, locales.ErrorUnknown(p.conflictsErr.Error())).TextColor(t.Danger)
			}
		} else if p.conflictsLoaded {
			ui.Text(c, locales.SkillsConflictPublication()).Bold()
			if len(p.conflicts.Publications.Items) == 0 {
				ui.Text(c, locales.SkillsConflictsEmpty()).FontSize(12).TextColor(t.TextMuted)
			} else {
				for _, item := range p.conflicts.Publications.Items {
					item := item
					ui.Row(c).Gap(8).Wrap().Children(func() {
						ui.Text(c, locales.SkillsConflictStatus(item.Status)+" · "+item.ID).FontSize(12).Grow(1)
						ui.Button(c, locales.SkillsConflictResolve()).OnClick(func() { a.openSkillStateWorkflow("resolve", item) })
					})
					if item.Reason != nil {
						ui.Text(c, locales.SkillsConflictReason(*item.Reason)).FontSize(12).TextColor(t.TextMuted)
					}
				}
			}
			ui.Text(c, locales.SkillsConflictMigration()).Bold()
			if len(p.conflicts.Migrations.Items) == 0 {
				ui.Text(c, locales.SkillsConflictsEmpty()).FontSize(12).TextColor(t.TextMuted)
			} else {
				for _, item := range p.conflicts.Migrations.Items {
					item := item
					ui.Row(c).Gap(8).Wrap().Children(func() {
						ui.Text(c, locales.SkillsConflictTarget(item.Name, item.Status)+" · "+item.ID).FontSize(12).Grow(1)
						ui.Button(c, locales.SkillsConflictResolve()).OnClick(func() { a.openSkillStateWorkflow("resolve", item) })
						ui.Button(c, locales.SkillsConflictMigrate()).OnClick(func() { a.openSkillStateWorkflow("migrate", item) })
						ui.Button(c, locales.SkillsConflictRestore()).OnClick(func() { a.openSkillStateWorkflow("restore", item) })
					})
				}
			}
		}
		if p.receipt != nil {
			actionLabel := locales.SkillsEnable()
			if p.receipt.Action == "disable" {
				actionLabel = locales.SkillsDisable()
			}
			ui.Text(c, locales.SkillsChangeAccepted(actionLabel)).Bold().TextColor(t.Success)
			ui.Text(c, locales.SkillsChangeReceipt(p.receipt.OperationID)).FontSize(12).TextColor(t.TextMuted)
		}
		if p.review != nil {
			ui.Column(c).FillWidth().Padding(12).Gap(5).Background(t.Surface).Radius(8).Children(func() {
				ui.Text(c, locales.SkillsChangeReview()).Bold()
				ui.Text(c, locales.SkillsChangeTarget(p.review.Name))
				actionLabel := locales.SkillsEnable()
				if p.review.Action == "disable" {
					actionLabel = locales.SkillsDisable()
				}
				ui.Text(c, locales.SkillsChangeAction(actionLabel))
				ui.Text(c, locales.SkillsGeneration(int(p.review.Generation))).FontSize(12).TextColor(t.TextMuted)
				ui.Row(c).Gap(8).Children(func() {
					ui.Button(c, locales.Cancel()).Disabled(p.changeBusy).OnClick(func() { p.review = nil })
					ui.PrimaryButton(c, locales.SkillsChangeConfirm()).Disabled(p.changeBusy).OnClick(a.submitSkillChange)
				})
			})
		}
		if p.changeBusy {
			ui.Text(c, locales.SkillsChangeWorking()).TextColor(t.TextMuted)
		}
		if p.changeErr != nil {
			ui.Text(c, locales.ErrorUnknown(p.changeErr.Error())).TextColor(t.Danger)
		}
		ui.TextInput(c, &p.filter).Placeholder(locales.SkillsFilterPlaceholder()).FillWidth()
		if p.busy {
			ui.Text(c, locales.SkillsLoading()).TextColor(t.TextMuted)
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(p.cancel)
			return
		}
		if p.err != nil {
			if op, ok := p.err.(*cliadapter.OperationError); ok && op.Code == "unsupported_method" {
				ui.Text(c, locales.SkillsUnsupported()).TextColor(t.Danger)
			} else {
				ui.Text(c, locales.ErrorUnknown(p.err.Error())).TextColor(t.Danger)
			}
			return
		}
		if !p.loaded {
			return
		}
		ui.Text(c, locales.SkillsGeneration(int(p.snapshot.Generation))).FontSize(12).TextColor(t.TextMuted)
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			shown := 0
			for _, item := range p.snapshot.Items {
				if !skillMatches(item, p.filter) {
					continue
				}
				shown++
				ui.Column(c.Key("skill-" + item.ID)).FillWidth().Padding(14).Gap(5).Background(t.Surface).Radius(8).Children(func() {
					ui.Row(c).Gap(10).Children(func() {
						ui.Text(c, item.Name).Bold().Grow(1)
						if item.Removed {
							ui.Text(c, locales.SkillsStatus("removed")).TextColor(t.Danger)
						} else if item.Effective != nil && item.Effective.Enabled {
							ui.Text(c, locales.SkillsEnabled()).TextColor(t.Accent)
						} else {
							ui.Text(c, locales.SkillsDisabled()).TextColor(t.TextMuted)
						}
					})
					ui.Text(c, item.ID).FontSize(12).TextColor(t.TextMuted)
					if item.Effective != nil {
						state := locales.SkillsExcluded()
						if item.Effective.Included {
							state = locales.SkillsIncluded()
						}
						ui.Text(c, locales.SkillsRevisionState(state, locales.SkillsRevision(item.Effective.RevisionID))).FontSize(12)
					} else {
						ui.Text(c, locales.SkillsRevision(item.DefaultRevisionID)).FontSize(12)
					}
					ui.Text(c, locales.SkillsDiscovery(item.ProjectDiscovery)).FontSize(12).TextColor(t.TextMuted)
					model := locales.SkillsModelPending()
					if item.ModelLoaded {
						model = locales.SkillsModelLoaded()
					}
					ui.Text(c, model).FontSize(12).TextColor(t.TextMuted)
					if !item.Removed && item.ID != "" {
						enable := item.Effective == nil || !item.Effective.Enabled
						label := locales.SkillsEnable()
						if !enable {
							label = locales.SkillsDisable()
						}
						ui.Button(c, label).Disabled(p.changeBusy || p.review != nil).OnClick(func() { a.reviewSkillChange(item, enable) })
						if canonicalSkillUUID(item.ID) {
							ui.Button(c, locales.SkillsInfo()).Disabled(p.infoBusy).OnClick(func() { a.inspectSkill(item) })
						}
					}
				})
			}
			if shown == 0 {
				if len(p.snapshot.Items) == 0 {
					ui.Text(c, locales.SkillsEmpty()).TextColor(t.TextMuted)
				} else {
					ui.Text(c, locales.SkillsNoMatch()).TextColor(t.TextMuted)
				}
			}
			if len(p.snapshot.LocalItems) > 0 {
				ui.Text(c, locales.SkillsLocal()).Bold().Margin(12, 0, 0)
				for _, item := range p.snapshot.LocalItems {
					ui.Column(c.Key("local-skill-" + item.ID)).FillWidth().Padding(12).Background(t.Surface.Alpha(.65)).Radius(8).Children(func() {
						ui.Row(c).Gap(8).Children(func() {
							ui.Text(c, item.Name).Bold().Grow(1)
							if canonicalSkillUUID(item.ID) && canonicalSkillUUID(item.AccountID) {
								ui.Button(c, locales.SkillsInfo()).Disabled(p.infoBusy).OnClick(func() { a.inspectLocalSkill(item) })
							}
						})
						ui.Text(c, item.ID).FontSize(12).TextColor(t.TextMuted)
						ui.Text(c, locales.SkillsAccount(item.AccountID)+" · "+locales.SkillsStatus(item.Status)).FontSize(12)
					})
				}
			}
		})
	})
}
