# ADR 008：窗口与界面

[English](008-windows-and-surfaces.md)

日期：2026-11-21。状态：采用。

## 背景

这些决定原本写在代码注释里，代码不再带注释后移到这里。范围是桌面入口（`app/main.go`、`app/osversion_*.go`）和各个界面：`ui/shell`、`ui/settings`、`ui/guide`、`ui/overlay`、`ui/subtitle`、`ui/imagecard`、`ui/panel` 和 `ui/shared`。[架构](../architecture_zh.md)、[设计语言](../design-system_zh.md)、[场景](../scenes_zh.md)和[图片翻译](../image-translation_zh.md)已经写明的不再重复。

## 决策

### 应用与 shell

1. `main`（`app/main.go`）在首次引导完成前，一启动就用普通应用的激活策略，而不是之后再切换。到了前台才切换的话，macOS 不把它放进可切回的应用里，权限提示回答后还会把前台交给之前的应用，常常是系统设置。引导在 `Asking` 或 `ScreenAsking` 结束后仍会用 `nav.Focus()` 把前台拿回来（`guide/permission.go`、`guide/picture.go`）。
2. 再次启动应用时，`main` 打开设置（`OnSecondInstance`，以及 `OnActivate`：macOS 不启动第二个实例，而是请正在运行的那个重新打开），因为菜单栏图标可能藏在刘海后面。
3. `main` 把 llama-server 的日志写进单独的文件，不混进 `tomoru.log`，因为它输出很多。
4. `osVersion`（`app/osversion_windows.go`）从 build 22000 起把系统称为 Windows 11，因为 Windows 11 报告的版本仍是 10.0。
5. 系统在登录时打开应用时（`WasOpenedAtLogin`），`Launch`（`shell/shell.go`）不管设置怎么写都不开始翻译：用户还没要字幕。
6. `openTrayMenu`（`shell/tray.go`）只在右键弹出时给托盘图标设菜单，弹完就去掉，因为带菜单的托盘图标左键也会打开菜单，而左键要打开面板。
7. `OpenPanel`（`shell/nav.go`）也能从字幕层里的按钮打开面板，字幕层还用提示显示翻译的状态（`notice`，`overlay/overlay.go`），因为用户看着字幕，菜单栏图标可能看不见：macOS 会把状态栏图标藏到刘海后面，采集时还会多出一个录制指示器占地方。
8. `ShowSettingsPage`（`shell/nav.go`）在 Linux 上用 `SetGTKCSS` 把设置窗口的标题栏按钮调成浅色。GTK 按主题颜色画它们，浅色主题里是深色，不管窗口本身是什么，而这里它们在夜色上。
9. `ShowGuide`（`shell/nav.go`）在 Windows 上隐藏系统标题栏（它是白的），引导在纸面上、窗口控制按钮下面画自己的；窗口为此加高 `TitleBar`（`guide/guide.go`）。

### 设置

10. `modelsFolder`（`settings/engines.go`）和 `export`（`settings/history.go`）在 goroutine 里调用 `mygo.Dialog.Open` 和 `mygo.Dialog.Save`，用 `mygo.RunOnMain` 把结果送回。两者都会阻塞到用户回答，而主线程负责绘制。

### 首次引导

11. `stepButton`（`guide/guide.go`）：“允许”调用 `RequestAccess`，由它开始录音，系统询问期间按钮保持禁用，因为 macOS 只在引擎第一次录音时询问“系统录音”权限。应用不弹自己的提示。
12. `permission`（`guide/permission.go`）在音频权限未授予时每 1.5 秒检查一次，因为用户可能在请求停止等待之后才回答系统提示。在系统设置里打开的权限要重新打开应用才生效，所以被拒绝时提供“重新打开”。
13. `part`（`guide/guide.go`）让一步里的各部分升起出现，不横向滑入：这一步的区域会滚动，会把从侧面进来的部分切掉。上一步立即消失，两者不会叠在一起。
14. `modelOption`（`guide/engines.go`）把模型的下载按钮做成图标，名字放在提示里：引导的列很窄，长语言里文字按钮会挤得模型名和需求换行。同一个按钮也继续已停止的下载，名字下的进度会说明。
15. `keycaps`（`guide/shortcuts.go`）按住修饰键时点亮它，读的是窗口的修饰键状态；主键只在快捷键触发后才亮：注册的全局快捷键在窗口看到之前就拿走了这个键。
16. `tryStep`（`guide/try.go`）在这一步出现时开始翻译。返回会停止；继续则保持运行，让引导结束时字幕已经开着。

### 字幕层

