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

type NavEntry struct {
	ID    string
	Icon  *ui.SVG
	Label string
}

const TitleBarHeight = 36

const navLine = 1

var windowCornerClear = Space(3)

func AppFrame(c *ui.Context, selected *string, top, bottom []NavEntry, page func()) {
	Paper(c)
	p := Colors(c)
	bar := c.TitleBar()
	c.Root().Background(p.Paper)
	navTop := max(Space(12), bar.Bottom+Space(3))
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(NavWidth(c, top, bottom)).Shrink(0).Margin(Space(2), 0, Space(2), Space(2)).
			Radius(RadiusSidebar).Background(p.Sand).Border(navLine, p.NavLine).Children(func() {
			ui.Box(c).Height(navTop - Space(2) - navLine).Shrink(0).DragWindow()
			ui.Column(c).Grow(1).Padding(0, navPad, Space(3)).Gap(Space(6)).Children(func() {
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
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			band := float32(TitleBarHeight)
			if bar.Right > 0 {
				band = max(band, bar.Height)
			}
			ui.Box(c).Height(band).Shrink(0).DragWindow()
			sc := ui.Scroll(c).Grow(1).ScrollbarInsets(0, Space(1), windowCornerClear)
			s := ui.Local(sc, "scroll", func() ui.ScrollState { return ui.ScrollState{} })
			sc.TrackScroll(s).Children(func() {
				ui.Column(c.Key(*selected)).Padding(Space(8)).Gap(SectionGap).Children(page)
			})
			scrolled := float32(0)
			if s.Y > 1 {
				scrolled = 1
			}
			fade := ui.Box(c).Absolute().Top(band).Left(0).Right(0).Height(Space(6)).PassThrough().
				LinearGradient(ui.LinearGradient{From: p.Paper, To: p.Paper.Alpha(0), Angle: 180})
			fade.Opacity(fade.AnimateTo("scrolled", scrolled, Fade))
		})
	})
}

func NavItem(c *ui.Context, on bool, icon *ui.SVG, label string) ui.Element {
	p := Colors(c)
	const h = 40
	b := ui.ButtonBase(c).Height(h).Padding(0, navItemPad).Gap(navItemGap).Radius(Pill(h)).FontSize(TextBody).
		Justify(ui.Start).Role(ui.RoleTab).Selected(on).Cursor(ui.CursorPointer)
	ink, tint := p.Ink, p.InkMuted
	switch {
	case on:
		b = b.Background(b.AnimateColor("face", p.NavOn, Fade)).FontWeight(WeightBold)
		ink, tint = p.NightTitle, p.Lantern
	case b.Hovered():
		b = b.Background(b.AnimateColor("face", p.Ink.Alpha(.06), Fade))
	default:
		b = b.Background(b.AnimateColor("face", p.Ink.Alpha(0), Fade))
	}
	return b.TextColor(ink).Children(func() {
		ui.Icon(c, icon).FontSize(IconSizeNav).TextColor(tint)
		ui.Text(c, label).SingleLine()
	})
}
