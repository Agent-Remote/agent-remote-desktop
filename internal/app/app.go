// Package app composes the native workbench and its application lifecycle.
package app

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/platform"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Options supplies platform integration services. A nil resolver uses the
// standard executable discovery, including common GUI installation locations.
type Options struct {
	FindExecutable func(string) (string, error)
}

type App struct {
	findExecutable func(string) (string, error)
	home           string
	store          *storage.Store
	state          storage.State
	// operationJournal contains only the safe projection persisted in
	// storage.State. Exact operation keys and receipts stay in the CLI journal.
	operationJournal    map[string]storage.OperationRecord
	sessions            *session.Manager
	win                 *mygo.Window
	tray                *mygo.Tray
	page                string
	pendingPage         string
	active              string
	message             func() string
	starting            bool
	newOpen             bool
	projectOpen         bool
	projectPath         string
	newTool             session.Tool
	newArgs             string
	newName             string
	splitTarget         string
	splitVertical       bool
	quitOpen            bool
	quitForce           bool
	stopID              string
	stopOpen            bool
	stopForce           bool
	quitting            atomic.Bool
	closing             bool
	saveQueue           chan storage.State
	saveDone            chan struct{}
	savePending         atomic.Int64
	saveError           atomic.Pointer[saveFailure]
	localeOff           func()
	tools               []toolLocation
	focusValue          string
	layoutSaveAt        time.Time
	recoverID           string
	recoverReference    string
	recoverOpen         bool
	paletteOpen         bool
	paletteQuery        string
	remote              remotePage
	accountDefault      accountDefaultPage
	account             accountPage
	accountImport       accountImportPage
	runtimeRecovery     runtimeRecoveryPage
	sync                syncPage
	mutagenEnvironment  mutagenEnvironmentPage
	bindingConnections  map[string]string
	login               loginPage
	logout              logoutPage
	setup               setupPage
	initWizard          initWizardPage
	wireguard           wireguardPage
	network             networkPage
	diagnostics         diagnosticsPage
	dependencies        dependenciesPage
	commandDiagnostics  commandDiagnosticsPage
	capabilities        capabilitiesPage
	device              devicePage
	egoBrowser          egoBrowserPage
	nodes               nodePage
	credentials         credentialsPage
	skills              skillsPage
	genericCLI          genericCLIPage
	dedicatedOperations []dedicatedOperation
	motionFrames        theme.FrameObserver
	launchResults       chan launchResult
	exitResults         chan exitResult
	launches            sync.WaitGroup
}

type saveFailure struct{ err error }

func New(home string) (*App, error) { return NewWithOptions(home, Options{}) }

func NewWithOptions(home string, options Options) (*App, error) {
	if options.FindExecutable == nil {
		options.FindExecutable = platform.Executable
	}
	s, err := storage.Open(home)
	if err != nil {
		return nil, err
	}
	state, err := s.Load()
	if err != nil {
		s.Close()
		return nil, err
	}
	a := &App{home: home, findExecutable: options.FindExecutable, store: s, state: state, newTool: session.Codex, saveQueue: make(chan storage.State, 1), saveDone: make(chan struct{}), launchResults: make(chan launchResult, 2), exitResults: make(chan exitResult, 32), operationJournal: make(map[string]storage.OperationRecord, len(state.Operations))}
	if recovery := state.RemoteStopRecovery; recovery != nil {
		// Restore only the reviewed public scope and caller key. The exact
		// receipt remains in the CLI journal and is queried only on explicit
		// user action after restart.
		a.remote.stopRequestKey = recovery.RequestID
		a.remote.stopSubmitted = true
		a.remote.stopSession = cliadapter.RemoteSession{
			ID: recovery.SessionID, Tool: "claude", NodeID: recovery.NodeID,
			Backend:    recovery.RuntimeBackend,
			StopTaskID: optionalString(recovery.StopTaskID),
			Status:     "starting",
		}
		a.remote.stopContext = cliadapter.ConfigContext{
			Home: recovery.Home, ServerURL: optionalString(recovery.ServerURL), DeviceID: optionalString(recovery.DeviceID),
		}
	}
	if recovery := state.RemoteCreateRecovery; recovery != nil {
		// Restore only the exact caller key and context. The create dialog is
		// opened explicitly from the remote page; no request is replayed.
		a.remote.createQueryKey = recovery.RequestID
		a.remote.createContext = cliadapter.ConfigContext{
			Home: recovery.Home, ServerURL: optionalString(recovery.ServerURL), DeviceID: optionalString(recovery.DeviceID),
		}
		a.remote.createRecovery = true
	}
	if recovery := state.RemoteDeleteRecovery; recovery != nil {
		// Restore only the exact caller key, authentication context and frozen
		// collection identity. The delete receipt remains owned by the CLI and
		// is queried only after the user explicitly opens this recovery.
		a.remote.deleteRequestKey = recovery.RequestID
		a.remote.deleteSubmitted = true
		a.remote.deleteRecovery = true
		a.remote.deleteContext = cliadapter.ConfigContext{
			Home: recovery.Home, ServerURL: optionalString(recovery.ServerURL), DeviceID: optionalString(recovery.DeviceID),
		}
		a.remote.deleteSelection = remote.SessionDeleteSelection{ToolType: recovery.ToolType, Statuses: []string{"failed", "interrupted", "stopped"}}
		a.remote.deleteDigest = recovery.SelectionDigest
		a.remote.deleteFingerprint = recovery.IntentFingerprint
		a.remote.deleteSelected = make(map[string]bool, len(recovery.SessionIDs))
		for _, id := range recovery.SessionIDs {
			a.remote.deleteSelected[id] = true
			a.remote.deleteSessions = append(a.remote.deleteSessions, cliadapter.RemoteSession{ID: id, Tool: recovery.ToolType, Status: "stopped"})
		}
	}
	recoveredOperation := false
	for _, record := range state.Operations {
		if record.Status == "running" {
			// A process restart cannot prove that the previous worker or CLI
			// mutation is still active. Surface it for explicit review instead
			// of presenting a stale running state or replaying the operation.
			record.Status = "attention"
			record.UpdatedAt = time.Now().UTC()
			recoveredOperation = true
		}
		a.operationJournal[record.ID] = record
	}
	recoveredDedicated := false
	for _, record := range state.DedicatedOperations {
		status := record.Status
		if status == "pending" || status == "running" {
			// A desktop restart cannot prove that a managed terminal or its CLI
			// request survived. Keep the lifecycle summary visible for explicit
			// review instead of presenting it as active or replaying it.
			status = "attention"
			record.Status = status
			record.UpdatedAt = time.Now().UTC()
			recoveredDedicated = true
		}
		a.dedicatedOperations = append(a.dedicatedOperations, dedicatedOperation{
			id: record.ID, command: record.Command, status: status,
			updatedAt: record.UpdatedAt, recovered: true,
		})
	}
	a.network.forwardRunners = make(map[string]*remote.ForwardRunner)
	a.network.forwardContexts = make(map[string]cliadapter.ConfigContext)
	a.sessions = session.New(func(cmd session.Command) (session.Process, error) {
		return terminalhost.New(cmd)
	})
	a.restoreFocus()
	go a.saveWorker()
	if recoveredOperation || recoveredDedicated {
		a.persist()
	}
	return a, nil
}

