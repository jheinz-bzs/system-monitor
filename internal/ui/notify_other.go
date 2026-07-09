//go:build !windows

package ui

// registerNotificationAppName is Windows-only: it creates the Start Menu
// shortcut that maps the app's AppUserModelID to a friendly toast-notification
// name. Other platforms take the name from the desktop entry / app bundle, so
// nothing to do here.
func registerNotificationAppName() {}
