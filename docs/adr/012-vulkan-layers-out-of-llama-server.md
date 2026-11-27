# ADR 012: Other apps' Vulkan layers stay out of llama-server

[中文](012-vulkan-layers-out-of-llama-server_zh.md)

Date: 2026-11-27. Status: accepted.

## Context

On Windows the bundled llama-server (`b11960`, Vulkan) runs the on-device models. Recorders,
overlays and game tools register Vulkan implicit layers under
`SOFTWARE\Khronos\Vulkan\ImplicitLayers`, and the Vulkan loader puts them into every process that
uses Vulkan. In [#172](https://github.com/ACAne0320/Tomoru/issues/172), an RTX 2060 SUPER with
Heybox, Cheat Happens, GamePP, OBS and Mirillis Action! installed, every llama-server, recognition
and translation alike, crashed (`0xC0000005`) inside `vkCreateDevice`, after the loader had put
those five layers in front of the driver. Listing the devices worked, the CPU worked, and a GTX
1080 Ti without these tools ran the same files. With each layer's own disable variable set, the
models ran.

The app showed the translation model's message for the recognition model too (in Chinese,
「本地翻译模型没能运行起来」), and logged neither llama-server's exit nor why the session stopped,
so the first logs told nothing.

`VK_LOADER_LAYERS_DISABLE=~implicit~` turns all implicit layers off, but needs a loader built with
the 1.3.234 headers or later, and on that machine it showed no effect.

## Decision

1. `resource.DisableVulkanLayers` (`app/internal/resource/vulkan.go`) reads the manifests of the
   enabled implicit layers in `HKEY_LOCAL_MACHINE` and `HKEY_CURRENT_USER`
   (`vulkan_windows.go`) and sets each layer's own `disable_environment` variable for the child.
   Every loader honors these. Layers named `VK_LAYER_AMD_`, `VK_LAYER_INTEL_` and `VK_LAYER_NV_`
   are the GPU drivers' own and stay; NVIDIA's current drivers register theirs elsewhere anyway.
2. `models.Server` and `models.Devices` call it for every llama-server they start. Other child
   processes don't use Vulkan and are left as they are.
3. Other platforms are unchanged: on Linux the implicit layers also include Mesa's device
   selection, which picks the GPU, and no crash there has been reported.
4. A recognition model that fails to start is `core.LocalASRError`, shown as its own message
   (`errAsrLocal`). llama-server exiting by itself writes its exit status to `llama-server.log`,
   and a session that can't start writes why to `tomoru.log`.

## Consequences

- The overlays, recorders and capture tools of those apps no longer see llama-server, which draws
  nothing.
- Every implicit layer the loader loads has a disable variable: it skips those that name none.
- A layer a GPU vendor registers under another name is turned off for llama-server too.
