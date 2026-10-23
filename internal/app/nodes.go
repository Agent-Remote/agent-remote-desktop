package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type nodePage struct {
	snapshot              remote.NodeOverview
	loaded, loading, busy bool
	filter                string
	selected              *cliadapter.NodeRecord
	review                bool
	enableEgoBrowser      bool
	argv                  []string
	err                   error
	cancel                context.CancelFunc
	// done closes after the current node query worker has published its
	// result. App.Close uses it as a lifecycle barrier so the worker cannot
	// outlive the store or application shutdown after cancellation.
	done    chan struct{}
	results chan nodeResult
}

type nodeResult struct {
	snapshot remote.NodeOverview
	err      error
}

func (a *App) openNodes() {
	a.page = "nodes"
	if !a.nodes.loading && !a.nodes.busy {
		a.loadNodes()
	}
}

func (a *App) loadNodes() {
	p := &a.nodes
	if a.closing || a.quitting.Load() || p.loading || p.busy {
		return
	}
	p.loaded, p.loading, p.err, p.selected, p.review, p.argv = false, true, nil, nil, false, nil
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results = cancel, make(chan struct{}), make(chan nodeResult, 1)
	results, done, home, find := p.results, p.done, a.home, a.findExecutable
	go func() {
		defer close(done)
		binary, err := find("agent-remote")
		var snapshot remote.NodeOverview
		if err == nil {
			snapshot, err = remote.LoadNodes(ctx, cliadapter.Client{Binary: binary, Home: home})
		}
		results <- nodeResult{snapshot: snapshot, err: err}
	}()
}

func (a *App) pollNodes(c *ui.Context) {
	p := &a.nodes
	if !p.loading && !p.busy {
		return
	}
	select {
	case result := <-p.results:
		p.loading, p.busy, p.loaded, p.err = false, false, true, result.err
		if result.err == nil {
			p.snapshot = result.snapshot
		}
		if p.cancel != nil {
			p.cancel()
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) cancelNodesWait() {
	if a.nodes.cancel != nil {
		a.nodes.cancel()
	}
}

func (a *App) selectNode(node cliadapter.NodeRecord) {
	if a.nodes.loading || a.nodes.busy || a.nodes.err != nil {
		return
	}
	copy := node
	a.nodes.selected, a.nodes.review, a.nodes.err, a.nodes.enableEgoBrowser = &copy, false, nil, false
	a.nodes.enableEgoBrowser = false
}

func (a *App) reviewNodeInstall() {
	p := &a.nodes
	if p.selected == nil || p.loading || p.busy || a.closing || a.quitting.Load() {
		return
	}
	if !remote.NodesListSupported(a.capabilities.snapshot.Capabilities) && a.capabilities.loaded {
		p.err = &cliadapter.OperationError{Code: "unsupported_method"}
		return
	}
	args := []string{"node", "install", "--node", p.selected.ID, "--yes"}
	if p.enableEgoBrowser {
		args = append(args, "--enable-ego-browser")
	}
	w, ok := cliadapter.Dedicated("agent-remote", []string{"node", "install"})
	if !ok || cliadapter.ValidateDedicatedArgv(w, args, true) != nil {
		p.err = fault.CLIProtocol
		return
	}
	p.argv, p.review, p.err = append([]string(nil), args...), true, nil
}

func (a *App) startNodeInstall() {
	p := &a.nodes
	if !p.review || len(p.argv) == 0 || p.selected == nil || a.starting || a.closing || a.quitting.Load() {
		return
	}
	project := a.project()
	if project == nil {
		project = a.ensureCLIUtilityProject()
	}
	if project == nil {
		p.err = fault.ProjectDirectory
		return
	}
	args, nodeID := append([]string(nil), p.argv...), p.selected.ID
	opID, name := workspace.NewID(), "agent-remote node install "+nodeID
	a.dedicatedOperations = append(a.dedicatedOperations, dedicatedOperation{id: opID, command: name, status: "pending", updatedAt: time.Now().UTC()})
	a.persist()
	a.page = ""
	a.nodes = nodePage{}
	a.launch(launchRequest{projectID: project.ID, directory: project.Path, tool: session.AgentRemote, args: args, name: name, operationID: opID})
}

func (a *App) nodesView(c *ui.Context) {
	p := &a.nodes
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, locales.NodesTitle()).FontSize(24).Bold().Grow(1)
			ui.Button(c, locales.ReturnWorkbench()).OnClick(func() { a.page = ""; a.nodes = nodePage{} })
		})
		ui.Text(c, locales.NodesHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
		if p.loaded {
			ui.TextInput(c, &p.filter).Label(locales.NodesFilter()).Placeholder(locales.NodesFilter())
		}
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			switch {
			case p.loading:
				ui.Text(c, locales.NodesLoading())
			case p.err != nil:
				ui.Text(c, presentation.Error(p.err)).TextColor(c.Theme().Danger)
			case p.review && p.selected != nil:
				a.nodeReviewView(c, p.selected)
			case p.loaded:
				filter := strings.ToLower(strings.TrimSpace(p.filter))
				count := 0
				for _, node := range p.snapshot.Items {
					if filter != "" && !strings.Contains(strings.ToLower(node.ID+" "+node.Name+" "+node.Status), filter) {
						continue
					}
					count++
					n := node
					ui.Column(c.Key("node-" + node.ID)).FillWidth().Padding(12).Gap(4).Background(c.Theme().Surface).Radius(8).Children(func() {
						ui.Text(c, locales.NodeSummary(n.Name, n.Status)).Bold()
						ui.Text(c, locales.NodeID(n.ID)).FontSize(12)
						if n.Version != nil {
							ui.Text(c, locales.NodeVersion(*n.Version)).FontSize(12)
						}
						if n.SSHHost != nil {
							ui.Text(c, locales.NodeSSH(nodeUser(n.SSHUser), *n.SSHHost, nodePort(n.SSHPort))).FontSize(12)
						}
						ui.Text(c, locales.NodeAdmission(boolText(n.EnrollmentAdmission), boolText(n.ExecutionAdmission))).FontSize(12).TextColor(c.Theme().TextMuted)
						ui.Button(c, locales.NodeSelect()).OnClick(func() { a.selectNode(n) })
					})
				}
				if count == 0 {
					ui.Text(c, locales.NodesEmpty()).TextColor(c.Theme().TextMuted)
				}
			}
		})
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			if p.loading {
				ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.cancelNodesWait)
			} else if p.review {
				ui.Button(c, locales.Cancel()).OnClick(func() { p.review = false; p.argv = nil })
				ui.PrimaryButton(c, locales.NodeConfirm()).OnClick(a.startNodeInstall)
			} else if p.selected != nil {
				ui.Checkbox(c, &p.enableEgoBrowser, locales.NodeEnableEgoBrowser())
				ui.Button(c, locales.Cancel()).OnClick(func() { p.selected = nil })
				ui.PrimaryButton(c, locales.NodeReview()).OnClick(a.reviewNodeInstall)
			} else {
				ui.Button(c, locales.NodesRefresh()).OnClick(a.loadNodes)
			}
		})
	})
}

