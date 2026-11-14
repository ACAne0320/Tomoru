package kit

import (
	"context"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/permissionflow"
)

// screenSettings is System Settings' pane of Screen & System Audio Recording, which lists System
// Audio Recording Only under it.
const screenSettings = "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"

// OpenScreenSettings opens System Settings where the app's recording permissions are turned on.
func OpenScreenSettings() { mygo.Shell.OpenExternal(screenSettings) }

// GuideScreenRecording opens System Settings where Screen Recording is turned on, which selecting
// a part of the screen needs, with a panel under its window that holds the app to drag into the
// list: turned off there once, it is turned on so. It returns at once; the panel goes when the
// permission is on, or as the user closes it or System Settings. Where it cannot guide, it opens
// the page alone.
func GuideScreenRecording() {
	go func() {
		if _, err := permissionflow.Guide(context.Background(), permissionflow.ScreenRecording, permissionflow.GuideOptions{}); err != nil {
			log.Println("screen recording:", err)
			OpenScreenSettings() // from any goroutine
		}
	}()
}
