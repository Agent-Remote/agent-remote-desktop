package app

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/egoist/mygo/ui"
)

type remotePage struct {
	generation             uint64
	snapshot               remote.Overview
	err                    error
	busy                   bool
	loaded                 bool
	tab                    string
	filter                 string
	sessionToolFilter      string
	sessionStatusFilters   []string
	sessionToolDraft       string
	sessionStatusDraft     map[string]bool
	sessionFilterDirty     bool
	sessionFiltersOpen     bool
	sessionToolOptions     []string
	sessionStatusOptions   []string
	cancel                 context.CancelFunc
	done                   chan struct{}
	results                chan remoteResult
	attachOpen             bool
	attach                 launchRequest
	attachSession          cliadapter.RemoteSession
	recovery               *workspace.Session
	recoveryNotice         func() string
	createOpen             bool
	createBusy             bool
	createErr              error
	createReceipt          *remote.SessionCreateReceipt
	createCancel           context.CancelFunc
	createDone             chan struct{}
	createResults          chan remoteCreateResult
	createGeneration       uint64
	createAccount          string
	createTool             string
	createWorkspace        string
	createProject          string
	createDigest           string
	createSyncTarget       string
	createReplace          string
	createReplaceAccount   string
	createReplaceWorkspace string
	createReplaceProject   string
	createReplaceNode      string
	createReplaceBackend   string
	createQueryKey         string
	createContext          cliadapter.ConfigContext
	createRecovery         bool
	stopOpen               bool
	stopBusy               bool
	stopErr                error
	stopReceipt            *remote.SessionStopReceipt
	stopSession            cliadapter.RemoteSession
	stopContext            cliadapter.ConfigContext
	stopRequestKey         string
	stopSubmitted          bool
	stopCancel             context.CancelFunc
	stopDone               chan struct{}
	stopResults            chan remoteStopResult
	deleteOpen             bool
	deleteBusy             bool
	deleteErr              error
	deleteReceipt          *remote.SessionDeleteReceipt
	deleteSessions         []cliadapter.RemoteSession
	deleteSelected         map[string]bool
	deleteContext          cliadapter.ConfigContext
	deleteRequestKey       string
	deleteSubmitted        bool
	deleteRecovery         bool
	deleteSelection        remote.SessionDeleteSelection
	deleteDigest           string
	deleteFingerprint      string
	deleteCancel           context.CancelFunc
	deleteDone             chan struct{}
	deleteResults          chan remoteDeleteResult
}
type remoteResult struct {
	generation uint64
	snapshot   remote.Overview
	err        error
}

type remoteCreateResult struct {
	generation uint64
	receipt    remote.SessionCreateReceipt
	err        error
	query      bool
}

type remoteStopResult struct {
	receipt remote.SessionStopReceipt
	err     error
	query   bool
}

type remoteDeleteResult struct {
	receipt remote.SessionDeleteReceipt
	err     error
	query   bool
}

func (a *App) openRemoteCreate() {
	if a.remoteLifecycleBusy() {
		return
	}
	a.remote.createOpen = true
	a.remote.createBusy = false
	a.remote.createErr = nil
	a.remote.createReceipt = nil
	a.remote.createQueryKey = ""
	a.remote.createContext = copyConfigContext(a.remote.snapshot.Config)
	a.remote.createGeneration = a.remote.generation
	a.remote.createRecovery = false
	a.remote.createTool = "claude"
	a.remote.createAccount = ""
	a.remote.createWorkspace = ""
	a.remote.createProject = ""
	a.remote.createDigest = ""
	a.remote.createSyncTarget = "{}"
	a.remote.createReplace = ""
	a.remote.createReplaceAccount = ""
	a.remote.createReplaceWorkspace = ""
	a.remote.createReplaceProject = ""
	a.remote.createReplaceNode = ""
	a.remote.createReplaceBackend = ""
	for _, account := range a.remote.snapshot.Accounts {
		if account.Tool == a.remote.createTool && account.Status != "disabled" {
			a.remote.createAccount = account.ID
			break
		}
	}
}

