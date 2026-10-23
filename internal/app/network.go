package app

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/google/uuid"
)

type networkPage struct {
	section, configMode, config string
	action                      string
	planDetails                 bool
	check                       networkQuery[remote.WireGuardCheckReport]
	status                      networkQuery[remote.WireGuardStatusReport]
	bridge                      networkQuery[remote.WireGuardBridgeStatus]
	plan                        networkQuery[remote.WireGuardTunnelPlan]
	tunnelApply                 networkQuery[remote.WireGuardTunnelReceipt]
	interfaceIndex, peerPage    int
	forwards                    networkQuery[remote.PortForwardList]
	forwardID, forwardSession   string
	forwardExpectedNode         string
	forwardExpectedBackend      string
	forwardCallerRequest        string
	forwardClientInstance       string
	forwardRemotePort           string
	forwardLocalPort            string
	forwardTTL                  string
	forwardActionBusy           bool
	forwardActionErr            error
	forwardActionReceipt        *remote.PortForwardReceipt
	forwardActionResults        chan networkForwardActionResult
	forwardActionGeneration     uint64
	forwardActionUncertain      bool
	forwardActionCancel         context.CancelFunc
	forwardActionDone           chan struct{}
	forwardRunners              map[string]*remote.ForwardRunner
	// forwardContexts preserves the exact authentication context used when a
	// local runner was created. Explicit application exit may happen after the
	// user changes account/server/device context; stopping the remote forward
	// must still use the original owner-scoped context.
	forwardContexts  map[string]cliadapter.ConfigContext
	forwardRunnersMu sync.Mutex
	// forwardStops tracks asynchronous local runner shutdowns initiated by a
	// receipt poll. App.Close waits for these goroutines before closing the
	// store, otherwise a close racing a stop receipt could leave a child
	// process alive after the desktop has torn down its state.
	forwardStops   sync.WaitGroup
	forwardClosing atomic.Bool
}

type networkForwardActionResult struct {
	receipt    remote.PortForwardReceipt
	err        error
	generation uint64
}

func (a *App) openNetwork() {
	a.page = "network"
	if a.network.action == "" {
		a.network.action = "up"
	}
}

// openPortForwards is the command-catalog entry point for fclaude forward.
// The forwarding page needs the authenticated remote snapshot before it can
// issue its context-bound list request, so retain a pending route while that
// snapshot is loaded instead of showing an empty, apparently usable form.
func (a *App) openPortForwards() {
	if !a.remote.loaded || a.remote.busy || a.remote.err != nil {
		a.pendingPage = "network-forwards"
		a.openRemote()
		return
	}
	a.page = "network"
	a.network.section = "forwards"
	a.readPortForwards()
}

func (a *App) networkContext() cliadapter.ConfigContext {
	return a.remote.snapshot.Config
}

func (a *App) networkBlocked() bool {
	// Network observations and mutations are scoped to the current CLI
	// context. Keep them from starting while authentication, setup, or another
	// context-changing workflow is in flight; invalidateNetwork also handles a
	// race where one was already running when the context changed.
	return a.wireguard.busy || a.setup.busy || a.network.tunnelApply.busy || a.login.busy || a.logout.busy || a.remote.busy || a.synchronizationBusy() || a.account.busy || a.accountImport.busy || a.runtimeRecovery.busy || a.accountDefault.busy || a.closing || a.quitting.Load()
}

