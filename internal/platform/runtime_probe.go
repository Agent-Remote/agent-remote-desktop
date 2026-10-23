package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/xpty"
)

// Requirement is one observation made by the read-only platform probe. A
// status of unknown means that inspection was inconclusive; required checks
// with that status fail support rather than being treated as a pass.
type Requirement struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Required bool   `json:"required"`
	Detail   string `json:"detail,omitempty"`
}

// RuntimeReport describes the host on which the probe is running. It never
// claims that a cross-compiled binary has been executed: HostRuntime is true
// and CrossCompilation is false only for this process's current host.
type RuntimeReport struct {
	SchemaVersion    int           `json:"schema_version"`
	Probe            string        `json:"probe"`
	GOOS             string        `json:"goos"`
	GOARCH           string        `json:"goarch"`
	Platform         string        `json:"platform"`
	HostRuntime      bool          `json:"host_runtime"`
	CrossCompilation bool          `json:"cross_compilation"`
	Display          string        `json:"display"`
	DesktopSession   string        `json:"desktop_session,omitempty"`
	Supported        bool          `json:"supported"`
	Requirements     []Requirement `json:"requirements"`
}

// ProbeRuntime gathers only local, read-only runtime observations. The native
// workbench requires GTK/AppIndicator on Linux and native PTY support on all
// release hosts; the supported target, shell and display (where applicable)
// are also required for acceptance. WebKitGTK is observed for future page
// windows but is optional while the workbench uses native-content windows.
func ProbeRuntime() RuntimeReport {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	report := RuntimeReport{
		SchemaVersion:    1,
		Probe:            "agent-remote-desktop.platform",
		GOOS:             goos,
		GOARCH:           goarch,
		Platform:         platformName(goos),
		HostRuntime:      true,
		CrossCompilation: false,
		Display:          displayState(goos),
		DesktopSession:   desktopSession(),
	}
	report.Requirements = append(report.Requirements,
		requirement("supported-target", supportedTarget(goos, goarch), true,
			goos+"/"+goarch),
		requirement("shell", shellRequirement(goos), true, shellDetail(goos)),
		terminalRequirement(goos),
	)
	if goos == "linux" {
		report.Requirements = append(report.Requirements,
			linuxDisplayRequirement(),
			pkgConfigRequirement("gtk3", "gtk+-3.0"),
			linuxWebKitRequirement(),
			linuxTrayRequirement(),
		)
	} else {
		report.Requirements = append(report.Requirements,
			requirement("display", report.Display, false, "native-window probe verifies an interactive desktop session"),
		)
	}
	if goos == "darwin" {
		report.Requirements = append(report.Requirements, macOSVersionRequirement())
	} else if goos == "windows" {
		report.Requirements = append(report.Requirements, webView2Requirement())
	}
	report.Supported = true
	for _, item := range report.Requirements {
		if item.Required && item.Status != "pass" {
			report.Supported = false
			break
		}
	}
	return report
}

func linuxWebKitRequirement() Requirement {
	// The desktop currently creates only mygo native-content windows. WebKitGTK
	// is loaded lazily by mygo and is needed only if a future page/WebView
	// window is added, so report its availability without making Linux support
	// fail closed when the optional library is absent.
	if library := runtimeLibrary("webkitgtk"); library != "" {
		return requirement("webkitgtk", "pass", false, "optional page windows: "+library)
	}
	if _, err := exec.LookPath("pkg-config"); err == nil {
		for _, packageName := range []string{"webkit2gtk-4.1", "webkit2gtk-4.0"} {
			if err := exec.Command("pkg-config", "--exists", packageName).Run(); err == nil {
				return requirement("webkitgtk", "unknown", false, "optional page windows; pkg-config metadata only: "+pkgConfigDetail(packageName))
			}
		}
	}
	return requirement("webkitgtk", "fail", false, "optional page windows: WebKitGTK 4.1 or 4.0 runtime library was not found")
}

