//go:build windows

package ui

import (
	"golang.org/x/sys/windows/registry"

	"fyne.io/fyne/v2"
)

// aumidKeyPath is where Windows looks up a friendly name for an unpackaged
// app's AppUserModelID. Fyne passes appID as the AUMID on toast notifications;
// without this registration Windows displays the raw ID instead of appName.
const aumidKeyPath = `SOFTWARE\Classes\AppUserModelId\` + appID

const aumidDisplayNameValue = "DisplayName"

// registerNotificationDisplayName maps appID → appName in HKCU so toast
// notifications are attributed to "System Monitor". Idempotent, per-user (no
// elevation needed); on failure notifications still work, just show the raw ID.
func registerNotificationDisplayName() {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, aumidKeyPath, registry.SET_VALUE)
	if err != nil {
		fyne.LogError("create notification AppUserModelId key", err)
		return
	}
	defer key.Close()
	if err := key.SetStringValue(aumidDisplayNameValue, appName); err != nil {
		fyne.LogError("set notification display name", err)
	}
}
