package kit

import (
	"math"

	"github.com/egoist/mygo/ui"
)

const navMin, navMax = 184, 252

func NavWidth(c *ui.Context, entries ...[]NavEntry) float32 {
	longest := float32(0)
	for _, es := range entries {
		for _, e := range es {
			w, _ := c.MeasureText(0, ui.Span{Text: e.Label, Size: TextBody, Weight: WeightBold})
			longest = max(longest, w)
		}
	}
	w := longest + 2*navItemPad + IconSizeNav + navItemGap + 2*navPad + navTextSlack
	return min(max(float32(math.Ceil(float64(w))), navMin), navMax)
}

var navPad, navItemPad, navItemGap = Space(2), Space(3), Space(2)

const navTextSlack = 4

const (
	paperScrollEnd = 9
	myGoTrackPad   = 2
)

type NavEntry struct {
	ID    string
	Icon  *ui.SVG
	Label string
}

func AppFrame(c *ui.Context, selected *string, top, bottom []NavEntry, page func()) {
	Paper(c)
	p := Colors(c)
	bar := c.TitleBar()
	c.Root().Background(p.Night)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(NavWidth(c, top, bottom)).Shrink(0).Padding(0, navPad, Space(3)).Gap(Space(6)).Children(func() {
			ui.Box(c).Height(max(bar.Height, Space(3))).DragWindow()
			nav := func(entries []NavEntry) {
				ui.Column(c).Gap(Space(1)).Role(ui.RoleTabList).FocusGroup(ui.Vertical).Children(func() {
					for _, e := range entries {
						if NavItem(c.Key(e.ID), *selected == e.ID, e.Icon, e.Label).Clicked() {
							*selected = e.ID
						}
					}
				})
			}
			nav(top)
			ui.Spacer(c)
			nav(bottom)
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			top := Space(2)
			if bar.Right > 0 {
				ui.Box(c).Height(bar.Height).DragWindow()
				top = 0
			}
			ui.Column(c).Grow(1).MinWidth(0).Margin(top, Space(2), Space(2), 0).Radius(RadiusPaper).Background(p.Paper).Clip().Children(func() {
				end := float32(paperScrollEnd - myGoTrackPad)
				ui.Scroll(c).Grow(1).ScrollbarInsets(end, Space(1), end).Children(func() {
					ui.Column(c.Key(*selected)).Padding(Space(8)).Gap(SectionGap).Children(page)
				})
			})
		})
	})
}

func NavItem(c *ui.Context, on bool, icon *ui.SVG, label string) ui.Element {
	p := Colors(c)
	const h = 40
	b := ui.ButtonBase(c).Height(h).Padding(0, navItemPad).Gap(navItemGap).Radius(Pill(h)).FontSize(TextBody).
		Justify(ui.Start).Role(ui.RoleTab).Selected(on).Cursor(ui.CursorPointer)
	ink, tint := p.NightText, p.NightText
	switch {
	case on:
		b = b.Background(b.AnimateColor("face", p.NavOn, Fade)).FontWeight(WeightBold)
		ink, tint = p.Ink, p.LanternText
	case b.Hovered():
		b = b.Background(b.AnimateColor("face", p.NightText.Alpha(.1), Fade))
	default:
		b = b.Background(b.AnimateColor("face", p.NightText.Alpha(0), Fade))
	}
	return b.TextColor(ink).Children(func() {
		ui.Icon(c, icon).FontSize(IconSizeNav).TextColor(tint)
		ui.Text(c, label).SingleLine()
	})
}
