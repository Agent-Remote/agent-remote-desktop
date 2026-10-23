package chrome

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

// The pieces of the shared operation page (doc §4.6): context strip →
// observation → review → stages → receipt → recovery banner. Every
// management page of the Control Center is assembled from these.

// StageState is a stage of an operation as observed — never a percentage.
type StageState uint8

const (
	StageWaiting StageState = iota
	StageRunning
	StageDone
	StageUnknown // receipt lost or interrupted: query the original, never replay
	StageFailed
)

// Stage is one row of the stage list; Note is its localized state text,
// which always accompanies the mark so state is not colour alone. A Name
// that already says its state (a CLI receipt's "Server: completed") leaves
// Note empty.
type Stage struct {
	Name, Note string
	State      StageState
}

// Panel is a titled glass card of an operation page; Review is the panel
// that freezes a plan, outlined in the accent.
func (e Env) Panel(c *ui.Context, title string, review bool, content func()) ui.Element {
	el := e.Glass(ui.Column(c).FillWidth().Padding(16, 18).Gap(10), Card, 18, false)
	if review && !e.S.HighContrast {
		el = el.Border(1, e.P.Accent.Alpha(0.45))
	}
	return el.Children(func() {
		e.Text(c, title, theme.BodyStrong, e.P.TextPrimary).FontSize(12.5 * c.Theme().FontSize / 14).FontWeight(700)
		content()
	})
}

// Well is a record card inside a sheet (an account, a check, a peer): the
// sheet's glass already frosts what is behind, so a well is a hairline
// tint over it, not a second backdrop blur (apple-design §6: one material
// per depth). Lists of dozens stay cheap to draw.
func (e Env) Well(el ui.Element) ui.Element {
	el = el.Radius(14).Background(e.wellBG())
	if e.S.HighContrast {
		return el.Border(1, e.P.TextSecondary)
	}
	return el.Border(1, e.P.Hairline)
}

func (e Env) wellBG() ui.Color {
	if e.S.ReduceTransparency {
		return theme.Over(e.P.GlassCard, e.P.SolidRaised)
	}
	return e.P.GlassCard
}

// Stages lists an operation's stages with their observed state.
func (e Env) Stages(c *ui.Context, stages []Stage) ui.Element {
	return ui.Column(c).Gap(10).FillWidth().Children(func() {
		for i, s := range stages {
			ui.Row(c.Key(i)).Gap(10).FillWidth().Children(func() {
				mark := ui.Box(c).Size(14, 14).Radius(7)
				text := e.P.TextPrimary
				switch s.State {
				case StageDone:
					mark.Background(e.P.Success)
				case StageRunning:
					mark.Border(1.5, e.P.Accent)
				case StageUnknown:
					mark.Border(1.5, e.P.Warning).BorderStyle(ui.BorderDashed)
					text = e.P.Warning
				case StageFailed:
					mark.Background(e.P.Danger)
					text = e.P.Danger
				default:
					mark.Border(1.5, e.P.TextTer)
				}
				e.Text(c, s.Name, theme.Body, text).Grow(1).Shrink(1).MinWidth(0)
				if s.Note != "" {
					e.Text(c, s.Note, theme.Caption, e.P.TextTer)
				}
			})
		}
	})
}

// KV is the observation grid: a label column and single-line monospace
// values. Callers pass full IDs; truncation is visual only.
func (e Env) KV(c *ui.Context, rows [][2]string) ui.Element {
	return ui.Column(c).Gap(6).FillWidth().Children(func() {
		for i, r := range rows {
			ui.Row(c.Key(i)).Gap(14).FillWidth().Children(func() {
				e.Text(c, r[0], theme.Body, e.P.TextSecondary).Width(150).FontSize(12.5 * c.Theme().FontSize / 14)
				e.Text(c, r[1], theme.MonoSmall, e.P.TextPrimary).FontSize(12 * c.Theme().FontSize / 14).
					SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			})
		}
	})
}

// Banner is the recovery banner shown when a result is unknown; action is
// the explicit "query the original operation" button, never a retry.
func (e Env) Banner(c *ui.Context, message string, action func()) ui.Element {
	bg, border := e.P.Warning.Alpha(0.12), e.P.Warning.Alpha(0.4)
	if e.S.ReduceTransparency {
		bg = theme.Over(e.P.Warning.Alpha(0.12), e.P.Solid)
	}
	return ui.Row(c).FillWidth().Gap(12).Padding(12, 16).Radius(14).Background(bg).Border(1, border).Children(func() {
		e.Text(c, "⚠", theme.Body, e.P.Warning)
		e.Text(c, message, theme.Body, e.P.TextPrimary).Grow(1).Shrink(1).MinWidth(0)
		if action != nil {
			action()
		}
	})
}