// terminalRequirement allocates the same native PTY implementation used by
// managed Agent sessions. It does not start a command or inspect user state;
// the allocation/size/close round trip catches missing Unix PTY or Windows
// ConPTY support before the UI probes are run.
func terminalRequirement(goos string) Requirement {
	if goos != "darwin" && goos != "windows" && goos != "linux" {
		return requirement("terminal", "unknown", false, "not applicable on unsupported host target")
	}
	backend := "unix-pty"
	if goos == "windows" {
		backend = "conpty"
	}
	pty, err := xpty.NewPty(100, 30)
	if err != nil {
		return requirement("terminal", "fail", true, backend+": allocation failed: "+probeError(err))
	}
	cols, rows, sizeErr := pty.Size()
	closeErr := pty.Close()
	if sizeErr != nil {
		return requirement("terminal", "fail", true, fmt.Sprintf("%s: size failed: %s", backend, probeError(sizeErr)))
	}
	if cols < 1 || rows < 1 {
		return requirement("terminal", "fail", true, fmt.Sprintf("%s: invalid size %dx%d", backend, cols, rows))
	}
	if closeErr != nil {
		return requirement("terminal", "fail", true, fmt.Sprintf("%s: close failed: %s", backend, probeError(closeErr)))
	}
	return requirement("terminal", "pass", true, fmt.Sprintf("%s %dx%d", backend, cols, rows))
}

func probeError(err error) string {
	value := strings.TrimSpace(err.Error())
	value = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\x00' {
			return ' '
		}
		return r
	}, value)
	if len(value) > 160 {
		value = value[:160]
	}
	return value
}

func requirement(id, status string, required bool, detail string) Requirement {
	if status == "" {
		status = "unknown"
	}
	return Requirement{ID: id, Status: status, Required: required, Detail: detail}
}

func supportedTarget(goos, goarch string) string {
	if (goos == "darwin" || goos == "windows" || goos == "linux") &&
		(goarch == "amd64" || goarch == "arm64") {
		return "pass"
	}
	return "fail"
}

func platformName(goos string) string {
	if goos == "darwin" {
		return "macos"
	}
	return goos
}

func displayState(goos string) string {
	if goos == "linux" {
		return displayAvailability()
	}
	// macOS and Windows use native windowing APIs and do not require DISPLAY,
	// but process startup alone cannot prove that an interactive desktop session
	// is available. The native-window probe supplies that evidence separately.
	if goos == "darwin" || goos == "windows" {
		return "unknown"
	}
	return "unavailable"
}

func displayAvailability() string {
	if strings.TrimSpace(os.Getenv("DISPLAY")) != "" || strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" {
		return "available"
	}
	return "unavailable"
}

func desktopSession() string {
	for _, key := range []string{"XDG_CURRENT_DESKTOP", "DESKTOP_SESSION", "WAYLAND_DISPLAY", "DISPLAY"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			// Keep evidence bounded and free of accidental multi-line values.
			value = strings.Map(func(r rune) rune {
				if r == '\n' || r == '\r' || r == '\x00' {
					return -1
				}
				return r
			}, value)
			if len(value) > 128 {
				value = value[:128]
			}
			return value
		}
	}
	return "unavailable"
}

func shellRequirement(goos string) string {
	if goos == "windows" {
		command := strings.TrimSpace(os.Getenv("ComSpec"))
		if command == "" {
			command = "cmd.exe"
		}
		if _, err := exec.LookPath(command); err == nil {
			return "pass"
		}
		return "fail"
	}
	info, err := os.Stat("/bin/sh")
	if err == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
		return "pass"
	}
	return "fail"
}

func shellDetail(goos string) string {
	if goos == "windows" {
		if command := strings.TrimSpace(os.Getenv("ComSpec")); command != "" {
			return filepath.Base(command)
		}
		return "cmd.exe"
	}
	return "/bin/sh"
}

