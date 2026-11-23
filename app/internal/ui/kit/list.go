package kit

import (
	"strings"

	"github.com/egoist/mygo/ui"
)

type ListItem struct {
	Key   any
	Shown bool
	Head  string
	Title string
	Meta  []string
	Lead  func()
	Trail func()
}

func List(c *ui.Context, items []ListItem) int {
	p := Colors(c)
	clicked := -1
	tabs := false
	for _, t := range items {
		tabs = tabs || t.Shown
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
				if t.Shown || items[i-1].Shown {
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

func listItem(c *ui.Context, t ListItem, tab bool) ui.Element {
	p := Colors(c)
	row := ui.Row(c).Padding(Space(3), Space(4)).Gap(Space(3)).AlignItems(ui.Center).Radius(RadiusTile)
	hovered := row.Hovered()
	face := AnimateColor(c, row, "face", hover, func(p *Palette) ui.Color {
		switch {
		case t.Shown:
			return p.Band
		case hovered:
			return p.Paper
		default:
			return p.Band.Alpha(0)
		}
	})
	var b ui.Element
	row.Background(face).Children(func() {
		b = ui.ButtonBase(c).Grow(1).MinWidth(0).Gap(Space(3)).Justify(ui.Start).AlignItems(ui.Center).
			Label(t.Title).Cursor(ui.CursorPointer)
		if tab {
			b = b.Role(ui.RoleTab).Selected(t.Shown).Background(ui.Transparent)
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

const partSep = " · "

func Parts(c *ui.Context, parts []string, size float32, ink ui.Color) ui.Element {
	all := strings.Join(parts, partSep)
	shown := all
	box := ui.Column(c).MinWidth(0)
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