func (a *App) remoteCreateRecoveryPresent() bool {
	return a.remote.createRecovery && strings.TrimSpace(a.remote.createQueryKey) != "" &&
		a.remote.createContext.Home == a.home && a.remote.createContext.ServerURL != nil && a.remote.createContext.DeviceID != nil
}

// openRemoteCreateRecovery restores only the exact caller key and its frozen
// configuration scope. All creation inputs and the receipt remain owned by
// the CLI journal; the user can only query this request after a restart.
func (a *App) openRemoteCreateRecovery() {
	recovery := a.state.RemoteCreateRecovery
	if recovery == nil || a.remote.createBusy || a.remote.stopBusy || a.remote.deleteBusy {
		return
	}
	context := cliadapter.ConfigContext{Home: recovery.Home, ServerURL: optionalString(recovery.ServerURL), DeviceID: optionalString(recovery.DeviceID)}
	if !sameConfigContext(a.remote.snapshot.Config, context) {
		a.remote.createErr = fault.CLIContextChanged
		a.remote.createOpen = true
		return
	}
	a.remote.createOpen = true
	a.remote.createBusy = false
	a.remote.createErr = nil
	a.remote.createReceipt = nil
	a.remote.createQueryKey = recovery.RequestID
	a.remote.createContext = context
	a.remote.createGeneration = a.remote.generation
	a.remote.createRecovery = true
	// Keep the query-only dialog free of stale values from a prior review.
	a.remote.createTool, a.remote.createAccount = "", ""
	a.remote.createWorkspace, a.remote.createProject = "", ""
	a.remote.createDigest, a.remote.createSyncTarget = "", "{}"
	a.remote.createReplace = ""
	a.remote.createReplaceAccount, a.remote.createReplaceWorkspace = "", ""
	a.remote.createReplaceProject, a.remote.createReplaceNode = "", ""
	a.remote.createReplaceBackend = ""
}