func (a *App) checkNetworkConfig() {
	if a.network.check.busy || a.networkBlocked() {
		return
	}
	input := remote.WireGuardCheckInput{ExpectedHome: a.home}
	if a.network.configMode == "custom" {
		if !filepath.IsAbs(a.network.config) {
			return
		}
		config := a.network.config
		input.Config = &config
	}
	a.network.section = ""
	home, find := a.home, a.findExecutable
	a.network.check.start(func(ctx context.Context) (remote.WireGuardCheckReport, error) {
		binary, err := find("agent-remote")
		if err != nil {
			return remote.WireGuardCheckReport{}, err
		}
		return remote.CheckWireGuard(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
	})
}

func (a *App) readNetworkStatus() {
	if a.network.status.busy || a.networkBlocked() {
		return
	}
	a.network.section, a.network.interfaceIndex, a.network.peerPage = "status", 0, 0
	home, find := a.home, a.findExecutable
	a.network.status.start(func(ctx context.Context) (remote.WireGuardStatusReport, error) {
		binary, err := find("agent-remote")
		if err != nil {
			return remote.WireGuardStatusReport{}, err
		}
		return remote.InspectWireGuardStatus(ctx, cliadapter.Client{Binary: binary, Home: home})
	})
}

func (a *App) readWireGuardBridgeStatus() {
	if a.network.bridge.busy || a.networkBlocked() {
		return
	}
	a.network.section = "bridge"
	home, find := a.home, a.findExecutable
	a.network.bridge.start(func(ctx context.Context) (remote.WireGuardBridgeStatus, error) {
		binary, err := find("agent-remote")
		if err != nil {
			return remote.WireGuardBridgeStatus{}, err
		}
		return remote.InspectWireGuardBridge(ctx, cliadapter.Client{Binary: binary, Home: home})
	})
}

func (a *App) selectNetworkConfig() {
	if a.network.check.busy || a.network.plan.busy {
		return
	}
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: locales.WireGuardConfigPath()})
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	if len(paths) != 0 {
		a.network.config, a.network.configMode = paths[0], "custom"
		a.invalidateNetworkConfig()
	}
}

func (a *App) pollNetwork(c *ui.Context) {
	a.network.check.poll(c)
	a.network.status.poll(c)
	a.network.bridge.poll(c)
	a.network.plan.poll(c)
	a.network.tunnelApply.poll(c)
	if a.network.tunnelApply.err != nil && wireGuardTunnelApplyInvalidatesPlan(a.network.tunnelApply.err) && !a.network.plan.stale {
		// A helper may have changed the interface before the observation was
		// lost, or the CLI may have re-observed a changed plan/precondition.
		// Do not allow the old reviewed plan to be replayed in any of those
		// cases; require a fresh native observation before another mutation.
		a.network.plan.invalidate()
		a.network.planDetails = false
	}
	a.network.forwards.poll(c)
	if a.network.forwardActionBusy && a.network.forwardActionResults != nil {
		select {
		case result := <-a.network.forwardActionResults:
			a.network.forwardActionBusy = false
			a.network.forwardActionCancel = nil
			if result.generation != a.network.forwardActionGeneration {
				// Authentication/configuration invalidation canceled a previous
				// request. Its result must never repopulate the new context or
				// recreate a local listener from a stale receipt.
				return
			}
			a.network.forwardActionErr = result.err
			a.network.forwardActionUncertain = errors.Is(result.err, fault.PortForwardOutcomeUnknown)
			var operation *cliadapter.OperationError
			if errors.As(result.err, &operation) && (operation.Code == "PORT_FORWARD_SUBMISSION_UNKNOWN" || operation.Code == "PORT_FORWARD_RECONNECT_UNKNOWN") {
				a.network.forwardActionUncertain = true
			}
			if result.err == nil {
				a.network.forwardActionReceipt = &result.receipt
				if result.receipt.Forward.Status == "stopped" || result.receipt.Forward.Status == "expired" || result.receipt.Forward.Status == "revoked" {
					a.stopLocalPortForward(result.receipt.Forward.ID)
				}
			}
			a.network.forwards.invalidate()
		default:
			c.After(50 * time.Millisecond)
		}
	}
}

func wireGuardTunnelApplyInvalidatesPlan(err error) bool {
	return errors.Is(err, fault.WireGuardOutcomeUnknown) ||
		errors.Is(err, fault.WireGuardPlanStale) ||
		errors.Is(err, fault.WireGuardPreconditionFailed)
}

