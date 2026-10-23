package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
	"slices"
	"time"
)

type draggedTab struct{ ID, ProjectID string }

func (a *App) tabbar(c *ui.Context, projectID string) {
	ui.ScrollHorizontal(c).Height(44).Padding(4, 12).Gap(6).Children(func() {
		// A drop may change the model while this frame is being built.
		for _, tab := range slices.Clone(a.state.Board.Tabs) {
			if tab.ProjectID != projectID {
				continue
			}
			row := ui.Row(c.Key(tab.ID)).Gap(2).Radius(8).Drag(draggedTab{tab.ID, projectID})
			if value, over := ui.DragOver[draggedTab](row); over && value.ProjectID == projectID && value.ID != tab.ID {
				row.Border(1, c.Theme().Accent)
			}
			if value, dropped := ui.Drop[draggedTab](row); dropped && value.ProjectID == projectID && a.state.Board.Move(value.ID, tab.ID) {
				a.focusValue = a.active
				a.persist()
				c.After(time.Millisecond)
			}
			row.Children(func() {
				label := tab.Title
				if count := len(tab.Root.Leaves()); count > 1 {
					label = locales.TabTerminals(label, count)
				}
				button := ui.Button(c, label).Label(locales.SelectTab(tab.Title)).Padding(8, 14).Tooltip(locales.DragTabHelp())
				if a.state.Board.Current(projectID) == tab {
					button.Background(a.tokens().Selected()).TextColor(a.tokens().AccentInk)
				}
				button.OnClick(func() { a.selectTab(tab.ID) })
				ui.Button(c, "×").Label(locales.HideTab(tab.Title)).Tooltip(locales.HideTabHelp()).OnClick(func() {
					for _, id := range tab.Root.Leaves() {
						a.state.Board.Hide(id)
					}
					a.restoreFocus()
					a.persist()
				})
			})
		}
	})
}