// MotionStats exposes the view-build timing sample used by diagnostics and
// native acceptance probes. It does not claim to measure GPU paint time.
func (a *App) MotionStats() theme.FrameStats { return a.motionFrames.Snapshot() }

func (a *App) Configure() {
	mygo.App.SetName(locales.AppTitle())
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnActivate(func(visible bool) {
		if !visible {
			a.openWindow()
		}
	})
	mygo.App.OnSecondInstance(func(_ []string, _ string) { a.openWindow() })
	mygo.App.OnBeforeQuit(func(e *mygo.QuitEvent) {
		if DecideQuitRequest(a.quitting.Load()) == QuitRequestAllow {
			return
		}
		e.PreventDefault()
		a.openWindow()
		a.quitOpen = true
		// A quit request can arrive from the single-instance/tray lifecycle
		// before the first window has finished being created. The prompt will
		// render once WhenReady opens it; never dereference a nil window here.
		if a.win != nil {
			a.win.Invalidate()
		}
	})
	a.installMenu()
	a.localeOff = mygo.App.OnLocaleChanged(func(string) {
		a.installMenu()
		if a.win != nil {
			a.win.SetTitle(locales.AppTitle())
		}
		if a.tray != nil {
			a.tray.SetToolTip(locales.AppTitle())
			a.tray.SetMenu(a.trayMenu())
		}
	})
	mygo.App.WhenReady(func() { a.openWindow(); a.installTray() })
}

func (a *App) openWindow() {
	if a.win != nil {
		a.win.Show()
		a.win.Restore()
		a.win.Focus()
		return
	}
	a.win = mygo.NewWindow(mygo.WindowOptions{Title: locales.AppTitle(), Width: 1366, Height: 820, MinWidth: 900, MinHeight: 600, Content: ui.View(a.View), AutoHideMenuBar: true})
	// Native file drops are routed to the focused local Agent terminal. The
	// handler inserts quoted paths as input; it never executes a dropped path.
	a.win.OnFileDrop(a.handleFileDrop)
	a.win.OnClose(func(e *mygo.CloseEvent) {
		if DecideWindowClose(a.quitting.Load()) == WindowCloseHide {
			e.PreventDefault()
			a.persist()
			a.win.Hide()
		}
	})
	a.win.OnFocus(func() {
		if a.page == "remote" {
			a.refreshRemote()
		}
	})
}

func (a *App) project() *workspace.Project {
	for i := range a.state.Projects {
		if a.state.Projects[i].ID == a.state.Preferences.SelectedProject {
			return &a.state.Projects[i]
		}
	}
	return nil
}

func (a *App) selectProject(id string) {
	if id != a.state.Preferences.SelectedProject {
		a.invalidateSyncContext()
	}
	a.state.Preferences.SelectedProject = id
	a.page = ""
	a.restoreFocus()
	a.persist()
}

func (a *App) addProject(path string) {
	p, err := workspace.NewProject(path)
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	for _, existing := range a.state.Projects {
		if existing.Path == p.Path {
			a.selectProject(existing.ID)
			a.projectOpen = false
			return
		}
	}
	a.state.Projects = append(a.state.Projects, p)
	a.selectProject(p.ID)
	a.projectOpen = false
	a.projectPath = ""
	a.message = nil
}

func (a *App) selectDirectory() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: locales.ChooseProjectDirectory(), Directory: true})
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	if len(paths) > 0 {
		a.addProject(paths[0])
	}
}

func (a *App) applyTheme() {
	for _, r := range a.sessions.Records() {
		if t, ok := r.Process.(*terminalhost.Host); ok {
			t.Terminal.SetTheme(theme.Terminal(a.state.Preferences.Theme), nil)
		}
	}
	a.persist()
}