func (a *App) applyTunnelPlan(plan remote.WireGuardTunnelPlan) {
	if a.network.tunnelApply.busy || a.networkBlocked() || len(plan.Issues) != 0 {
		return
	}
	home, find := a.home, a.findExecutable
	a.network.tunnelApply.start(func(ctx context.Context) (remote.WireGuardTunnelReceipt, error) {
		binary, err := find("agent-remote")
		if err != nil {
			return remote.WireGuardTunnelReceipt{}, err
		}
		return remote.ApplyWireGuardTunnel(ctx, cliadapter.Client{Binary: binary, Home: home}, plan)
	})
}

// cancelTunnelApply handles the write-side cancellation boundary separately
// from read-only network queries. Once a confirmed tunnel request has been
// handed to the CLI, cancellation can race the native helper and the desktop
// cannot prove whether the interface changed. Preserve that uncertainty and
// invalidate the reviewed plan so the user must inspect and preview again
// before another mutation can be submitted.
func (a *App) cancelTunnelApply() {
	query := &a.network.tunnelApply
	if !query.busy {
		return
	}
	query.generation++
	if query.cancel != nil {
		query.cancel()
	}
	query.report = nil
	query.err = fault.WireGuardOutcomeUnknown
	query.stale = true
	a.network.plan.invalidate()
	a.network.planDetails = false
}

func (a *App) invalidateNetwork() {
	hadForwardAction := a.network.forwardActionBusy || a.network.forwardActionReceipt != nil || a.network.forwardActionErr != nil
	a.network.check.invalidate()
	a.network.status.invalidate()
	a.network.bridge.invalidate()
	a.invalidateTunnelPlan()
	a.network.forwards.invalidate()
	a.network.forwardActionGeneration++
	// A receipt is scoped to the same authenticated context as the list and
	// runner. Keep only a stable error marker after invalidation; retaining the
	// old receipt would make the operation center or a reopened page present a
	// completed forward from another identity as current.
	a.network.forwardActionReceipt = nil
	// These values form the reviewed forward intent. They are scoped to the
	// same remote context and must not survive an authentication or network
	// configuration change into a newly selected session.
	a.network.forwardID = ""
	a.network.forwardSession = ""
	a.network.forwardExpectedNode = ""
	a.network.forwardExpectedBackend = ""
	a.network.forwardCallerRequest = ""
	a.network.forwardClientInstance = ""
	a.network.forwardRemotePort = ""
	a.network.forwardLocalPort = ""
	a.network.forwardTTL = ""
	if hadForwardAction {
		a.network.forwardActionErr = fault.CLIContextChanged
		a.network.forwardActionUncertain = false
	}
	if a.network.forwardActionBusy && a.network.forwardActionCancel != nil {
		a.network.forwardActionCancel()
	}
}

func (a *App) invalidateNetworkConfig() {
	a.network.check.invalidate()
	a.invalidateTunnelPlan()
}

func (a *App) readPortForwards() {
	if a.network.forwards.busy || a.networkBlocked() {
		return
	}
	expected := a.networkContext()
	home, find := a.home, a.findExecutable
	a.network.forwards.start(func(ctx context.Context) (remote.PortForwardList, error) {
		binary, err := find("agent-remote")
		if err != nil {
			return remote.PortForwardList{}, err
		}
		return remote.ListPortForwards(ctx, cliadapter.Client{Binary: binary, Home: home}, expected)
	})
}

