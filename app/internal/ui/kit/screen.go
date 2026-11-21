package kit

import (
	"context"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/permissionflow"
)

const screenSettings = "x-apple.systempreferences:com.apple.preference.security?Privacy_ScreenCapture"

func OpenScreenSettings() { mygo.Shell.OpenExternal(screenSettings) }

func GuideScreenRecording() {
	go func() {
		if _, err := permissionflow.Guide(context.Background(), permissionflow.ScreenRecording, permissionflow.GuideOptions{}); err != nil {
			log.Println("screen recording:", err)
			OpenScreenSettings()
		}
	}()
}
