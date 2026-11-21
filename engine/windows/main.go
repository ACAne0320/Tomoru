//go:build windows

package main

import "tomoru-engine-shared"

func main() {
	shared.Run(shared.Platform{Engine: "windows", System: "Windows", App: "<exe>",
		Source: systemSource, HearsOneApp: true, Apps: listApps, Speak: speak, Select: selectArea})
}
