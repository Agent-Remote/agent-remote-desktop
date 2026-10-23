package session

import (
	"errors"
	"sync"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

// Process is owned by Manager; views may disappear without closing it.
type Process interface {
	Close() error
	Done() <-chan struct{}
	ExitCode() int
}
type Factory func(Command) (Process, error)

type Record struct {
	ID        string
	ProjectID string
	Tool      Tool
	Directory string
	Remote    bool
	Started   time.Time
	Exited    bool
	ExitCode  int
	Process   Process
}

type Manager struct {
	mu      sync.RWMutex
	records []Record
	factory Factory
	closing bool
	limit   int
	pending int
	starts  sync.WaitGroup
	closeMu sync.Mutex
}

func New(factory Factory) *Manager { return &Manager{factory: factory, limit: 24} }

func (m *Manager) Start(projectID string, tool Tool, cmd Command) (Record, error) {
	m.mu.Lock()
	if m.closing {
		m.mu.Unlock()
		return Record{}, fault.Stopping
	}
	if len(m.records)+m.pending >= m.limit {
		m.mu.Unlock()
		return Record{}, fault.SessionLimit
	}
	m.pending++
	m.starts.Add(1)
	m.mu.Unlock()
	defer m.starts.Done()
	// Loading the native library and starting an Agent must not block UI snapshots.
	p, err := m.factory(cmd)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pending--
	if err != nil {
		return Record{}, err
	}
	rec := Record{ID: workspace.NewID(), ProjectID: projectID, Tool: tool, Directory: cmd.Directory, Remote: cmd.Remote, Started: time.Now(), Process: p}
	m.records = append(m.records, rec)
	if m.closing {
		// Close waits for this start and owns cleanup, including a failed stop.
		return Record{}, fault.Stopping
	}
	return rec, nil
}

func (m *Manager) Records() []Record {
	m.mu.RLock()
	defer m.mu.RUnlock()
	records := append([]Record(nil), m.records...)
	for i := range records {
		select {
		case <-records[i].Process.Done():
			records[i].Exited = true
			records[i].ExitCode = records[i].Process.ExitCode()
		default:
		}
	}
	return records
}

func (m *Manager) Remove(id string) error {
	// Serialize removal with shutdown. Without this barrier a close request can
	// snapshot the same process while Remove is stopping it, causing two
	// independent lifecycle owners to call Process.Close concurrently. Native
	// PTY/ConPTY implementations are defensive, but the Manager contract should
	// guarantee one owner and one close even for injected processes.
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	m.mu.Lock()
	var p Process
	for _, r := range m.records {
		if r.ID == id {
			p = r.Process
			break
		}
	}
	m.mu.Unlock()
	if p == nil {
		return fault.SessionMissing
	}
	if err := p.Close(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, r := range m.records {
		if r.ID == id {
			m.records = append(m.records[:i], m.records[i+1:]...)
			break
		}
	}
	return nil
}

func (m *Manager) Close() error {
	_, err := m.CloseWithRecords()
	return err
}

// CloseWithRecords stops every managed process and returns its final lifecycle
// snapshot before the manager releases ownership. Callers that persist session
// history can therefore record an exit code that only becomes available after
// the graceful stop completes.
func (m *Manager) CloseWithRecords() ([]Record, error) {
	m.closeMu.Lock()
	defer m.closeMu.Unlock()
	m.mu.Lock()
	m.closing = true
	m.mu.Unlock()
	m.starts.Wait()
	m.mu.RLock()
	records := append([]Record(nil), m.records...)
	m.mu.RUnlock()
	var result error
	for _, rec := range records {
		result = errors.Join(result, rec.Process.Close())
	}
	final := make([]Record, len(records))
	copy(final, records)
	for i := range final {
		select {
		case <-final[i].Process.Done():
			final[i].Exited = true
			final[i].ExitCode = final[i].Process.ExitCode()
		default:
		}
	}
	m.mu.Lock()
	if result == nil {
		m.records = nil
	} else {
		m.closing = false
	}
	m.mu.Unlock()
	return final, result
}
