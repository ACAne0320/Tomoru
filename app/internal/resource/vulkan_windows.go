package resource

import "golang.org/x/sys/windows/registry"

func implicitLayerManifests() []string {
	var paths []string
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		k, err := registry.OpenKey(root, `SOFTWARE\Khronos\Vulkan\ImplicitLayers`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, _ := k.ReadValueNames(0)
		for _, name := range names {
			if skip, _, err := k.GetIntegerValue(name); err == nil && skip == 0 {
				paths = append(paths, name)
			}
		}
		k.Close()
	}
	return paths
}
