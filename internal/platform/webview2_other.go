//go:build !windows

package platform

// webView2Requirement keeps the runtime probe portable. WebView2 is a
// Windows-only dependency; the Linux and macOS backends report their own
// webview requirements in ProbeRuntime.
func webView2Requirement() Requirement {
	return requirement("webview2", "unknown", false, "not applicable on this host")
}
