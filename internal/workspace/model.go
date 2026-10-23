// Package workspace owns the desktop's project and presentation state.
package workspace

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type Project struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func NewProject(path string) (Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Project{}, err
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return Project{}, err
	}
	s, err := os.Stat(abs)
	if err != nil {
		return Project{}, err
	}
	if !s.IsDir() {
		return Project{}, fault.ProjectDirectory
	}
	return Project{ID: NewID(), Name: filepath.Base(abs), Path: abs}, nil
}

func NewID() string {
	var bytes [16]byte
	_, _ = rand.Read(bytes[:]) // crypto/rand.Read always fills the buffer or terminates.
	return hex.EncodeToString(bytes[:])
}

type Preferences struct {
	Theme     string `json:"theme"`
	FocusMode bool   `json:"focus_mode"`
	// ReduceMotion opts the workbench out of decorative transitions. The
	// native mygo preference is also honored at render time, so platform
	// accessibility settings always take effect even when this remains false.
	ReduceMotion    bool    `json:"reduce_motion,omitempty"`
	FontSize        float32 `json:"font_size"`
	SidebarWidth    float32 `json:"sidebar_width"`
	SelectedProject string  `json:"selected_project"`
	ShowDetails     bool    `json:"show_details,omitempty"`
}

// Session contains only desktop history metadata, never argv, environment,
// terminal output, credentials or an assumption that the process survived.
type Session struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Tool      string         `json:"tool"`
	Name      string         `json:"name,omitempty"`
	Directory string         `json:"directory"`
	Started   time.Time      `json:"started"`
	ExitCode  *int           `json:"exit_code,omitempty"`
	Reference string         `json:"reference,omitempty"`
	Remote    *RemoteContext `json:"remote,omitempty"`
	// RemoteInfo is the public runtime identity observed when an fclaude
	// session was attached. It is deliberately separate from RemoteContext:
	// the latter is the exact reconnect target and must remain comparable to a
	// freshly queried target without embedding mutable runtime placement.
	RemoteInfo *RemoteSessionInfo `json:"remote_info,omitempty"`
}

// RemoteSessionInfo is a safe, metadata-only projection of the remote
// session's account, node and runtime backend. It contains no credentials,
// tokens, command arguments or terminal output and is only used to keep the
// session identity visible after the workbench is hidden or restarted.
type RemoteSessionInfo struct {
	AccountID string `json:"account_id"`
	NodeID    string `json:"node_id"`
	Backend   string `json:"backend"`
}

func (r *RemoteSessionInfo) Clone() *RemoteSessionInfo {
	if r == nil {
		return nil
	}
	copy := *r
	return &copy
}

func Defaults() Preferences { return Preferences{Theme: "midnight", FontSize: 14, SidebarWidth: 240} }

// Pane is a persistent split tree. Leaves reference session IDs, never processes.
type Pane struct {
	SessionID string  `json:"session_id,omitempty"`
	Vertical  bool    `json:"vertical,omitempty"`
	Ratio     float32 `json:"ratio,omitempty"`
	First     *Pane   `json:"first,omitempty"`
	Second    *Pane   `json:"second,omitempty"`
}

func (p *Pane) Leaves() []string {
	if p == nil {
		return nil
	}
	if p.First == nil {
		return []string{p.SessionID}
	}
	return append(p.First.Leaves(), p.Second.Leaves()...)
}

func (p *Pane) Clone() *Pane {
	if p == nil {
		return nil
	}
	copy := *p
	copy.First, copy.Second = p.First.Clone(), p.Second.Clone()
	return &copy
}

func (p *Pane) Replace(previous, next string) bool {
	if p == nil {
		return false
	}
	if p.First != nil {
		return p.First.Replace(previous, next) || p.Second.Replace(previous, next)
	}
	if p.SessionID != previous {
		return false
	}
	p.SessionID = next
	return true
}

func (p *Pane) Split(target, next string, vertical bool) bool {
	if p == nil {
		return false
	}
	if p.First != nil {
		return p.First.Split(target, next, vertical) || p.Second.Split(target, next, vertical)
	}
	if p.SessionID != target {
		return false
	}
	p.First, p.Second = &Pane{SessionID: target}, &Pane{SessionID: next}
	p.SessionID, p.Ratio, p.Vertical = "", 50, vertical
	return true
}

func (p *Pane) Remove(id string) *Pane {
	if p == nil {
		return nil
	}
	if p.First == nil {
		if p.SessionID == id {
			return nil
		}
		return p
	}
	p.First, p.Second = p.First.Remove(id), p.Second.Remove(id)
	if p.First == nil {
		return p.Second
	}
	if p.Second == nil {
		return p.First
	}
	return p
}
