// Command performance-probe measures the workbench's Go/mygo view-build
// phase with a persisted ten-session, four-pane layout. It deliberately does
// not claim to measure GPU paint, PTY rendering, or input-to-paint latency;
// those require a native host session and are release evidence gathered by
// the platform acceptance process.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

type report struct {
	Probe              string   `json:"probe"`
	SchemaVersion      int      `json:"schema_version"`
	HostRuntime        bool     `json:"host_runtime"`
	CrossCompilation   bool     `json:"cross_compilation"`
	GOOS               string   `json:"goos"`
	GOARCH             string   `json:"goarch"`
	ViewportWidth      int      `json:"viewport_width"`
	ViewportHeight     int      `json:"viewport_height"`
	PersistedSessions  int      `json:"persisted_sessions"`
	VisiblePaneCount   int      `json:"visible_panes"`
	Frames             uint64   `json:"frames"`
	SampleCount        uint64   `json:"sample_count"`
	SlowFrames         uint64   `json:"slow_frames"`
	ViewBuildP95MS     float64  `json:"view_build_p95_ms"`
	ViewBuildMaxMS     float64  `json:"view_build_max_ms"`
	FrameBudgetMS      float64  `json:"frame_budget_ms"`
	Status             string   `json:"status"`
	MeasurementScope   string   `json:"measurement_scope"`
	InputFeedbackP95   *float64 `json:"input_feedback_p95_ms"`
	TerminalRenderTest bool     `json:"terminal_rendering_measured"`
}

func main() {
	frames := flag.Int("frames", 120, "number of view frames to collect")
	maxP95 := flag.Duration("max-p95", 16*time.Millisecond, "maximum permitted Go/mygo view-build P95")
	flag.Parse()
	if *frames < 32 {
		fail("-frames must be at least 32")
	}
	if *maxP95 <= 0 {
		fail("-max-p95 must be positive")
	}

	home, err := os.MkdirTemp("", "agent-remote-desktop-performance-")
	if err != nil {
		fail(err.Error())
	}
	defer os.RemoveAll(home)
	projectPath := filepath.Join(home, "project")
	if err := os.MkdirAll(projectPath, 0700); err != nil {
		fail(err.Error())
	}
	if err := seed(home, projectPath); err != nil {
		fail(err.Error())
	}

	i18n.SetLocaleFunc(func() string { return "en" })
	defer i18n.SetLocaleFunc(nil)
	desktop, err := app.New(home)
	if err != nil {
		fail(err.Error())
	}
	tester := ui.NewTester(desktop.View, 1366, 768)
	for i := 0; i < *frames; i++ {
		tester.Frame()
	}
	stats := desktop.MotionStats()
	closeErr := desktop.Close()
	if closeErr != nil {
		fail(closeErr.Error())
	}

	result := report{
		Probe:              "agent-remote-desktop.performance",
		SchemaVersion:      1,
		HostRuntime:        true,
		CrossCompilation:   false,
		GOOS:               runtime.GOOS,
		GOARCH:             runtime.GOARCH,
		ViewportWidth:      1366,
		ViewportHeight:     768,
		PersistedSessions:  10,
		VisiblePaneCount:   4,
		Frames:             stats.Frames,
		SampleCount:        stats.SampleCount,
		SlowFrames:         stats.SlowFrames,
		ViewBuildP95MS:     float64(stats.P95) / float64(time.Millisecond),
		ViewBuildMaxMS:     float64(stats.Max) / float64(time.Millisecond),
		FrameBudgetMS:      float64(16*time.Millisecond) / float64(time.Millisecond),
		MeasurementScope:   "go-mygo-view-build",
		TerminalRenderTest: false,
	}
	if result.ViewBuildP95MS <= float64(*maxP95)/float64(time.Millisecond) {
		result.Status = "passed"
	} else {
		result.Status = "failed"
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		fail(err.Error())
	}
	fmt.Println(string(encoded))
	if result.Status != "passed" {
		os.Exit(1)
	}
}

func seed(home, projectPath string) error {
	const projectID = "performance-project"
	state := storage.State{Preferences: workspace.Defaults()}
	state.Preferences.ReduceMotion = true
	state.Preferences.SelectedProject = projectID
	state.Projects = []workspace.Project{{ID: projectID, Name: "performance", Path: projectPath}}
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("performance-session-%02d", i+1)
		state.Sessions = append(state.Sessions, workspace.Session{ID: id, ProjectID: projectID, Tool: "codex", Name: fmt.Sprintf("Codex %02d", i+1), Directory: projectPath, Started: time.Unix(int64(i+1), 0).UTC()})
	}
	state.Board.Add(projectID, state.Sessions[0].ID, "Performance", "", false)
	for i := 1; i < 4; i++ {
		state.Board.Add(projectID, state.Sessions[i].ID, "Performance", state.Sessions[0].ID, i%2 == 0)
	}
	store, err := storage.Open(home)
	if err != nil {
		return err
	}
	if err := store.Save(state); err != nil {
		_ = store.Close()
		return err
	}
	return store.Close()
}

func fail(message string) {
	fmt.Fprintf(os.Stderr, "performance-probe: %s\n", message)
	os.Exit(2)
}
