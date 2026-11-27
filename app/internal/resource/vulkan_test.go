package resource

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLayerSwitches(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	manifests := []string{
		write("obs-vulkan64.json", `{"file_format_version": "1.0.0", "layer": {"name": "VK_LAYER_OBS_HOOK", "disable_environment": {"DISABLE_VULKAN_OBS_CAPTURE": "1"}}}`),
		write("overlay-vk-layer.json", "\xef\xbb\xbf"+`{"file_format_version": "1.1.2", "layer": {"name": "VK_LAYER_HEYBOX_overlay", "disable_environment": {"HEYBOX_OVERLAY_VK_DISABLE": "1"}}}`),
		write("several.json", `{"file_format_version": "1.0.1", "layers": [
			{"name": "VK_LAYER_GAMEPP", "disable_environment": {"DISABLE_GAMEPP_LAYER": ""}},
			{"name": "VK_LAYER_RENDERDOC_Capture", "disable_environment": {"DISABLE_MIRILLIS_LAYER": "1"}}]}`),
		write("amd.json", `{"file_format_version": "1.0.0", "layer": {"name": "VK_LAYER_AMD_switchable_graphics_64", "disable_environment": {"DISABLE_LAYER_AMD_SWITCHABLE_GRAPHICS_1": "1"}}}`),
		write("nv.json", `{"file_format_version": "1.0.1", "layers": [{"name": "VK_LAYER_NV_optimus", "disable_environment": {"DISABLE_LAYER_NV_OPTIMUS_1": ""}}]}`),
		write("again.json", `{"file_format_version": "1.0.0", "layer": {"name": "VK_LAYER_OBS_HOOK", "disable_environment": {"DISABLE_VULKAN_OBS_CAPTURE": "1"}}}`),
		write("broken.json", `{"layer": `),
		write("odd.json", `{"file_format_version": "1.0.0", "layer": {"name": "VK_LAYER_ODD", "disable_environment": {"": "1", "A=B": "1", "NUL\u0000": "1", "VALUE_NUL": "1\u0000"}}}`),
		filepath.Join(dir, "uninstalled.json"),
	}
	want := []string{
		"DISABLE_GAMEPP_LAYER=1",
		"DISABLE_MIRILLIS_LAYER=1",
		"DISABLE_VULKAN_OBS_CAPTURE=1",
		"HEYBOX_OVERLAY_VK_DISABLE=1",
	}
	if got := layerSwitches(manifests); !slices.Equal(got, want) {
		t.Errorf("switches %q, want %q", got, want)
	}
	if got := layerSwitches(nil); got != nil {
		t.Errorf("no manifests gave %q", got)
	}
}
