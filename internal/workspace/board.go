package workspace

type Tab struct {
	ID             string `json:"id"`
	ProjectID      string `json:"project_id"`
	Title          string `json:"title"`
	Root           *Pane  `json:"root"`
	FocusedSession string `json:"focused_session,omitempty"`
}

// Board keeps independent split layouts per tab, without owning Agent processes.
type Board struct {
	Tabs   []*Tab            `json:"tabs"`
	Active map[string]string `json:"active"`
}

func (b *Board) Current(projectID string) *Tab {
	for _, t := range b.Tabs {
		if t.ProjectID == projectID && t.ID == b.Active[projectID] {
			return t
		}
	}
	return nil
}

func (b *Board) Select(tabID string) {
	if b.Active == nil {
		b.Active = map[string]string{}
	}
	for _, t := range b.Tabs {
		if t.ID == tabID {
			b.Active[t.ProjectID] = tabID
			return
		}
	}
}

func (b *Board) Add(projectID, sessionID, title, target string, vertical bool) *Tab {
	if target != "" {
		for _, t := range b.Tabs {
			if t.ProjectID == projectID && len(t.Root.Leaves()) < 4 && t.Root.Split(target, sessionID, vertical) {
				t.FocusedSession = sessionID
				b.Select(t.ID)
				return t
			}
		}
	}
	t := &Tab{ID: NewID(), ProjectID: projectID, Title: title, Root: &Pane{SessionID: sessionID}, FocusedSession: sessionID}
	b.Tabs = append(b.Tabs, t)
	b.Select(t.ID)
	return t
}

func (b *Board) Show(projectID, sessionID, title string) *Tab {
	for _, t := range b.Tabs {
		if t.ProjectID == projectID {
			for _, id := range t.Root.Leaves() {
				if id == sessionID {
					t.FocusedSession = sessionID
					b.Select(t.ID)
					return t
				}
			}
		}
	}
	return b.Add(projectID, sessionID, title, "", false)
}

func (b *Board) Hide(sessionID string) {
	tabs := b.Tabs[:0]
	for _, t := range b.Tabs {
		t.Root = t.Root.Remove(sessionID)
		if t.Root != nil {
			if t.FocusedSession == sessionID {
				t.FocusedSession = t.Root.Leaves()[0]
			}
			tabs = append(tabs, t)
		}
	}
	b.Tabs = tabs
	for projectID := range b.Active {
		if b.Current(projectID) == nil {
			delete(b.Active, projectID)
		}
	}
	for _, t := range b.Tabs {
		if b.Current(t.ProjectID) == nil {
			b.Select(t.ID)
		}
	}
}

func (b *Board) Clone() Board {
	copy := Board{Active: make(map[string]string, len(b.Active))}
	for key, value := range b.Active {
		copy.Active[key] = value
	}
	for _, tab := range b.Tabs {
		clone := *tab
		clone.Root = tab.Root.Clone()
		copy.Tabs = append(copy.Tabs, &clone)
	}
	return copy
}

func (b *Board) Focus(sessionID string) bool {
	for _, tab := range b.Tabs {
		for _, id := range tab.Root.Leaves() {
			if id == sessionID {
				tab.FocusedSession = id
				return true
			}
		}
	}
	return false
}

// Replace keeps the old pane's location and split ratios when a user launches
// a fresh connection from a saved history entry.
func (b *Board) Replace(previous, next string) bool {
	for _, tab := range b.Tabs {
		if tab.Root.Replace(previous, next) {
			if tab.FocusedSession == previous {
				tab.FocusedSession = next
			}
			return true
		}
	}
	return false
}

// Move reorders tabs within a project. Focus and terminal identities stay intact.
func (b *Board) Move(sourceID, targetID string) bool {
	from, to := -1, -1
	for i, tab := range b.Tabs {
		if tab.ID == sourceID {
			from = i
		}
		if tab.ID == targetID {
			to = i
		}
	}
	if from < 0 || to < 0 || from == to || b.Tabs[from].ProjectID != b.Tabs[to].ProjectID {
		return false
	}
	item := b.Tabs[from]
	if from < to {
		copy(b.Tabs[from:to], b.Tabs[from+1:to+1])
	} else {
		copy(b.Tabs[to+1:from+1], b.Tabs[to:from])
	}
	b.Tabs[to] = item
	return true
}
