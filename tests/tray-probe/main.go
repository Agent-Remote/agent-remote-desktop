// tray-probe exercises the actual mygo notification-area/menu-bar backend.
// It only creates a temporary icon and menu, then destroys them before exit;
// it never touches the user's persisted application state.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
)

type report struct {
	SchemaVersion    int    `json:"schema_version"`
	Probe            string `json:"probe"`
	GOOS             string `json:"goos"`
	GOARCH           string `json:"goarch"`
	HostRuntime      bool   `json:"host_runtime"`
	CrossCompilation bool   `json:"cross_compilation"`
	Status           string `json:"status"`
	Detail           string `json:"detail,omitempty"`
}

func iconPNG() ([]byte, error) {
	icon := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := 4; y < 28; y++ {
		for x := 4; x < 28; x++ {
			if x < 8 || y < 8 || x > 23 || y > 23 {
				icon.Set(x, y, color.RGBA{R: 120, G: 160, B: 200, A: 255})
			}
		}
	}
	var data bytes.Buffer
	if err := png.Encode(&data, icon); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

func main() {
	r := report{
		SchemaVersion: 1, Probe: "agent-remote-desktop.tray",
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		HostRuntime: true, CrossCompilation: false, Status: "failed",
	}
	mygo.App.SetName("Agent Remote Tray Probe")
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.WhenReady(func() {
		icon, err := iconPNG()
		if err == nil {
			tray, trayErr := mygo.NewTray(mygo.TrayOptions{
				ID:   "agent-remote-desktop-native-probe",
				Icon: icon, ToolTip: "Agent Remote Desktop tray probe",
				Menu: mygo.NewMenu([]*mygo.MenuItem{{Label: "Probe"}}),
			})
			err = trayErr
			if err == nil {
				tray.SetToolTip("Agent Remote Desktop tray probe")
				tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "Probe"}}))
				tray.Destroy()
				r.Status = "passed"
			}
		}
		if err != nil {
			r.Detail = boundedError(err)
		}
		_ = json.NewEncoder(os.Stdout).Encode(r)
		mygo.App.Quit()
	})
	if err := mygo.App.Run(); err != nil {
		r.Detail = boundedError(err)
		_ = json.NewEncoder(os.Stdout).Encode(r)
		os.Exit(2)
	}
	if r.Status != "passed" {
		os.Exit(2)
	}
}

func boundedError(err error) string {
	if err == nil {
		return ""
	}
	value := strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '\x00' {
			return ' '
		}
		return r
	}, fmt.Sprintf("%v", err))
	if len(value) > 240 {
		value = value[:240]
	}
	return value
}