func (a *App) submitPortForwardCreate() {
	remotePort, errRemote := strconv.ParseUint(a.network.forwardRemotePort, 10, 16)
	localPort, errLocal := remote.ResolveLocalPort(a.network.forwardLocalPort)
	if errRemote != nil || errLocal != nil || remotePort == 0 || localPort == 0 || a.network.forwardSession == "" {
		a.network.forwardActionErr = fault.PortForwardInputInvalid
		return
	}
	for _, session := range a.remote.snapshot.Sessions {
		if session.ID == a.network.forwardSession {
			a.network.forwardExpectedNode = session.NodeID
			a.network.forwardExpectedBackend = session.Backend
			break
		}
	}
	if a.network.forwardExpectedNode == "" || a.network.forwardExpectedBackend == "" {
		a.network.forwardActionErr = fault.PortForwardInputInvalid
		return
	}
	if a.network.forwardCallerRequest == "" {
		a.network.forwardCallerRequest = uuid.NewString()
	}
	if a.network.forwardClientInstance == "" {
		a.network.forwardClientInstance = uuid.NewString()
	}
	var ttl *uint64
	if value := a.network.forwardTTL; value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed < 60 || parsed > 604800 {
			a.network.forwardActionErr = fault.PortForwardInputInvalid
			return
		}
		ttl = &parsed
	}
	input := remote.PortForwardCreateInput{SchemaVersion: 1, Expected: a.networkContext(), SessionID: a.network.forwardSession, ExpectedNodeID: a.network.forwardExpectedNode, ExpectedRuntimeBackend: a.network.forwardExpectedBackend, RemotePort: uint16(remotePort), LocalPort: uint16(localPort), ClientInstanceID: a.network.forwardClientInstance, CallerRequestID: a.network.forwardCallerRequest, TTLSeconds: ttl, Confirmed: true}
	a.startPortForwardRunner(input)
}

// startPortForwardRunner starts the Rust CLI's real local listener. The
// desktop-control API remains available for inspection and exact stop/
// reconnect operations, while this child owns the actual TCP listener and
// bounded tunnel reconnect loop.
func (a *App) startPortForwardRunner(input remote.PortForwardCreateInput) {
	var ttl *uint64
	if input.TTLSeconds != nil {
		v := *input.TTLSeconds
		ttl = &v
	}
	expectedServer, expectedDevice := "", ""
	if input.Expected.ServerURL != nil {
		expectedServer = *input.Expected.ServerURL
	}
	if input.Expected.DeviceID != nil {
		expectedDevice = *input.Expected.DeviceID
	}
	a.startPortForwardProcess(remote.ForwardLaunchInput{
		SessionID: input.SessionID, RemotePort: input.RemotePort, LocalPort: input.LocalPort, TTLSeconds: ttl,
		ExpectedServerURL: expectedServer, ExpectedDeviceID: expectedDevice, ExpectedNodeID: input.ExpectedNodeID,
		ExpectedRuntimeBackend: input.ExpectedRuntimeBackend, ClientInstanceID: input.ClientInstanceID, CallerRequestID: input.CallerRequestID,
	}, input.Expected, input.CallerRequestID)
}

// startPortForwardResume rehydrates one exact forward after the desktop or
// its child process was restarted. The Rust CLI performs the one-shot
// reconnect and starts the listener; no create request is sent.
func (a *App) startPortForwardResume(item remote.PortForward) {
	if item.ID == "" || item.SessionID == "" || item.NodeID == "" || item.RequestedLocalPort == 0 || item.RemotePort == 0 {
		a.network.forwardActionErr = fault.PortForwardInputInvalid
		return
	}
	expectedBackend := ""
	for _, session := range a.remote.snapshot.Sessions {
		if session.ID == item.SessionID {
			if session.NodeID != item.NodeID || session.Backend == "" {
				a.network.forwardActionErr = fault.PortForwardInputInvalid
				return
			}
			expectedBackend = session.Backend
			break
		}
	}
	if expectedBackend == "" {
		a.network.forwardActionErr = fault.PortForwardInputInvalid
		return
	}
	expected := a.networkContext()
	expectedServer, expectedDevice := "", ""
	if expected.ServerURL != nil {
		expectedServer = *expected.ServerURL
	}
	if expected.DeviceID != nil {
		expectedDevice = *expected.DeviceID
	}
	// Resume is keyed solely by the exact server-side forward UUID. Session,
	// remote-port, TTL and caller-key values are create-time selectors and must
	// never be mixed into the reconnect argv. Node/client metadata remains a
	// local startup-event binding check.
	a.startPortForwardProcess(remote.ForwardLaunchInput{
		ForwardID: item.ID, LocalPort: item.RequestedLocalPort,
		ExpectedServerURL: expectedServer, ExpectedDeviceID: expectedDevice, ExpectedNodeID: item.NodeID, ExpectedRuntimeBackend: expectedBackend,
		ClientInstanceID: item.ClientInstanceID,
	}, expected, item.ID)
}

