# ADR 007：UI kit 与设计 token

[English](007-ui-kit-and-design-tokens.md)

日期：2026-11-21。状态：采用。

## 背景

这些决定原本写在代码注释里，代码不再带注释后移到这里。范围是 `app/internal/ui/kit` 和 `app/internal/ui/assets`：控件、窗框、快捷键输入框和 token。[设计语言](../design-system_zh.md)已经写明的不再重复。

## 决策

1. `Destructive`（`kit/button.go`）用于停止、取消进行中的任务、删除、关闭或退出：悬停时底色变红，和标题栏的关闭按钮一样。对话框的“取消”只是退回，所以 `confirmActions`（`kit/dialog.go`）给它普通按钮。
2. `press`（`kit/button.go`）让静止时没有底色的危险按钮从 `CloseHover.Alpha(0)` 起变，而不是 `ui.Transparent`。MyGo 的 `AnimateColor` 直接插值 RGBA，从透明黑渐变会经过灰色。
3. `TurnedIconButton`（`kit/button.go`）转的是图标，不是按钮。MyGo 画旋转的图标是清晰的，旋转整个元素则要重采样，看着发虚。
4. `choiceRow`（`kit/choice.go`）和 `listItem`（`kit/list.go`）的标签页设 `Background(ui.Transparent)`，自己画 `Shown` 或 `Band` 底色，因为 MyGo 的 `ButtonBase.Selected` 会涂主题强调色。
5. `offOpacity`（`kit/controls.go`）用透明度淡化 `SettingRowOff` 里的滑块，因为 MyGo 的滑块用自己的颜色，行改不了它的颜色。同理，MyGo 会淡化禁用元素里的一切，所以 `optionRow`（`kit/option.go`）只禁用 `Off` 选项的单选点，文字保持清晰。
6. `Parts`（`kit/list.go`）记住排版时的宽度，宽度变了就再要一帧（`c.After(0)`），因为 MyGo 的 `LayoutBounds` 是上一帧的布局。
7. `AppFrame`（`kit/frame.go`）的侧栏不放字标或标题：页面紧接在窗口控制按钮那条带下面开始，窗口标题和 Dock 已经说明了是哪个应用。
8. `AppFrame` 用所选页面给页面列设 key（`c.Key(*selected)`），切换时新页面立刻出现，而不是把旧页面的元素动画成新页面。
9. `Heatmap`（`kit/heatmap.go`）把星期标签的高度固定为 `heatSize`。Segoe UI 的行高比 SF 大，在 Windows 上会让行距比列距宽。
10. `Accelerator`（`kit/shortcut.go`）接受带 Control、Option 或 Command 的键，或单独的功能键。只带 Shift 的不接受：作为全局快捷键，它会从每个应用里抢走一个大写字母。
11. `ShortcutField`（`kit/shortcut_field.go`）录制时当前快捷键仍然注册着。系统在任何应用之前拿走这些键，所以按下它会照常执行，不会进到输入框。
12. `ShortcutField` 录 `ev.PhysicalKey`，只有位置未知时才退回 `ev.Key`，因为全局快捷键按键位注册，而不是按键盘布局的字符。
13. `screenSettings`（`kit/screen.go`）是两种录制权限共用的系统设置 URL（`Privacy_ScreenCapture`）：macOS 把“仅系统录音”列在“录屏与系统录音”下面，没有单独的 URL。
14. `AppIcon`（`assets/assets.go`）按 macOS 图标网格绘制，由 `scripts/app-icon.swift` 生成：824/1024 的圆角方块居中，带柔和投影，留约十分之一边距。UI 原样显示，不再加圆角或阴影。
15. `Arrive`（`kit/tokens.go`）用关键帧播放入场，因为 MyGo 的进入过渡需要元素出现之前的一帧，和容器一起构建的东西不会播放。

## 影响与取舍

决策 2 到 6 绕开的是 MyGo 目前的绘制和布局方式；升级 MyGo 时先检查它们，再决定保留还是去掉。快捷键规则在所有平台一致，一个平台能用的快捷键在其他平台也能用。

## 证据

- `app/internal/ui/kit/shortcut_test.go`：`TestAccelerator` 拒绝只带 Shift 的键。
- `app/internal/ui/kit/choice_test.go`：`TestOptionOff`：`Off` 选项不能被选中。
- `app/internal/ui/kit/heatmap_test.go`：热力图的布局和悬停。
