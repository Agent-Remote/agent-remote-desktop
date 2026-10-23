package app

import (
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
)

func (a *App) newSession(target string, vertical bool) {
	if a.project() == nil {
		a.projectOpen = true
		return
	}
	if tab := a.state.Board.Current(a.project().ID); target != "" && tab != nil && len(tab.Root.Leaves()) >= 4 {
		a.message = locales.SplitLimit
		return
	}
	a.splitTarget = target
	a.splitVertical = vertical
	a.newArgs = ""
	a.newName = ""
	a.message = nil
	a.newOpen = true
}

func (a *App) focusSession(id string) {
	if saved, ok := a.savedSession(id); ok {
		a.state.Preferences.SelectedProject = saved.ProjectID
		a.page = ""
		title := saved.Name
		if title == "" {
			title = saved.Tool
		}
		a.state.Board.Show(saved.ProjectID, id, title)
		a.focusPane(id, true)
		a.persist()
	}
}

func (a *App) removeSession() {
	id := a.stopID
	force := a.stopForce
	a.stopOpen = false
	go func() {
		var err error
		for _, r := range a.sessions.Records() {
			if r.ID == id {
				if host, ok := r.Process.(*terminalhost.Host); ok {
					err = host.Stop(force)
				}
			}
		}
		if err == nil {
			err = a.sessions.Remove(id)
		}
		mygo.RunOnMain(func() {
			// A quit can begin while the explicit stop is still waiting for a
			// process. Its callback may be delivered after teardown starts, so never
			// persist into a closed save queue or reopen a modal during shutdown.
			if a.closing {
				return
			}
			if err != nil {
				a.message = presentation.Notice(err)
				if errors.Is(err, terminalhost.ErrStopTimeout) {
					a.stopID = id
					a.stopForce = true
					a.stopOpen = true
				}
			} else {
				a.forgetSession(id)
				a.stopForce = false
			}
			if a.win != nil {
				a.win.Invalidate()
			}
		})
	}()
}

func (a *App) hideSession(id string) {
	p := a.project()
	if p == nil {
		return
	}
	a.state.Board.Hide(id)
	a.restoreFocus()
	a.persist()
}

func (a *App) confirmQuit() {
	if a.starting {
		a.message = locales.WaitForStart
		a.quitOpen = false
		return
	}
	a.quitting.Store(true)
	force := a.quitForce
	records := a.sessions.Records()
	plan := PlanConfirmedQuit()
	go func() {
		results := make(chan error, len(records))
		if plan.StopManagedSessions {
			for _, r := range records {
				go func() {
					if host, ok := r.Process.(*terminalhost.Host); ok {
						results <- host.Stop(force)
					} else {
						results <- r.Process.Close()
					}
				}()
			}
		}
		var stopErr error
		if plan.StopManagedSessions {
			for range records {
				stopErr = errors.Join(stopErr, <-results)
			}
		}
		if plan.StopLocalPortForwards {
			stopErr = errors.Join(stopErr, a.stopLocalPortForwards())
		}
		if stopErr != nil {
			mygo.RunOnMain(func() {
				a.quitting.Store(false)
				a.quitForce = errors.Is(stopErr, terminalhost.ErrStopTimeout)
				a.message = presentation.Notice(stopErr)
				a.quitOpen = true
				if a.win != nil {
					a.win.Invalidate()
				}
			})
			return
		}
		mygo.App.Quit()
	}()
}
