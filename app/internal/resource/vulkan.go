package resource

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"strings"
)

var driverLayers = []string{"VK_LAYER_AMD_", "VK_LAYER_INTEL_", "VK_LAYER_NV_"}

type layerManifest struct {
	Layer  *implicitLayer  `json:"layer"`
	Layers []implicitLayer `json:"layers"`
}

type implicitLayer struct {
	Name    string            `json:"name"`
	Disable map[string]string `json:"disable_environment"`
}

func DisableVulkanLayers(cmd *exec.Cmd) {
	switches := layerSwitches(implicitLayerManifests())
	if len(switches) == 0 {
		return
	}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, switches...)
}

func layerSwitches(manifests []string) []string {
	var env []string
	for _, path := range manifests {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var m layerManifest
		if json.Unmarshal(bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), &m) != nil {
			continue
		}
		layers := m.Layers
		if m.Layer != nil {
			layers = append(layers, *m.Layer)
		}
		for _, l := range layers {
			if slices.ContainsFunc(driverLayers, func(prefix string) bool { return strings.HasPrefix(l.Name, prefix) }) {
				continue
			}
			for name, value := range l.Disable {
				if name == "" || strings.ContainsAny(name, "=\x00") || strings.ContainsRune(value, 0) {
					continue
				}
				if value == "" {
					value = "1"
				}
				env = append(env, name+"="+value)
			}
		}
	}
	slices.Sort(env)
	return slices.Compact(env)
}