func (a *App) nodeReviewView(c *ui.Context, n *cliadapter.NodeRecord) {
	p := &a.nodes
	ui.Column(c).FillWidth().Padding(14).Gap(8).Background(c.Theme().Surface).Radius(8).Children(func() {
		ui.Text(c, locales.NodeReview()).Bold()
		ui.Text(c, locales.NodeSummary(n.Name, n.Status))
		ui.Text(c, locales.NodeID(n.ID)).FontSize(12)
		ui.Text(c, locales.NodeContext(p.snapshot.Context.Home)).FontSize(12)
		if p.snapshot.Context.ServerURL != nil {
			ui.Text(c, locales.NodeServer(*p.snapshot.Context.ServerURL)).FontSize(12)
		}
		if p.snapshot.Context.DeviceID != nil {
			ui.Text(c, locales.NodeDevice(*p.snapshot.Context.DeviceID)).FontSize(12)
		}
		// The intent is part of the reviewed argv; once the review exists it is
		// frozen so a late checkbox change cannot alter the submitted request.
		ui.Checkbox(c, &p.enableEgoBrowser, locales.NodeEnableEgoBrowser()).Disabled(p.busy || p.review)
		ui.Text(c, locales.NodeInstallManaged()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.NodeArgv(strings.Join(p.argv, " "))).FontSize(12).TextColor(a.tokens().AccentInk)
	})
}

func nodePort(port *uint16) string {
	if port == nil {
		return "22"
	}
	return fmt.Sprint(*port)
}
func nodeUser(user *string) string {
	if user == nil || *user == "" {
		return "agent-remote"
	}
	return *user
}
