package app

// WindowCloseDecision describes what the native window close event should do.
// A normal close hides the workbench so managed sessions and the tray entry
// remain alive. During an explicit, confirmed application quit the native
// close event is allowed to complete the teardown.
type WindowCloseDecision uint8

const (
	WindowCloseHide WindowCloseDecision = iota
	WindowCloseAllow
)

// DecideWindowClose keeps a normal window close in the background. The
// quitting flag is set only after the user confirms the quit dialog and all
// managed processes have begun their shutdown path.
func DecideWindowClose(quitting bool) WindowCloseDecision {
	if quitting {
		return WindowCloseAllow
	}
	return WindowCloseHide
}

// QuitRequestDecision describes how a native/application quit request is
// handled before the user has confirmed stopping local work.
type QuitRequestDecision uint8

const (
	QuitRequestPrompt QuitRequestDecision = iota
	QuitRequestAllow
)

// DecideQuitRequest makes the first quit request prompt instead of tearing
// down sessions implicitly. Once the confirmed quit path owns teardown,
// subsequent native quit callbacks are allowed through.
func DecideQuitRequest(quitting bool) QuitRequestDecision {
	if quitting {
		return QuitRequestAllow
	}
	return QuitRequestPrompt
}

// ConfirmedQuitPlan records the resources that an explicit quit confirmation
// must stop. It is deliberately independent of the number of sessions: the
// caller still enumerates live records and reports an empty set in the same
// confirmation flow, while local port forwards are always cleaned up.
type ConfirmedQuitPlan struct {
	StopManagedSessions   bool
	StopLocalPortForwards bool
}

// PlanConfirmedQuit returns the fixed cleanup contract for an explicit quit.
// Closing a window never invokes this plan.
func PlanConfirmedQuit() ConfirmedQuitPlan {
	return ConfirmedQuitPlan{
		StopManagedSessions:   true,
		StopLocalPortForwards: true,
	}
}
