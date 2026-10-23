package platform

import (
	"path/filepath"
	"strconv"
	"strings"
)

// WebView2RuntimeCandidate is one complete WebView2 runtime observed in an
// EdgeUpdate registry view. It is kept independent from the Windows registry
// package so the selection policy can be checked on every build host.
//
// Channel is ordered by the loader's preference: stable, beta, dev, canary.
// Version is the directory basename (for example 128.0.2739.42). A candidate
// is considered complete only after its browser and architecture-specific
// client DLL have been inspected by the Windows probe.
type WebView2RuntimeCandidate struct {
	Channel   string
	Hive      string
	View      string
	Directory string
	Version   string
	Complete  bool
}

// ChooseWebView2Runtime applies the same precedence as WebView2Loader.dll:
// stable wins over beta/dev/canary, and the newest complete installation wins
// within a channel. Registry enumeration order must therefore never affect
// which runtime the desktop starts.
func ChooseWebView2Runtime(candidates []WebView2RuntimeCandidate) (WebView2RuntimeCandidate, bool) {
	var selected WebView2RuntimeCandidate
	selectedRank := -1
	for _, candidate := range candidates {
		if !candidate.Complete || strings.TrimSpace(candidate.Directory) == "" {
			continue
		}
		rank := webView2ChannelRank(candidate.Channel)
		if rank < 0 {
			continue
		}
		if selectedRank < 0 || rank < selectedRank ||
			(rank == selectedRank && compareWebView2Versions(candidate.Version, selected.Version) > 0) {
			selected = candidate
			selectedRank = rank
		}
	}
	if selectedRank < 0 {
		return WebView2RuntimeCandidate{}, false
	}
	return selected, true
}

func webView2ChannelRank(channel string) int {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "stable":
		return 0
	case "beta":
		return 1
	case "dev":
		return 2
	case "canary":
		return 3
	default:
		return -1
	}
}

func compareWebView2Versions(a, b string) int {
	left := parseWebView2Version(a)
	right := parseWebView2Version(b)
	for i := 0; i < len(left) || i < len(right); i++ {
		var x, y int
		if i < len(left) {
			x = left[i]
		}
		if i < len(right) {
			y = right[i]
		}
		if x < y {
			return -1
		}
		if x > y {
			return 1
		}
	}
	return 0
}

func parseWebView2Version(version string) []int {
	if version == "" {
		return nil
	}
	if version == "." || version == string(filepath.Separator) {
		return nil
	}
	parts := strings.Split(version, ".")
	result := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nil
		}
		result = append(result, value)
	}
	return result
}
