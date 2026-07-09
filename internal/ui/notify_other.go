//go:build !windows

package ui

// registerNotificationDisplayName is Windows-only: it maps the app's
// AppUserModelID to a friendly toast-notification name. Other platforms take
// the name from the desktop entry / app bundle, so nothing to do here.
func registerNotificationDisplayName() {}