17. `windowOptions`（`overlay/overlay.go`）建的窗口覆盖工作区再加 `below`，字幕块按这块画布排布。在 macOS 上，`place`（`overlay/fit.go`）再把窗口收到所画内容的大小：栏的宽度，从最上面的字幕、提示或工具栏到字幕块底部，高度按 40 pt 取整，文字变化时很少需要改大小。什么都不画时不透明度为 0。截图的窗口选择（⌘⇧5）只要指针下有窗口的图层就选中它，不管透明与否，并截下整个窗口；MyGo 把窗口画在一个图层里，所以盖满屏幕的窗口每次都被选中，截下整个屏幕，而不透明度为 0 的窗口会被跳过（2026-11-23）。悬停时仍然收着，因为截图选择时指针停在字幕上也算悬停；拖动时 `moveAhead` 在排版之前先移动窗口，字幕块在窗口里的位置就不变；在这一帧之后才移动的话，窗口带着按旧位置画的一帧一起移动，字幕块会抖。字幕列的高度取画布的高度而不是窗口的：窗口顶边移动时，每行在父元素里的位置不变；取窗口高度时，它会从旧位置滑过来（位置过渡），每次悬停都闪一下。`anchorBottomLeft`（`overlay/fit_darwin.go`）把 MyGo 的 `CAMetalLayer` 的 `contentsGravity` 设为 `topLeft`，因为这个图层是翻转的，在屏幕上就是左下角：新尺寸的一帧画好之前，上一帧留在原处，不会被拉伸。窗口有子窗口（面板、提示）时不动，因为 AppKit 会带着子窗口一起移动。窗口不超出画布，所以句子再多也不会比屏幕高，以前按文字大小定的窗口就会这样。Windows 和 Linux 仍用整块窗口：它们的截图工具没有测过。
18. `windowOptions` 同时设 `DisableFullScreen` 和 `SetVisibleOnAllWorkspaces`，MyGo 据此把它做成其他应用全屏空间的辅助窗口（egoist/mygo#309），这是 macOS 文档里显示在全屏应用之上的做法。
19. `toolWindow`（`overlay/window_windows.go`）在字幕层第一次显示前把它设成工具窗口（`WS_EX_TOOLWINDOW`）。Chromium 系浏览器和 Electron 应用在被它们认为不透明的窗口盖住时停止绘制（`IsWindowVisibleAndFullyOpaque`，ui/gfx/win/hwnd_util.cc），又看不出 DirectComposition 的透明；在字幕层里拖动会把它带到前台，下面的视频变黑（2026-10-28）。工具窗口会被跳过，也不出现在 Alt+Tab 里。
20. `toolWindow`（`overlay/window_linux.go`）调用 `gtk_window_set_accept_focus(false)`：X11 窗口管理器会把键盘交给被点击的窗口，或另一个窗口关闭时剩下的那个，字幕就会从视频那里抢走键盘，Tomoru 的其他窗口也会在它后面打开。点击和拖动照样收到。
21. `hasGlass`（`overlay/glass_darwin.go`）在第一次用到时才通过 `sync.Once` 查找 `NSGlassEffectView`，而不是在程序启动时，那时 AppKit 还没加载。
22. `watchPointer`（`overlay/handles.go`）每 50 毫秒查询一次光标位置，因为让点击穿透的窗口收不到指针事件。只有指针在字幕块上停留 400 毫秒、且块里有内容时，块才接收鼠标，路过不触发。在只能看到自己窗口上指针的环境（`pointerUnseen`，Wayland），未锁定的块只要有内容就接收鼠标，并相信自己的悬停状态，直到指针静止 3 秒，因为 XWayland 不会告诉 X11 窗口指针离开去了 Wayland 窗口；锁定时那里无法显示锁。
23. 没有可抓的东西时 `watchPointer` 停下：定时器停止，直到 `View` 或 `SetLocked` 唤醒它，窗口忽略鼠标且不设输入区域，因为空的区域仍会让 MyGo 在 macOS 上跟踪每次鼠标移动、在 Windows 上每个系统时钟周期查询一次。按 50 毫秒空转时应用占一个核的 1.4%，停下后 0.0%（2026-11-20）。停下时先忽略鼠标再清除区域，唤醒时先设区域再接收鼠标：忽略优先于区域，没有区域等于整个窗口，反过来的顺序会有一瞬间拦下所有点击。
24. `capsule`（`overlay/handles.go`）让工具栏比字幕块深 12%：系统的玻璃对胶囊这么薄的面板提亮得更多，整块都是弯曲的边缘。同样深浅时，工具栏测得 (160, 154, 167)，字幕块 (134, 125, 145)（2026-10-21）。
25. `sceneMenu`（`overlay/handles.go`）用 `assets.Chevron` 标记菜单，而不是字符“▾”。系统字体没有这个字形时，后备字体更高的行把语言顶得比场景名高（Linux 上的 Noto Sans，#100）。
26. `follow`（`overlay/handles.go`）在手柄 `Dragged()` 时移动和缩放，而不是 `Pressed()`：指针离开元素时按下就结束了，窄的侧边手柄晚一帧跟上，立刻就会离开。
27. `fades`（`overlay/overlay.go`）只动画颜色（`Colors: true`）。过渡不指定时也会动画位置，控件会落在被拖动的字幕块后面。
28. `View`（`overlay/overlay.go`）在悬停或显示锁时让字幕块保持悬停开始时的高度，新文字在顶部裁掉，让工具栏或锁留在指针下。
29. `View` 每帧都构建工具栏，不悬停时不可见：MyGo 只在构建菜单按钮的那一帧送出菜单的选择，而场景菜单伸到字幕块下方，指针离开字幕块时悬停在选择之前就结束了。
30. `notePill`（`overlay/overlay.go`）只通过模糊出现和消失，因为它下面的系统玻璃不能缩放。它的位置随宽度移动，变宽时保持居中。