func (a *App) startPortForwardProcess(launch remote.ForwardLaunchInput, expected cliadapter.ConfigContext, requestID string) {
	if a.network.forwardActionBusy || a.networkBlocked() {
		return
	}
	a.network.forwardActionBusy = true
	a.network.forwardActionErr = nil
	a.network.forwardActionReceipt = nil
	a.network.forwardActionResults = make(chan networkForwardActionResult, 1)
	results := a.network.forwardActionResults
	ctx, cancel := context.WithCancel(context.Background())
	a.network.forwardActionCancel = cancel
	a.network.forwardActionDone = make(chan struct{})
	done := a.network.forwardActionDone
	generation := a.network.forwardActionGeneration
	home, find := a.home, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.PortForwardReceipt
		var runner *remote.ForwardRunner
		if err == nil {
			runner, err = remote.StartForward(ctx, binary, home, launch)
		}
		if err == nil {
			// Close cancels the launch worker before collecting existing
			// runners. A startup event can still win that cancellation race;
			// never publish such a runner into the map after teardown has
			// started, because the close path would otherwise miss it.
			if ctx.Err() != nil || a.network.forwardClosing.Load() {
				_ = runner.Stop(context.Background())
				err = context.Canceled
			}
		}
		if err == nil {
			listener := runner.Listener()
			a.network.forwardRunnersMu.Lock()
			if a.network.forwardRunners == nil {
				a.network.forwardRunners = make(map[string]*remote.ForwardRunner)
			}
			if a.network.forwardContexts == nil {
				a.network.forwardContexts = make(map[string]cliadapter.ConfigContext)
			}
			a.network.forwardRunners[listener.Forward.ID] = runner
			a.network.forwardContexts[listener.Forward.ID] = expected
			a.network.forwardRunnersMu.Unlock()
			receipt = remote.PortForwardReceipt{SchemaVersion: 1, RequestID: requestID, Context: expected, Forward: listener.Forward}
		}
		results <- networkForwardActionResult{receipt: receipt, err: err, generation: generation}
	}()
}

func (a *App) submitPortForwardAction(reconnect bool) {
	if a.network.forwardID == "" {
		a.network.forwardActionErr = fault.PortForwardInputInvalid
		return
	}
	if reconnect {
		if a.network.forwards.report == nil {
			a.network.forwardActionErr = fault.PortForwardInputInvalid
			return
		}
		if !remote.PortForwardReconnectSupported(a.remote.snapshot.Capabilities) {
			a.network.forwardActionErr = &cliadapter.OperationError{Code: "unsupported_method"}
			return
		}
		for _, item := range a.network.forwards.report.Items {
			if item.ID != a.network.forwardID {
				continue
			}
			if item.Status != "active" && item.Status != "pending" && item.Status != "disconnected" {
				a.network.forwardActionErr = fault.PortForwardInputInvalid
				return
			}
			if _, ok := a.localPortForward(item.ID); ok {
				a.network.forwardActionErr = fault.PortForwardInputInvalid
				return
			}
			// An exited runner is only a stale in-memory marker. Remove it so
			// an explicit reconnect can rehydrate the exact server forward.
			if a.hasLocalPortForward(item.ID) {
				a.stopLocalPortForward(item.ID)
			}
			a.startPortForwardResume(item)
			return
		}
		a.network.forwardActionErr = fault.PortForwardInputInvalid
		return
	}
	input := remote.PortForwardActionInput{SchemaVersion: 1, Expected: a.networkContext(), ForwardID: a.network.forwardID, Confirmed: true, ExpectedSessionID: a.network.forwardSession, ExpectedNodeID: a.network.forwardExpectedNode}
	a.runPortForwardAction(func(ctx context.Context, client cliadapter.Client) (remote.PortForwardReceipt, error) {
		return remote.StopPortForward(ctx, client, input)
	})
}

