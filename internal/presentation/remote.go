package presentation

import "github.com/Agent-Remote/agent-remote-desktop/locales"

func RemoteStatus(status string) string {
	switch status {
	case "running", "active":
		return locales.Running()
	case "ready":
		return locales.RemoteStatusReady()
	case "pending", "creating", "starting", "saving", "accepted":
		return locales.RemoteStatusPending()
	case "stopped":
		return locales.RemoteStatusStopped()
	case "stopping":
		return locales.RemoteStatusStopping()
	case "interrupted":
		return locales.RemoteStatusInterrupted()
	case "failed":
		return locales.RemoteStatusFailed()
	case "disabled":
		return locales.RemoteStatusDisabled()
	default:
		return locales.RemoteStatusOther(status)
	}
}