### 字幕

31. `presets`（`subtitle/subtitle.go`）每种字体是一份跨平台的字体族列表：先 macOS 的，再 Windows 的，再 Linux 的 Noto CJK，第一个有这个字形的来画。Windows 和 Linux 没有圆体 CJK 字体，所以那里的“圆体”由系统字体画。
32. `Scheme`（`subtitle/subtitle.go`）把颜色放在译文上，原文白色或灰色，因为人们读的是译文。
33. 一次只显示一句时，`Visible`（`subtitle/subtitle.go`）在识别下一句期间保留上一句的译文并调暗：它不是这一句的译文。已提交的行在流式译文完成前显示草稿，因为流开始时比草稿短，会把这行缩回几个词。
34. `Failure`（`subtitle/subtitle.go`）只在超时、停滞或连接失败（`net.OpError`）时说服务没有响应。Go 的 HTTP 客户端把每种失败都包成 `net.Error`，错误的 URL 也是。
35. `Align`（`subtitle/subtitle.go`）为实时文字提供左对齐：词出现在固定位置，行变长时不移动，这是 BBC 对实时字幕的要求。居中看起来像电影字幕。
36. `Draw`（`subtitle/subtitle.go`）把最新一句放在底部，视线停留的地方，旧的在上面、更淡。除了文字变长多出的部分，什么都不动，和滚动式实时字幕一样：新的一行把上面的平滑推上去，更上面的稍晚一点，像 Apple Music 的歌词，而且只上下移动。说完的句子折成长度相近的几行，像电影字幕；还在出现的随来随折。
37. `Draw` 把正在识别的句子显示成滚动的一行（“…”加最新的词），而不是逐词填满的第二行。
38. `line`（`subtitle/subtitle.go`）给每行标上语言，让汉字用那种语言的字形，中文界面里的日语读起来是日语。
39. `edgeShader`（`subtitle/subtitle.go`）一次着色器处理就从字形覆盖画出描边和阴影。每个偏移再画一遍文字，每遍都要复制一份字形，描边的角还会有缺口。
40. `gpuOnly`（`subtitle/subtitle.go`）在 MyGo 的内存渲染器（如测试中）里跳过描边，它会逐像素执行着色器；那里文字原样显示。

### 图片卡片、面板与名称

41. `translation`（`imagecard/imagecard.go`）按 GitHub Flavored Markdown 解析，但不启用数学：对白里的美元符号是钱。
42. `lines`（`imagecard/imagecard.go`）用行尾两个空格保留译文的换行，而不是反斜杠，反斜杠在文字流入时会显示出来；代码块不动。
43. `foot`（`panel/panel.go`）把“退出”放在面板底栏，离顶栏的“暂停”远，免得误退出。
44. `CloudName`（`shared/names.go`）是 “Index-Translate 35B”，和本地模型一样按大小命名，不写“云端”：它免费、默认，而且只在云端。它的其他大小会写云端（`locales.EngineCloudSize`），因为本地模型也有这些大小。
45. `SourceMenu`（`shared/status.go`）是原生菜单，和 macOS 的弹出按钮一样：不会被窗口截断，应用多时可以滚动。

## 影响与取舍

字幕层很多地方取决于 MyGo 和各窗口系统如何对待让点击穿透的透明窗口；升级 MyGo 或支持新的桌面时，要按 AGENTS.md 的要求，在前面开着别的应用的情况下上屏检查。引导依赖 macOS 自己的权限提示和重启，只能人工测试。

## 证据

- `app/internal/ui/overlay/overlay_test.go`：`TestWindowOverFullScreen`、`TestFitsDrawing`、`TestToolbarLine`、`TestInputRegion`、`TestParks`、`TestWakesWatcher`、`TestSceneMenu`、`TestUnlock`。
- `app/internal/ui/subtitle/subtitle_test.go`：`TestVisible`、`TestFailure`、`TestBalance`。
- `app/internal/ui/shared/names_test.go`：引擎名称，包括 `CloudName`。