func linuxDisplayRequirement() Requirement {
	return requirement("display", passFail(displayAvailability() == "available"), true, displayState("linux"))
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func pkgConfigRequirement(id, packageName string) Requirement {
	if _, err := exec.LookPath("pkg-config"); err == nil {
		if err := exec.Command("pkg-config", "--exists", packageName).Run(); err == nil {
			return requirement(id, "pass", true, pkgConfigDetail(packageName))
		}
	}
	if library := runtimeLibrary(id); library != "" {
		return requirement(id, "pass", true, library)
	}
	return requirement(id, "fail", true, packageName+" runtime library was not found")
}

func linuxTrayRequirement() Requirement {
	// mygo's Linux backend loads libayatana-appindicator3. Older desktop
	// distributions expose the same ABI under libappindicator3. Accept either
	// pkg-config name or runtime library, then let the native tray probe verify
	// the actual loader and panel integration on the target session.
	if library := runtimeLibrary("tray"); library != "" {
		return requirement("tray", "pass", true, library)
	}
	if _, err := exec.LookPath("pkg-config"); err != nil {
		return requirement("tray", "fail", true, "pkg-config and AppIndicator runtime library are unavailable")
	}
	for _, packageName := range []string{"ayatana-appindicator3-0.1", "appindicator3-0.1", "libappindicator-gtk3"} {
		if err := exec.Command("pkg-config", "--exists", packageName).Run(); err == nil {
			return requirement("tray", "pass", true, pkgConfigDetail(packageName))
		}
	}
	return requirement("tray", "fail", true, "libayatana-appindicator3 (or compatible AppIndicator ABI) was not found")
}

// pkgConfigDetail keeps the host evidence useful when a distribution carries
// more than one ABI generation. The version is informational; the successful
// --exists check remains the capability gate and no package manager is run.
func pkgConfigDetail(packageName string) string {
	version := ""
	if output, err := exec.Command("pkg-config", "--modversion", packageName).Output(); err == nil {
		version = strings.TrimSpace(string(output))
		version = strings.Map(func(r rune) rune {
			if r == '\n' || r == '\r' || r == '\x00' {
				return ' '
			}
			return r
		}, version)
		if len(version) > 96 {
			version = version[:96]
		}
	}
	if version == "" {
		return packageName + " (pkg-config; version unavailable)"
	}
	return packageName + " " + version + " (pkg-config)"
}

// runtimeLibrary checks the shared objects used by mygo's Linux backend. A
// runtime package commonly omits .pc files, so pkg-config alone would reject
// valid user machines that have the libraries but no development headers.
func runtimeLibrary(id string) string {
	candidates := map[string][]string{
		"gtk3":      {"libgtk-3.so.0"},
		"webkitgtk": {"libwebkit2gtk-4.1.so.0", "libwebkit2gtk-4.0.so.37", "libwebkit2gtk-4.0.so.0"},
		"tray":      {"libayatana-appindicator3.so.1", "libappindicator3.so.1"},
	}[id]
	if len(candidates) == 0 {
		return ""
	}
	if ldconfig, err := exec.LookPath("ldconfig"); err == nil {
		if output, err := exec.Command(ldconfig, "-p").Output(); err == nil {
			text := string(output)
			for _, candidate := range candidates {
				if strings.Contains(text, candidate) {
					return candidate + " (ldconfig)"
				}
			}
		}
	}
	for _, candidate := range candidates {
		for _, pattern := range []string{"/lib*/" + candidate, "/lib/*/" + candidate, "/usr/lib*/" + candidate, "/usr/lib/*/" + candidate} {
			if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
				return matches[0]
			}
		}
	}
	return ""
}

func macOSVersionRequirement() Requirement {
	command := exec.Command("sw_vers", "-productVersion")
	output, err := command.Output()
	if err != nil {
		return requirement("macos-version", "unknown", true, "sw_vers unavailable")
	}
	version := strings.TrimSpace(string(output))
	majorText, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(majorText)
	if err != nil {
		return requirement("macos-version", "unknown", true, "unparseable macOS version")
	}
	status := "fail"
	if major >= 13 {
		status = "pass"
	}
	return requirement("macos-version", status, true, version)
}
