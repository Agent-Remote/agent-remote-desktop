package app

func (a *App) restoreFocus() {
	a.active = ""
	if tab := a.state.Board.Current(a.state.Preferences.SelectedProject); tab != nil {
		a.active = tab.FocusedSession
	}
	a.focusValue = a.active
}

func (a *App) focusPane(id string, request bool) {
	if a.active != id {
		a.active = id
		a.state.Board.Focus(id)
		a.layoutChanged()
	}
	if request {
		a.focusValue = id
	}
}

func (a *App) selectTab(id string) {
	a.page = ""
	a.state.Board.Select(id)
	a.restoreFocus()
	a.persist()
}

func (a *App) cycleTab(delta int) {
	project := a.state.Preferences.SelectedProject
	var ids []string
	current := 0
	for _, tab := range a.state.Board.Tabs {
		if tab.ProjectID == project {
			if a.state.Board.Current(project) == tab {
				current = len(ids)
			}
			ids = append(ids, tab.ID)
		}
	}
	if len(ids) > 0 {
		a.selectTab(ids[(current+delta+len(ids))%len(ids)])
	}
}

func (a *App) cycleProject(delta int) {
	if len(a.state.Projects) == 0 {
		return
	}
	current := 0
	for i, p := range a.state.Projects {
		if p.ID == a.state.Preferences.SelectedProject {
			current = i
			break
		}
	}
	a.selectProject(a.state.Projects[(current+delta+len(a.state.Projects))%len(a.state.Projects)].ID)
}

func (a *App) cyclePane() {
	a.page = ""
	tab := a.state.Board.Current(a.state.Preferences.SelectedProject)
	if tab == nil {
		return
	}
	ids := tab.Root.Leaves()
	for i, id := range ids {
		if id == a.active {
			a.focusPane(ids[(i+1)%len(ids)], true)
			return
		}
	}
	if len(ids) > 0 {
		a.focusPane(ids[0], true)
	}
}
