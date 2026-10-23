package session_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
)

type fakeProcess struct {
	done   chan struct{}
	once   sync.Once
	closes int
}

func TestSlowStartDoesNotBlockSnapshotsOrEscapeShutdown(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	p := &fakeProcess{done: make(chan struct{})}
	m := session.New(func(session.Command) (session.Process, error) {
		close(entered)
		<-release
		return p, nil
	})
	start := make(chan error, 1)
	go func() { _, err := m.Start("p", session.Codex, session.Command{}); start <- err }()
	<-entered
	snapshot := make(chan []session.Record, 1)
	go func() { snapshot <- m.Records() }()
	select {
	case records := <-snapshot:
		if len(records) != 0 {
			t.Fatal("pending process published")
		}
	case <-time.After(time.Second):
		close(release)
		t.Fatal("a slow start blocked UI snapshots")
	}
	shutdown := make(chan error, 1)
	go func() { shutdown <- m.Close() }()
	// A pending factory must be accounted for even if Close races with its return.
	close(release)
	if err := <-start; err != nil && !errors.Is(err, fault.Stopping) {
		t.Fatal(err)
	}
	select {
	case err := <-shutdown:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown failed to wait for pending process")
	}
	if p.closes != 1 || len(m.Records()) != 0 {
		t.Fatal("process escaped shutdown")
	}
}

func (p *fakeProcess) Close() error          { p.once.Do(func() { p.closes++; close(p.done) }); return nil }
func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) ExitCode() int         { return 7 }

func TestManagerOwnsProcessUntilExplicitRemoval(t *testing.T) {
	p := &fakeProcess{done: make(chan struct{})}
	m := session.New(func(session.Command) (session.Process, error) { return p, nil })
	r, err := m.Start("project", session.Codex, session.Command{})
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		records := m.Records()
		if len(records) != 1 || records[0].Exited || p.closes != 0 {
			t.Fatal("reading view state changed the process")
		}
	}
	if err := m.Remove(r.ID); err != nil {
		t.Fatal(err)
	}
	if p.closes != 1 || len(m.Records()) != 0 {
		t.Fatal("explicit removal did not stop process")
	}
}

func TestShutdownRejectsNewProcesses(t *testing.T) {
	count := 0
	m := session.New(func(session.Command) (session.Process, error) {
		count++
		return &fakeProcess{done: make(chan struct{})}, nil
	})
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start("p", session.Claude, session.Command{}); err == nil || count != 0 {
		t.Fatal("shutdown allowed a new process")
	}
}

func TestCompletedProcessReportsActualExit(t *testing.T) {
	p := &fakeProcess{done: make(chan struct{})}
	m := session.New(func(session.Command) (session.Process, error) { return p, nil })
	_, err := m.Start("p", session.Claude, session.Command{})
	if err != nil {
		t.Fatal(err)
	}
	p.Close()
	if r := m.Records()[0]; !r.Exited || r.ExitCode != 7 {
		t.Fatalf("wrong exit state: %+v", r)
	}
}

func TestConcurrentRemoveAndShutdownOwnProcessExactlyOnce(t *testing.T) {
	p := &fakeProcess{done: make(chan struct{})}
	m := session.New(func(session.Command) (session.Process, error) { return p, nil })
	r, err := m.Start("project", session.Codex, session.Command{})
	if err != nil {
		t.Fatal(err)
	}
	removeDone := make(chan error, 1)
	closeDone := make(chan error, 1)
	go func() { removeDone <- m.Remove(r.ID) }()
	go func() { closeDone <- m.Close() }()
	if err := <-removeDone; err != nil && !errors.Is(err, fault.SessionMissing) {
		t.Fatalf("concurrent remove failed: %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("concurrent shutdown failed: %v", err)
	}
	if p.closes != 1 {
		t.Fatalf("process was closed %d times, want exactly once", p.closes)
	}
}
