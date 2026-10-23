//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// These are the channel keys used by WebView2Loader and mygo. The value named
// EBWebView is the installed runtime directory; a client/version key is not a
// runtime and can remain in the registry after the files have been removed.
var webView2Channels = []struct {
	name string
	id   string
}{
	{name: "stable", id: "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"},
	{name: "beta", id: "{2CD8A007-E189-409D-A2C8-9AF4EF3C72AA}"},
	{name: "dev", id: "{0D50BFEC-CD6A-4F9A-964C-C7416E3ACB10}"},
	{name: "canary", id: "{65C35B14-6C1D-4122-AC46-7148CC9D6497}"},
}

const (
	webView2BrowserExecutable = "msedgewebview2.exe"
	webView2ClientDirectory   = "EBWebView"
	webView2ClientDLL         = "EmbeddedBrowserWebView.dll"
)

// webView2Requirement follows the same discovery contract as mygo's native
// backend. It reads the fixed-runtime override first, then the EBWebView
// directory recorded by EdgeUpdate for each channel, hive, and registry view.
// The probe is intentionally read-only and observational. The current
// workbench uses mygo native-content windows, so a missing or stale WebView2
// runtime is recorded as optional evidence; a future page/WebView window must
// promote this observation to a required host check.
func webView2Requirement() Requirement {
	if fixed := strings.TrimSpace(os.Getenv("WEBVIEW2_BROWSER_EXECUTABLE_FOLDER")); fixed != "" {
		if detail, ok := inspectWebView2Runtime(fixed); ok {
			return requirement("webview2", "pass", false, "optional page windows; fixed runtime: "+detail)
		}
		return requirement("webview2", "fail", false, "optional page windows; WEBVIEW2_BROWSER_EXECUTABLE_FOLDER has no complete WebView2 runtime")
	}
	hives := []struct {
		name string
		root registry.Key
	}{
		{name: "HKCU", root: registry.CURRENT_USER},
		{name: "HKLM", root: registry.LOCAL_MACHINE},
	}
	var accessErrors []string
	staleRuntime := ""
	var candidates []WebView2RuntimeCandidate
	for _, channel := range webView2Channels {
		path := `SOFTWARE\Microsoft\EdgeUpdate\ClientState\` + channel.id
		for _, hive := range hives {
			// EdgeUpdate records the runtime in the 32-bit view on common
			// installations. Query both views so a fixed enterprise install in
			// the native view is also accepted.
			for _, view := range []struct {
				name string
				flag uint32
			}{{"32", registry.WOW64_32KEY}, {"64", registry.WOW64_64KEY}} {
				directory, found, err := readWebView2Directory(hive.root, path, view.flag)
				if err != nil {
					accessErrors = append(accessErrors, hive.name+"/"+view.name+"\\"+path+": "+err.Error())
					continue
				}
				if !found {
					continue
				}
				if detail, ok := inspectWebView2Runtime(directory); ok {
					candidates = append(candidates, WebView2RuntimeCandidate{
						Channel: channel.name, Hive: hive.name, View: view.name,
						Directory: directory, Version: detail, Complete: true,
					})
					continue
				}
				if staleRuntime == "" {
					staleRuntime = fmt.Sprintf("%s/%s %s", hive.name, view.name, channel.name)
				}
			}
		}
	}
	if selected, ok := ChooseWebView2Runtime(candidates); ok {
		return requirement("webview2", "pass", false, fmt.Sprintf("optional page windows; %s/%s %s runtime %s", selected.Hive, selected.View, selected.Channel, selected.Version))
	}
	if staleRuntime != "" {
		return requirement("webview2", "fail", false, "optional page windows; "+staleRuntime+" runtime is missing browser or client DLL")
	}
	if len(accessErrors) > 0 {
		return requirement("webview2", "unknown", false, "optional page windows; registry query failed: "+accessErrors[0])
	}
	return requirement("webview2", "fail", false, "optional page windows; Evergreen WebView2 Runtime registry client key was not found")
}

func readWebView2Directory(root registry.Key, path string, view uint32) (directory string, found bool, err error) {
	key, err := registry.OpenKey(root, path, registry.QUERY_VALUE|view)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	defer key.Close()
	directory, _, err = key.GetStringValue(webView2ClientDirectory)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", false, nil
	}
	return directory, true, nil
}

func inspectWebView2Runtime(directory string) (string, bool) {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(directory, webView2BrowserExecutable)); err != nil {
		return "", false
	}
	architecture := "x64"
	switch runtime.GOARCH {
	case "arm64":
		architecture = "arm64"
	case "386":
		architecture = "x86"
	}
	client := filepath.Join(directory, webView2ClientDirectory, architecture, webView2ClientDLL)
	if _, err := os.Stat(client); err != nil {
		return "", false
	}
	version := filepath.Base(filepath.Clean(directory))
	if version == "." || version == string(filepath.Separator) || version == "" {
		version = "runtime"
	}
	return version, true
}
