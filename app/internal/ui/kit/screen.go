package kit

import "github.com/egoist/mygo"

// screenSettings is System Settings' pane of Screen & System Audio Recording, which lists System
// Audio Recording Only under it.
const screenSettings = "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"

// OpenScreenSettings opens System Settings where the app's recording permissions are turned on.
func OpenScreenSettings() { mygo.Shell.OpenExternal(screenSettings) }
