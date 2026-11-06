package kit

import (
	"math"

	"github.com/egoist/mygo/ui"
)

// The sidebar's width: as the longest name of its pages needs, within these.
const navMin, navMax = 184, 252

// NavWidth is the sidebar's width for its pages, entries: room for the longest name, in the
// language shown, bold as the page shown has it.
func NavWidth(c *ui.Context, entries ...[]NavEntry) float32 {
	longest := float32(0)
	for _, es := range entries {
		for _, e := range es {
			w, _ := c.MeasureText(0, ui.Span{Text: e.Label, Size: TextBody, Weight: WeightBold})
			longest = max(longest, w)
		}
	}
	// The item's padding, its icon and the gap after it, and the sidebar's padding, with a little
	// for rounding.
	w := longest + 2*navItemPad + IconSizeNav + navItemGap + 2*navPad + 4
	return min(max(float32(math.Ceil(float64(w))), navMin), navMax)
}

// The padding of the sidebar and of its items, and the gap after an item's icon.
var navPad, navItemPad, navItemGap = Space(2), Space(3), Space(2)

// NavEntry is a page of the window's sidebar.
type NavEntry struct {
	ID    string
	Icon  *ui.SVG
	Label string
}

// AppFrame is the app window of docs/design-system.md: the night's frame, its sidebar with the
// pages, top's first and bottom's at its foot, and the paper, on which page
// builds the page *selected names. The window hides its title bar; the sidebar's top drags it.
func AppFrame(c *ui.Context, selected *string, top, bottom []NavEntry, page func()) {
	Paper(c)
	p := Colors(c)
	bar := c.TitleBar()
	c.Root().Background(p.Night)
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Width(NavWidth(c, top, bottom)).Shrink(0).Padding(0, navPad, Space(3)).Gap(Space(6)).Children(func() {
			// The window's controls' band, which drags the window. The window's title and the Dock
			// name the app already: the pages start under it.
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
				// The window's controls on this side, as on Windows: a band over the paper for them, which
				// drags the window.
				ui.Box(c).Height(bar.Height).DragWindow()
				top = 0
			}
			ui.Column(c).Grow(1).MinWidth(0).Margin(top, Space(2), Space(2), 0).Radius(RadiusPaper).Background(p.Paper).Clip().Children(func() {
				// The scroll bar keeps clear of the paper's rounded corners: its ends 9 from the paper's
				// top and bottom (MyGo leaves 2 more at the ends of its track), a little more than the 7
				// at its side.
				end := float32(7)
				ui.Scroll(c).Grow(1).ScrollbarInsets(end, Space(1), end).Children(func() {
					// Each page is its own: switching shows it at once, rather than as the last page changing.
					ui.Column(c.Key(*selected)).Padding(Space(8)).Gap(SectionGap).Children(page)
				})
			})
		})
	})
}

// NavItem is a page of the sidebar: its icon and name, on the night; the page shown, on the paper.
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
