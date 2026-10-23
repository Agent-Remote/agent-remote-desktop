package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
)

func (a *App) installMenu() {
	var items []*mygo.MenuItem
	for _, command := range a.commands() {
		items = append(items, &mygo.MenuItem{Label: command.label(), Accelerator: command.accelerator, Click: func(*mygo.MenuItem, *mygo.Window) {
			if !a.modalOpen() {
				command.run()
			}
			if a.win != nil {
				a.win.Invalidate()
			}
		}})
	}
	items = append(items, mygo.Separator(), &mygo.MenuItem{Role: mygo.RoleClose}, &mygo.MenuItem{Role: mygo.RoleQuit})
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: locales.Workbench(), Submenu: items},
		{Role: mygo.RoleEditMenu}, {Role: mygo.RoleWindowMenu},
	}))
}

func (a *App) installTray() {
	icon := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 5; y < 27; y++ {
		for x := 5; x < 27; x++ {
			if x < 9 || y < 9 || y > 22 || x > 22 {
				icon.Set(x, y, color.RGBA{R: 120, G: 160, B: 200, A: 255})
			}
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, icon); err != nil {
		return
	}
	// Keep a stable identifier so macOS preserves the user's menu-bar
	// placement and Windows remembers whether the icon is in the taskbar or
	// overflow area across restarts. Linux panels ignore the identifier.
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "agent-remote-desktop",
		Icon:    data.Bytes(),
		ToolTip: locales.AppTitle(),
		Menu:    a.trayMenu(),
	})
	if err == nil {
		a.tray = tray
		return
	}
	// A missing Linux AppIndicator implementation (or a native tray API
	// failure on another host) must remain visible to the user. The native
	// acceptance probe treats this as a failed runtime dependency; the running
	// app still reports the bounded error instead of silently losing its
	// background-reopen affordance.
	a.message = presentation.Notice(err)
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *App) trayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: locales.OpenWorkbench(), Click: func(*mygo.MenuItem, *mygo.Window) { a.openWindow() }},
		mygo.Separator(), {Role: mygo.RoleQuit},
	})
}