func (a *App) submitRemoteCreate() {
	if a.remote.createBusy {
		return
	}
	if !a.remoteCreateConfirmationCurrent() {
		a.remote.createErr = fault.SessionCreateInputInvalid
		if !sameConfigContext(a.remote.snapshot.Config, a.remote.createContext) {
			a.remote.createErr = fault.CLIContextChanged
		}
		return
	}
	var target json.RawMessage
	if !remoteCreateInputValid(a.remote.createTool, a.remote.createAccount, a.remote.createWorkspace, a.remote.createProject, a.remote.createDigest, a.remote.createSyncTarget) || json.Unmarshal([]byte(a.remote.createSyncTarget), &target) != nil || len(target) == 0 || string(target) == "null" {
		a.remote.createErr = fault.SessionCreateInputInvalid
		return
	}
	input := remote.SessionCreateInput{
		SchemaVersion: 1, Expected: a.remote.createContext,
		ToolType: a.remote.createTool, AccountID: a.remote.createAccount,
		WorkspaceID: a.remote.createWorkspace, ProjectKey: a.remote.createProject,
		ArgvDigest: a.remote.createDigest, SyncTarget: target, Confirmed: true,
	}
	if a.remote.createReplace != "" {
		input.Replacement = &remote.SessionReplacement{
			SessionID:   a.remote.createReplace,
			AccountID:   a.remote.createReplaceAccount,
			WorkspaceID: a.remote.createReplaceWorkspace,
			ProjectKey:  a.remote.createReplaceProject,
			NodeID:      a.remote.createReplaceNode,
			Backend:     a.remote.createReplaceBackend,
		}
	}
	a.remote.createBusy = true
	a.remote.createErr = nil
	a.remote.createReceipt = nil
	a.remote.createDone = make(chan struct{})
	a.remote.createResults = make(chan remoteCreateResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	a.remote.createCancel = cancel
	results, done, home, generation := a.remote.createResults, a.remote.createDone, a.home, a.remote.createGeneration
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var receipt remote.SessionCreateReceipt
		if err == nil {
			receipt, err = remote.CreateSession(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
		}
		results <- remoteCreateResult{generation: generation, receipt: receipt, err: err}
	}()
}

func (a *App) queryRemoteCreate() {
	if a.remote.createBusy || a.remote.createQueryKey == "" {
		return
	}
	if !a.remoteCreateQueryCurrent() {
		a.remote.createErr = fault.CLIContextChanged
		return
	}
	request := remote.SessionCreateRequestInput{RequestID: a.remote.createQueryKey, Expected: a.remote.createContext}
	a.remote.createBusy = true
	a.remote.createErr = nil
	a.remote.createReceipt = nil
	a.remote.createDone = make(chan struct{})
	a.remote.createResults = make(chan remoteCreateResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	a.remote.createCancel = cancel
	results, done, home, generation := a.remote.createResults, a.remote.createDone, a.home, a.remote.createGeneration
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var receipt remote.SessionCreateReceipt
		if err == nil {
			receipt, err = remote.QuerySessionCreate(ctx, cliadapter.Client{Binary: binary, Home: home}, request)
		}
		results <- remoteCreateResult{generation: generation, receipt: receipt, err: err, query: true}
	}()
}

func (a *App) pollRemoteCreate(c *ui.Context) {
	if !a.remote.createBusy || a.remote.createResults == nil {
		return
	}
	select {
	case result := <-a.remote.createResults:
		a.remote.createBusy = false
		a.remote.createCancel = nil
		a.remote.createDone, a.remote.createResults = nil, nil
		if result.generation != a.remote.generation || result.generation != a.remote.createGeneration {
			a.remote.createErr = fault.CLIContextChanged
			a.remote.createReceipt = nil
			return
		}
		if result.err != nil {
			a.remote.createErr = result.err
		} else {
			a.remote.createReceipt = &result.receipt
			if a.remote.createQueryKey == "" {
				a.remote.createQueryKey = result.receipt.RequestID
			}
			// Persist only the immutable query scope. Exact receipt recovery is
			// performed by the CLI journal after restart.
			a.remote.createContext = copyConfigContext(result.receipt.Context)
			a.remote.createRecovery = true
			a.state.RemoteCreateRecovery = a.remoteCreateRecoveryRecord()
			a.persist()
		}
	default:
		if c != nil {
			c.After(50 * time.Millisecond)
		}
	}
}

func (a *App) remoteWorkflowAvailable(method string) bool {
	return a.remote.loaded && !a.remote.busy && a.remote.err == nil && a.remote.snapshot.Capabilities.MatchesHostPlatform() && capability.Assess(a.remote.snapshot.Capabilities, a.remote.snapshot.Config, nil, method, false, nil).CanStart()
}

// remoteLifecycleBusy prevents authentication changes from racing an exact
// caller-key session mutation. A session create/stop/delete request freezes
// its context until the CLI returns a receipt or an unknown result.
func (a *App) remoteLifecycleBusy() bool {
	return a.remote.createBusy || a.remote.stopBusy || a.remote.deleteBusy
}

func (a *App) remoteCreateConfirmationCurrent() bool {
	if a.remote.createRecovery || a.remote.createBusy || a.remote.stopBusy || a.remote.deleteBusy || a.remote.createGeneration != a.remote.generation || !a.remoteWorkflowAvailable(remote.SessionCreateMethod) {
		return false
	}
	config := a.remote.snapshot.Config
	return config.Home == a.home && config.ServerURL != nil && config.DeviceID != nil &&
		sameConfigContext(config, a.remote.createContext) &&
		remoteCreateInputValid(a.remote.createTool, a.remote.createAccount, a.remote.createWorkspace, a.remote.createProject, a.remote.createDigest, a.remote.createSyncTarget)
}

func (a *App) remoteCreateQueryCurrent() bool {
	if a.remote.createQueryKey == "" || a.remote.createBusy || a.remote.stopBusy || a.remote.deleteBusy || a.remote.createGeneration != a.remote.generation || !a.remoteWorkflowAvailable(remote.SessionCreateRequestMethod) {
		return false
	}
	config := a.remote.snapshot.Config
	return config.Home == a.home && config.ServerURL != nil && config.DeviceID != nil && sameConfigContext(config, a.remote.createContext)
}

// Keep the UI fail-closed before invoking the CLI. The Rust boundary remains
// authoritative; this only catches fields that are structurally known to be
// required and avoids launching a guaranteed-invalid request.
func remoteCreateInputValid(tool, account, workspaceID, project, digest, target string) bool {
	for _, value := range []string{tool, account, workspaceID, project, digest} {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	digest = strings.TrimSpace(digest)
	digestHex := strings.TrimPrefix(digest, "sha256:")
	if !strings.HasPrefix(digest, "sha256:") || len(digestHex) != 64 || digestHex != strings.ToLower(digestHex) {
		return false
	}
	if _, err := hex.DecodeString(digestHex); err != nil {
		return false
	}
	return remote.SessionCreateSyncTargetValid([]byte(target))
}

func (a *App) openRemote() {
	a.page = "remote"
	a.remote.recovery = nil
	a.remote.recoveryNotice = nil
	a.refreshRemote()
}

func (a *App) refreshRemote() {
	if a.remote.busy || a.closing || a.quitting.Load() {
		return
	}
	a.clearRemoteAttach()
	ctx, cancel := context.WithCancel(context.Background())
	a.remote.cancel = cancel
	a.remote.done = make(chan struct{})
	a.remote.results = make(chan remoteResult, 1)
	a.remote.busy = true
	a.remote.err = nil
	results, done, home := a.remote.results, a.remote.done, a.home
	generation := a.remote.generation
	sessionFilter := remote.SessionListFilter{ToolType: a.remote.sessionToolFilter, Statuses: append([]string(nil), a.remote.sessionStatusFilters...)}
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var snapshot remote.Overview
		if err == nil {
			snapshot, err = remote.LoadWithSessionFilter(ctx, cliadapter.Client{Binary: binary, Home: home}, sessionFilter)
		}
		results <- remoteResult{generation, snapshot, err}
	}()
}

func (a *App) pollRemote(c *ui.Context) {
	if !a.remote.busy {
		return
	}
	select {
	case result := <-a.remote.results:
		a.remote.busy = false
		if result.generation != a.remote.generation {
			return
		}
		a.remote.loaded = true
		a.remote.snapshot, a.remote.err = result.snapshot, result.err
		if result.err == nil {
			a.observeRemoteSessionFilterOptions(result.snapshot.Sessions)
			if !a.remote.sessionFilterDirty {
				a.resetRemoteSessionFilterDraft()
			}
		}
		if result.err != nil {
			a.pendingPage = ""
		} else if a.pendingPage == "login" {
			a.pendingPage = ""
			a.openLogin()
		} else if a.pendingPage == "logout" {
			a.pendingPage = ""
			a.openLogout()
		} else if a.pendingPage == "network-forwards" {
			a.pendingPage = ""
			a.page = "network"
			a.network.section = "forwards"
			a.readPortForwards()
		} else if a.pendingPage == "remote-accounts" {
			a.pendingPage = ""
			a.page = "remote"
			a.remote.tab = "accounts"
		} else if a.pendingPage == "remote-sessions" {
			a.pendingPage = ""
			a.page = "remote"
			a.remote.tab = ""
		} else if a.pendingPage == "remote-create" {
			a.pendingPage = ""
			a.page = "remote"
			a.remote.tab = ""
			// The command catalog gate normally prevents this route when the
			// current CLI lacks sessions.create. Keep the asynchronous hand-off
			// fail-closed as well so a capability/context change during loading
			// cannot open an unsubmitable create review.
			if a.remoteWorkflowAvailable(remote.SessionCreateMethod) {
				a.openRemoteCreate()
			}
		}
		a.reviewRemoteRecovery()
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) observeRemoteSessionFilterOptions(sessions []cliadapter.RemoteSession) {
	seenTools := make(map[string]bool, len(a.remote.sessionToolOptions)+len(sessions))
	for _, tool := range append([]string{"claude", "codex", "fclaude"}, a.remote.sessionToolOptions...) {
		if tool != "" && remote.ValidateSessionListFilter(remote.SessionListFilter{ToolType: tool}) == nil {
			seenTools[tool] = true
		}
	}
	seenStatuses := make(map[string]bool, len(a.remote.sessionStatusOptions)+len(sessions))
	for _, status := range a.remote.sessionStatusOptions {
		if remote.ValidateSessionListFilter(remote.SessionListFilter{Statuses: []string{status}}) == nil {
			seenStatuses[status] = true
		}
	}
	for _, session := range sessions {
		if remote.ValidateSessionListFilter(remote.SessionListFilter{ToolType: session.Tool}) == nil {
			seenTools[session.Tool] = true
		}
		if remote.ValidateSessionListFilter(remote.SessionListFilter{Statuses: []string{session.Status}}) == nil {
			seenStatuses[session.Status] = true
		}
	}
	a.remote.sessionToolOptions = a.remote.sessionToolOptions[:0]
	for tool := range seenTools {
		a.remote.sessionToolOptions = append(a.remote.sessionToolOptions, tool)
	}
	sort.Strings(a.remote.sessionToolOptions)
	a.remote.sessionStatusOptions = a.remote.sessionStatusOptions[:0]
	for status := range seenStatuses {
		a.remote.sessionStatusOptions = append(a.remote.sessionStatusOptions, status)
	}
	sort.Strings(a.remote.sessionStatusOptions)
}

func (a *App) resetRemoteSessionFilterDraft() {
	a.remote.sessionToolDraft = a.remote.sessionToolFilter
	a.remote.sessionStatusDraft = make(map[string]bool, len(a.remote.sessionStatusFilters))
	for _, status := range a.remote.sessionStatusFilters {
		a.remote.sessionStatusDraft[status] = true
	}
	a.remote.sessionFilterDirty = false
}

func (a *App) applyRemoteSessionFilter() {
	statuses := make([]string, 0, len(a.remote.sessionStatusDraft))
	for status, selected := range a.remote.sessionStatusDraft {
		if selected {
			statuses = append(statuses, status)
		}
	}
	sort.Strings(statuses)
	filter := remote.SessionListFilter{ToolType: a.remote.sessionToolDraft, Statuses: statuses}
	if remote.ValidateSessionListFilter(filter) != nil {
		a.remote.err = fault.CLIProtocol
		return
	}
	a.remote.sessionToolFilter = filter.ToolType
	a.remote.sessionStatusFilters = statuses
	a.remote.sessionFilterDirty = false
	a.refreshRemote()
}

func (a *App) clearRemoteSessionFilter() {
	a.remote.sessionToolDraft = ""
	a.remote.sessionStatusDraft = nil
	a.remote.sessionToolFilter = ""
	a.remote.sessionStatusFilters = nil
	a.remote.sessionFilterDirty = false
	a.refreshRemote()
}

// invalidateRemoteContext discards data and confirmations captured before an
// authentication mutation finished. Keep the canceled worker's handles until it
// exits; polling can collect it and Close can still wait for process cleanup.
func (a *App) invalidateRemoteContext() {
	a.invalidateSyncContext()
	a.invalidateAccountContext()
	a.invalidateAccountImport()
	a.invalidateRuntimeRecovery()
	a.invalidateAccountDefault()
	a.invalidateWireGuardPlan()
	// Network observations and port-forward actions are scoped to the
	// authenticated home/server/device context as well. Cancel and invalidate
	// them together with the remote inventory so a late response from the old
	// identity cannot appear in the newly unauthenticated/logged-in workspace.
	a.invalidateNetwork()
	a.invalidateCapabilities()
	a.invalidateCredentials()
	a.invalidateSkills()
	a.remote.generation++
	if a.remote.createCancel != nil {
		a.remote.createCancel()
	}
	a.remote.snapshot = remote.Overview{}
	a.remote.loaded = false
	a.remote.err = fault.CLIContextChanged
	a.clearRemoteAttach()
	a.remote.recovery = nil
	a.remote.recoveryNotice = nil
	if a.remote.busy && a.remote.cancel != nil {
		a.remote.cancel()
	}
}
