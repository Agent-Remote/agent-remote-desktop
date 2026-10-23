package storage

import (
	"math"
	"slices"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

// Clone freezes a snapshot before handing it to the background writer.
func (s State) Clone() State {
	copy := s
	copy.Projects = slices.Clone(s.Projects)
	copy.Sessions = slices.Clone(s.Sessions)
	for i, session := range copy.Sessions {
		copy.Sessions[i].Remote = session.Remote.Clone()
		copy.Sessions[i].RemoteInfo = session.RemoteInfo.Clone()
		if session.ExitCode != nil {
			value := *session.ExitCode
			copy.Sessions[i].ExitCode = &value
		}
	}
	copy.Board = s.Board.Clone()
	copy.Operations = slices.Clone(s.Operations)
	for i, operation := range copy.Operations {
		copy.Operations[i].Stages = slices.Clone(operation.Stages)
	}
	copy.DedicatedOperations = slices.Clone(s.DedicatedOperations)
	if s.RemoteStopRecovery != nil {
		recovery := *s.RemoteStopRecovery
		copy.RemoteStopRecovery = &recovery
	}
	if s.RemoteCreateRecovery != nil {
		recovery := *s.RemoteCreateRecovery
		copy.RemoteCreateRecovery = &recovery
	}
	if s.RemoteDeleteRecovery != nil {
		recovery := *s.RemoteDeleteRecovery
		recovery.SessionIDs = slices.Clone(s.RemoteDeleteRecovery.SessionIDs)
		copy.RemoteDeleteRecovery = &recovery
	}
	return copy
}

// Validate rejects broken references before writing or displaying a saved layout.
// A directory can have been removed since the last run; checking the filesystem
// belongs to an explicit launch, not to restoration of the user's history.
func (s *State) Validate() error {
	if err := validatePreferences(s.Preferences); err != nil {
		return err
	}
	if s.RemoteStopRecovery != nil && !s.RemoteStopRecovery.Valid() {
		return fault.WorkspaceData
	}
	if s.RemoteCreateRecovery != nil && !s.RemoteCreateRecovery.Valid() {
		return fault.WorkspaceData
	}
	if s.RemoteDeleteRecovery != nil && !s.RemoteDeleteRecovery.Valid() {
		return fault.WorkspaceData
	}
	if len(s.Operations) > 128 {
		return fault.WorkspaceData
	}
	operationIDs := map[string]bool{}
	for _, operation := range s.Operations {
		if !operation.Valid() || operationIDs[operation.ID] {
			return fault.WorkspaceData
		}
		operationIDs[operation.ID] = true
	}
	if len(s.DedicatedOperations) > 128 {
		return fault.WorkspaceData
	}
	dedicatedIDs := map[string]bool{}
	for _, operation := range s.DedicatedOperations {
		if !operation.Valid() || dedicatedIDs[operation.ID] {
			return fault.WorkspaceData
		}
		dedicatedIDs[operation.ID] = true
	}
	projects, slots := map[string]bool{}, map[int]bool{}
	for _, p := range s.Projects {
		if p.ID == "" || p.Path == "" || projects[p.ID] {
			return fault.WorkspaceData
		}
		// A wheel slot is unique and within the wheel; 0 is off the wheel.
		if p.Slot < 0 || p.Slot > workspace.WheelSlots || (p.Slot != 0 && slots[p.Slot]) {
			return fault.WorkspaceData
		}
		projects[p.ID], slots[p.Slot] = true, p.Slot != 0
	}
	sessions := map[string]workspace.Session{}
	for _, session := range s.Sessions {
		if session.ID == "" || session.Tool == "" || session.Directory == "" || !projects[session.ProjectID] {
			return fault.WorkspaceData
		}
		if session.Remote != nil && (session.Tool != "fclaude" || !session.Remote.Valid() || session.Reference != session.Remote.SessionID) {
			return fault.WorkspaceData
		}
		if session.RemoteInfo != nil && (session.Tool != "fclaude" || session.Remote == nil || !session.RemoteInfo.Valid()) {
			return fault.WorkspaceData
		}
		if _, exists := sessions[session.ID]; exists {
			return fault.WorkspaceData
		}
		sessions[session.ID] = session
	}
	tabs, visible := map[string]bool{}, map[string]bool{}
	for _, tab := range s.Board.Tabs {
		if tab == nil || tab.ID == "" || tabs[tab.ID] || !projects[tab.ProjectID] || tab.Root == nil {
			return fault.WorkspaceData
		}
		tabs[tab.ID] = true
		leaves, err := validatePane(tab.Root, tab.ProjectID, sessions, visible, 0)
		if err != nil || leaves > 4 {
			return fault.WorkspaceData
		}
	}
	return nil
}

// validatePreferences keeps persisted UI state within the values understood
// by the workbench. A desktop state file is user-writable metadata, so an
// invalid value must be rejected before it reaches mygo layout or terminal
// configuration. Zero numeric values remain accepted for old/in-memory
// snapshots; Load normalizes those values to the documented defaults.
func validatePreferences(p workspace.Preferences) error {
	switch p.Theme {
	case "", "midnight", "sakura", "daybreak":
	default:
		return fault.WorkspaceData
	}
	if !validPreferenceNumber(p.FontSize, 10, 32) || !validPreferenceNumber(p.SidebarWidth, 180, 400) {
		return fault.WorkspaceData
	}
	if strings.ContainsAny(p.SelectedProject, "\x00\r\n") {
		return fault.WorkspaceData
	}
	return nil
}

func validPreferenceNumber(value, min, max float32) bool {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return false
	}
	// A zero value is the Go representation of an omitted legacy preference.
	if value == 0 {
		return true
	}
	return value >= min && value <= max
}

// restoreSelection repairs disposable selection hints, not session or layout
// data. Save validates without mutating a caller's snapshot.
func (s *State) restoreSelection() {
	for _, tab := range s.Board.Tabs {
		if !slices.Contains(tab.Root.Leaves(), tab.FocusedSession) {
			tab.FocusedSession = tab.Root.Leaves()[0]
		}
	}
	for projectID := range s.Board.Active {
		if s.Board.Current(projectID) == nil {
			delete(s.Board.Active, projectID)
		}
	}
	for _, tab := range s.Board.Tabs {
		if s.Board.Current(tab.ProjectID) == nil {
			s.Board.Select(tab.ID)
		}
	}
	if !slices.ContainsFunc(s.Projects, func(p workspace.Project) bool { return p.ID == s.Preferences.SelectedProject }) {
		s.Preferences.SelectedProject = ""
		if len(s.Projects) > 0 {
			s.Preferences.SelectedProject = s.Projects[0].ID
		}
	}
}

func validatePane(p *workspace.Pane, project string, sessions map[string]workspace.Session, seen map[string]bool, depth int) (int, error) {
	if p == nil || depth > 3 {
		return 0, fault.WorkspaceData
	}
	if p.First == nil && p.Second == nil {
		session, ok := sessions[p.SessionID]
		if !ok || session.ProjectID != project || seen[p.SessionID] {
			return 0, fault.WorkspaceData
		}
		seen[p.SessionID] = true
		return 1, nil
	}
	if p.First == nil || p.Second == nil || p.SessionID != "" || !(p.Ratio > 0 && p.Ratio < 100) {
		return 0, fault.WorkspaceData
	}
	a, err := validatePane(p.First, project, sessions, seen, depth+1)
	if err != nil {
		return 0, err
	}
	b, err := validatePane(p.Second, project, sessions, seen, depth+1)
	return a + b, err
}
