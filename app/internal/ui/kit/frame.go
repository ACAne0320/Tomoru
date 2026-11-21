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

// TitleBarHeight is the height of the settings window's title bar on Windows and Linux
// (mygo.WindowOptions.TitleBarHeight), and of the band at the top of its paper that drags it.
const TitleBarHeight = 36

// navLine is the width of the sidebar card's outline.
const navLine = 1

// AppFrame is the app window of docs/design-system.md: the paper, the sidebar's card on it with
// the pages, top's first and bottom's at its foot, and beside it the page, on which page builds
// the page *selected names. The window hides its title bar; its buttons sit on the paper or the
// card, which follow the appearance, and the card's top and the paper's drag it.
func AppFrame(c *ui.Context, selected *string, top, bottom []NavEntry, page func()) {
	Paper(c)
	p := Colors(c)
	bar := c.TitleBar()
	c.Root().Background(p.Paper)
	// The pages start as far down on every platform, below the traffic lights where they are.
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
				band = max(band, bar.Height) // the window's buttons are on this side, as on Windows
			}
			ui.Box(c).Height(band).Shrink(0).DragWindow()
			// The scroll bar keeps clear of the window's rounded corner at its foot.
			sc := ui.Scroll(c).Grow(1).ScrollbarInsets(0, Space(1), Space(3))
			s := ui.Local(sc, "scroll", func() ui.ScrollState { return ui.ScrollState{} })
			sc.TrackScroll(s).Children(func() {
				// Each page is its own: switching shows it at once, rather than as the last page changing.
				ui.Column(c.Key(*selected)).Padding(Space(8)).Gap(SectionGap).Children(page)
			})
			// What is scrolled up fades out under the band, rather than being cut at it.
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

// NavItem is a page of the sidebar: its icon and name, on the card; the page shown, on a piece of
// the night.
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
