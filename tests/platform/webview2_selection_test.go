package platform_test

import (
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/platform"
)

func TestChooseWebView2RuntimeUsesLoaderPrecedence(t *testing.T) {
	candidates := []platform.WebView2RuntimeCandidate{
		{Channel: "canary", Directory: `C:\canary`, Version: "200.0.1.1", Complete: true},
		{Channel: "beta", Directory: `C:\beta`, Version: "199.0.1.1", Complete: true},
		{Channel: "stable", Directory: `C:\stable-old`, Version: "198.0.1.1", Complete: true},
		{Channel: "stable", Directory: `C:\stable-new`, Version: "198.0.2.1", Complete: true},
	}
	selected, ok := platform.ChooseWebView2Runtime(candidates)
	if !ok {
		t.Fatal("complete WebView2 candidate was not selected")
	}
	if selected.Channel != "stable" || selected.Directory != `C:\stable-new` {
		t.Fatalf("selected runtime = %+v, want newest stable candidate", selected)
	}
}

func TestChooseWebView2RuntimeIgnoresIncompleteAndUnknownCandidates(t *testing.T) {
	for name, candidates := range map[string][]platform.WebView2RuntimeCandidate{
		"empty":           nil,
		"incomplete":      {{Channel: "stable", Directory: `C:\stable`, Version: "1.0.0.0"}},
		"unknown channel": {{Channel: "nightly", Directory: `C:\nightly`, Version: "9.0.0.0", Complete: true}},
		"empty directory": {{Channel: "stable", Version: "9.0.0.0", Complete: true}},
	} {
		t.Run(name, func(t *testing.T) {
			if selected, ok := platform.ChooseWebView2Runtime(candidates); ok {
				t.Fatalf("selected invalid runtime %+v", selected)
			}
		})
	}
}

func TestChooseWebView2RuntimeTreatsVersionAsNumeric(t *testing.T) {
	candidates := []platform.WebView2RuntimeCandidate{
		{Channel: "stable", Directory: `C:\v9`, Version: "9.10.1.0", Complete: true},
		{Channel: "stable", Directory: `C:\v10`, Version: "10.1.0.0", Complete: true},
	}
	selected, ok := platform.ChooseWebView2Runtime(candidates)
	if !ok || selected.Directory != `C:\v10` {
		t.Fatalf("selected runtime = %+v, want numeric newest version", selected)
	}
}
