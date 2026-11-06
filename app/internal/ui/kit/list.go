package kit

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

// ListItem is an item of a List: a scene, a session kept.
type ListItem struct {
	Key   any    // the item's own, as a session's path: its state stays with it across frames
	On    bool   // shown beside the list
	Head  string // what starts a part of the list over it, as a day; "" for none
	Title string
	Meta  []string // what it is, in parts, under its title
	Lead  func()   // what goes at its start: a time
	Trail func()   // what goes at its end: In use, an action on it
}

// List lists items as Options lists rows, their content as far in from the card's edge as a
// Card's: the one shown on the band, a rule between two not shown, each its lead, its title over
// its meta on a line each (OneLine, Parts), and its trail at its end, lead and trail centred on
// the two lines. A list with one shown is a list of tabs, the item shown beside it. Put it in an
// OptionsCard. It returns the index of the item clicked, or -1.
func List(c *ui.Context, items []ListItem) int {
	p := Colors(c)
	clicked := -1
	tabs := false
	for _, t := range items {
		tabs = tabs || t.On
	}
	list := ui.Column(c)
	if tabs {
		list = list.Role(ui.RoleTabList).FocusGroup(ui.Vertical)
	}
	list.Children(func() {
		for i, t := range items {
			switch {
			case t.Head != "":
				top := Space(2)
				if i > 0 {
					top = Space(4)
				}
				ui.Text(c.Key([2]any{"head", t.Key}), t.Head).FontSize(TextCaption).FontWeight(WeightBold).TextColor(p.InkMuted).
					Padding(top, Space(4), Space(1)).MaxLines(1)
			case i > 0:
				col := p.LineSoft
				if t.On || items[i-1].On {
					col = ui.Transparent
				}
				ui.Box(c.Key([2]any{"rule", t.Key})).Height(1).Margin(0, Space(4)).Background(col)
			}
			if listItem(c.Key(t.Key), t, tabs).Clicked() {
				clicked = i
			}
		}
	})
	return clicked
}

// listItem is an item of a List, returning the part that shows it.
func listItem(c *ui.Context, t ListItem, tab bool) ui.Element {
	p := Colors(c)
	row := ui.Row(c).Padding(Space(3), Space(4)).Gap(Space(3)).AlignItems(ui.Center).Radius(RadiusTile)
	face := p.Band.Alpha(0)
	switch {
	case t.On:
		face = p.Band
	case row.Hovered():
		face = p.Paper
	}
	var b ui.Element
	row.Background(row.AnimateColor("face", face, hover)).Children(func() {
		b = ui.ButtonBase(c).Grow(1).MinWidth(0).Gap(Space(3)).Justify(ui.Start).AlignItems(ui.Center).
			Label(t.Title).Cursor(ui.CursorPointer)
		if tab {
			// Selected paints the theme's accent: the row's face is the item's.
			b = b.Role(ui.RoleTab).Selected(t.On).Background(ui.Transparent)
		}
		b.Children(func() {
			if t.Lead != nil {
				t.Lead()
			}
			ui.Column(c).Grow(1).MinWidth(0).Gap(Space(1)).Children(func() {
				OneLine(c, t.Title, TextBody, WeightBold, p.Ink)
				if len(t.Meta) > 0 {
					Parts(c, t.Meta, TextCaption, p.InkMuted)
				}
			})
		})
		if t.Trail != nil {
			t.Trail()
		}
	})
	return b
}

// OneLine is s on one line, of size, weight (0 for the body's) and ink: cut with an ellipsis where
// it doesn't fit, and then told in full in its tooltip.
func OneLine(c *ui.Context, s string, size float32, weight int, ink ui.Color) ui.Element {
	t := ui.Text(c, s).FontSize(size).TextColor(ink).MaxLines(1).MinWidth(0)
	if weight != 0 {
		t = t.FontWeight(weight)
	}
	if room := t.LayoutBounds().W; room > 0 {
		if w, _ := c.MeasureText(0, ui.Span{Text: s, Size: size, Weight: weight}); w > room+.5 {
			t = t.Tooltip(s)
		}
	}
	return t
}

// partSep is what goes between the parts of a line of Parts.
const partSep = " · "

// Parts is parts on one line, partSep between them, of size and ink: as many from the first as
// fit, never one cut short but the first alone, and then all of them in its tooltip.
func Parts(c *ui.Context, parts []string, size float32, ink ui.Color) ui.Element {
	all := strings.Join(parts, partSep)
	shown := all
	box := ui.Column(c).MinWidth(0)
	// The room is the last frame's: once it changes, one more frame fits the parts to it.
	room, was := box.LayoutBounds().W, ui.Local(box, "room", func() float32 { return -1 })
	if room != *was {
		*was = room
		c.After(0)
	}
	if room > 0 {
		for n := len(parts); n > 0; n-- {
			shown = strings.Join(parts[:n], partSep)
			if w, _ := c.MeasureText(0, ui.Span{Text: shown, Size: size}); w <= room+.5 {
				break
			}
		}
	}
	return box.Children(func() {
		t := ui.Text(c, shown).FontSize(size).TextColor(ink).MaxLines(1).MinWidth(0)
		if shown != all {
			t.Tooltip(all)
		}
	})
}
