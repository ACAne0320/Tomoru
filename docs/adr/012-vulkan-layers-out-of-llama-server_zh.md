# ADR 012：其他软件的 Vulkan 层不进 llama-server

[English](012-vulkan-layers-out-of-llama-server.md)

日期：2026-11-27。状态：已采纳。

## 背景

Windows 上本地模型由随附的 llama-server（`b11960`，Vulkan）运行。录屏、游戏覆盖层和游戏工具会在
`SOFTWARE\Khronos\Vulkan\ImplicitLayers` 下注册 Vulkan 隐式层，Vulkan 加载器把它们放进每个用到
Vulkan 的进程。[#172](https://github.com/ACAne0320/Tomoru/issues/172) 中，一台装了小黑盒、Cheat
Happens、游戏加加、OBS 和 Mirillis Action! 的 RTX 2060 SUPER 上，所有 llama-server，识别和翻译都一样，
都在 `vkCreateDevice` 里崩溃（`0xC0000005`），此时加载器已经把这五个层放在驱动前面。列出显卡正常、
纯 CPU 正常，一台没装这些工具的 GTX 1080 Ti 运行同样的文件也正常。设上各层自己的禁用变量后，模型就能运行。

应用对识别模型也显示「本地翻译模型没能运行起来」，既没记下 llama-server 的退出，也没记下会话为什么
停了，所以最初的日志什么也看不出。

`VK_LOADER_LAYERS_DISABLE=~implicit~` 能关掉所有隐式层，但要求加载器基于 1.3.234 及以上的头文件构建，
在那台机器上它没有效果。

## 决定

1. `resource.DisableVulkanLayers`（`app/internal/resource/vulkan.go`）读取 `HKEY_LOCAL_MACHINE` 和
   `HKEY_CURRENT_USER` 中已启用隐式层的清单（`vulkan_windows.go`），为子进程设上每个层自己的
   `disable_environment` 变量。所有版本的加载器都认这些变量。名为 `VK_LAYER_AMD_`、`VK_LAYER_INTEL_` 和
   `VK_LAYER_NV_` 的是显卡驱动自己的层，保留；NVIDIA 现在的驱动本来也把它们注册在别处。
2. `models.Server` 和 `models.Devices` 启动每个 llama-server 时都调用它。其他子进程不用 Vulkan，不变。
3. 其他平台不变：Linux 的隐式层里还有 Mesa 的设备选择层，它决定用哪块显卡，那里也没有崩溃的报告。
4. 识别模型启动失败是 `core.LocalASRError`，显示自己的提示（`errAsrLocal`）。llama-server 自己退出时把
   退出状态写进 `llama-server.log`，会话启动不了时把原因写进 `tomoru.log`。

## 后果

- 这些软件的覆盖层、录屏和采集工具不再看到 llama-server，它本来也不画任何东西。
- 加载器会加载的隐式层都有禁用变量：没有登记的，它本来就跳过。
- 显卡厂商用别的名字注册的层，在 llama-server 里也会被关掉。
