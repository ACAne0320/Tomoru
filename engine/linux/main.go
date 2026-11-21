//go:build linux

package main

import "tomoru-engine-shared"

func main() {
	shared.Run(shared.Platform{Engine: "linux", System: "Linux", App: "<app>",
		Source: systemSource, Apps: func() {}, Speak: speak, Select: selectArea, ScreenAccess: screenAccess})
}

func speak(locale string, _ []string) { shared.Fail("no-voice", "no voice for %s on Linux", locale) }
