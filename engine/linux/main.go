//go:build linux

package main

import "tomoru-engine-shared"

func main() {
	shared.Run(shared.Platform{Engine: "linux", System: "Linux", App: "<app>",
		Source: systemSource, Speak: speak, Select: selectArea, ScreenAccess: screenAccess})
}