func (a *App) stopLocalPortForward(id string) {
	a.network.forwardRunnersMu.Lock()
	runner := a.network.forwardRunners[id]
	delete(a.network.forwardRunners, id)
	delete(a.network.forwardContexts, id)
	a.network.forwardRunnersMu.Unlock()
	if runner == nil {
		return
	}
	if a.network.forwardClosing.Load() {
		// App.Close owns the shutdown barrier once closing begins. Avoid adding
		// to a WaitGroup that may already be draining and stop this late runner
		// directly instead.
		_ = runner.Stop(context.Background())
		return
	}
	a.network.forwardStops.Add(1)
	go func() {
		defer a.network.forwardStops.Done()
		_ = runner.Stop(context.Background())
	}()
}

func (a *App) localPortForward(id string) (remote.ForwardListener, bool) {
	a.network.forwardRunnersMu.Lock()
	runner := a.network.forwardRunners[id]
	a.network.forwardRunnersMu.Unlock()
	if runner == nil {
		return remote.ForwardListener{}, false
	}
	listener := runner.Listener()
	if listener.State == "exited" || listener.State == "stopped" {
		return listener, false
	}
	return listener, true
}

func (a *App) hasLocalPortForward(id string) bool {
	a.network.forwardRunnersMu.Lock()
	defer a.network.forwardRunnersMu.Unlock()
	_, ok := a.network.forwardRunners[id]
	return ok
}

func (a *App) stopLocalPortForwards() error {
	a.network.forwardRunnersMu.Lock()
	runners := a.network.forwardRunners
	contexts := a.network.forwardContexts
	a.network.forwardRunners = make(map[string]*remote.ForwardRunner)
	a.network.forwardContexts = make(map[string]cliadapter.ConfigContext)
	a.network.forwardRunnersMu.Unlock()
	var result error
	for _, runner := range runners {
		listener := runner.Listener()
		if listener.Forward.ID != "" {
			expected := contexts[listener.Forward.ID]
			if expected.Home == "" {
				expected = a.networkContext()
			}
			binary, findErr := a.findExecutable("agent-remote")
			if findErr != nil {
				result = errors.Join(result, findErr)
			} else {
				stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				_, stopErr := remote.StopPortForward(stopCtx, cliadapter.Client{Binary: binary, Home: expected.Home}, remote.PortForwardActionInput{
					SchemaVersion: 1, Expected: expected, ForwardID: listener.Forward.ID,
					ExpectedSessionID: listener.Forward.SessionID, ExpectedNodeID: listener.Forward.NodeID, Confirmed: true,
				})
				cancel()
				result = errors.Join(result, stopErr)
			}
		}
		result = errors.Join(result, runner.Stop(context.Background()))
	}
	// A stop receipt may have removed a runner and started an asynchronous
	// shutdown just before this snapshot. Include that process in the explicit
	// quit barrier as well.
	a.network.forwardStops.Wait()
	return result
}

func (a *App) waitLocalPortForwardStops() {
	a.network.forwardStops.Wait()
}

func (a *App) runPortForwardAction(work func(context.Context, cliadapter.Client) (remote.PortForwardReceipt, error)) {
	if a.network.forwardActionBusy || a.networkBlocked() {
		return
	}
	a.network.forwardActionBusy = true
	a.network.forwardActionErr = nil
	a.network.forwardActionReceipt = nil
	a.network.forwardActionResults = make(chan networkForwardActionResult, 1)
	results := a.network.forwardActionResults
	ctx, cancel := context.WithCancel(context.Background())
	a.network.forwardActionCancel = cancel
	a.network.forwardActionDone = make(chan struct{})
	done := a.network.forwardActionDone
	generation := a.network.forwardActionGeneration
	home, find := a.home, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.PortForwardReceipt
		if err == nil {
			receipt, err = work(ctx, cliadapter.Client{Binary: binary, Home: home})
		}
		// UI polling is intentionally avoided here: this operation is short and
		// the result is published through a one-slot channel owned by the page.
		results <- networkForwardActionResult{receipt: receipt, err: err, generation: generation}
	}()
}

func (a *App) invalidateTunnelPlan() {
	a.network.plan.invalidate()
	a.network.tunnelApply.invalidate()
	a.network.planDetails = false
}
