package kit

import (
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

type Option struct {
	Key    any
	On     bool
	Title  string
	Tag    string
	Accent bool
	Done   string
	Meta   string
	Warn   bool
	Off    bool
	Action func()
	Below  func()
}

const (
	dotSize = 20
	optGap  = 12
)

func Options(c *ui.Context, rows []Option) int {
	p := Colors(c)
	clicked := -1
	ui.Column(c).Children(func() {
		for i, o := range rows {
			if i > 0 {
				col := p.LineSoft
				if o.On || rows[i-1].On {
					col = ui.Transparent
				}
				ui.Box(c.Key([2]any{"rule", o.Key})).Height(1).Margin(0, Space(3), 0, Space(3)+dotSize+optGap).Background(col)
			}
			if optionRow(c.Key(o.Key), o).Clicked() {
				clicked = i
			}
		}
	})
	return clicked
}

func optionRow(c *ui.Context, o Option) ui.Element {
	p := Colors(c)
	row := ui.Column(c).Padding(Space(3)).Gap(Space(2)).Radius(RadiusTile)
	face := p.Band.Alpha(0)
	if o.On {
		face = p.Band
	}
	text := func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(Space(1)).Children(func() {
			ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Text(c, o.Title).FontSize(TextBody).FontWeight(WeightBold).TextColor(p.Ink)
				switch {
				case o.Tag != "" && o.Accent:
					AccentTag(c, o.Tag)
				case o.Tag != "":
					Tag(c, o.Tag)
				}
				if o.Done != "" {
					OkTag(c, o.Done)
				}
			})
			if o.Meta != "" {
				ink := p.InkMuted
				if o.Warn {
					ink = p.Danger
				}
				ui.Text(c, o.Meta).FontSize(TextCaption).TextColor(ink).MaxLines(2)
			}
		})
	}
	var pick ui.Element
	row.Background(row.AnimateColor("face", face, Fade)).Children(func() {
		ui.Row(c).Gap(Space(3)).AlignItems(ui.Center).Children(func() {
			if o.Off {
				ui.Row(c).Grow(1).MinWidth(0).Gap(optGap).AlignItems(ui.Center).Children(func() {
					pick = ui.ButtonBase(c).Shrink(0).Role(ui.RoleRadio).Checked(false).Label(o.Title).Disabled(true).
						Children(func() { dotOff(c) })
					text()
				})
			} else {
				pick = ui.ButtonBase(c).Grow(1).MinWidth(0).Gap(optGap).Justify(ui.Start).AlignItems(ui.Center).
					Role(ui.RoleRadio).Checked(o.On).Label(o.Title).Cursor(ui.CursorPointer).
					Children(func() {
						Dot(c, o.On)
						text()
					})
			}
			if o.Action != nil {
				o.Action()
			}
		})
		if o.Below != nil {
			ui.Column(c).Gap(Space(1)).Padding(0, 0, 0, dotSize+optGap).Children(o.Below)
		}
	})
	return pick
}

func OptionsCard(c *ui.Context) ui.Element {
	return Card(c).Padding(Space(1)).Gap(0)
}

func AddRow(c *ui.Context, label string) ui.Element {
	p := Colors(c)
	b := ui.ButtonBase(c).Padding(Space(3)).Gap(optGap).Justify(ui.Start).Radius(RadiusTile).
		Label(label).Cursor(ui.CursorPointer).TextColor(p.InkMuted)
	if b.Hovered() {
		b = b.Background(p.Paper).TextColor(p.Ink)
	}
	return b.Children(func() {
		ui.Box(c).Size(dotSize, dotSize).Center().Shrink(0).Children(func() { ui.Icon(c, assets.Plus).FontSize(IconSizeSmall) })
		ui.Text(c, label).FontSize(TextBody).FontWeight(WeightBold).SingleLine()
	})
}

func Dot(c *ui.Context, on bool) ui.Element {
	p := Colors(c)
	ring, col, heart := float32(2), p.DotRing, p.Card
	if on {
		ring, col, heart = 6, p.Lantern, p.Starlight
	}
	dot := ui.Box(c).Size(dotSize, dotSize).Radius(Pill(dotSize)).Shrink(0)
	a := ui.Animation{Duration: 180 * time.Millisecond}
	return dot.Background(dot.AnimateColor("heart", heart, a)).Border(dot.AnimateTo("ring", ring, a), dot.AnimateColor("color", col, a))
}

func dotOff(c *ui.Context) ui.Element {
	return ui.Box(c).Size(dotSize, dotSize).Radius(Pill(dotSize)).Shrink(0).Border(2, Colors(c).DotRing).BorderStyle(ui.BorderDashed)
}

func OptionProgress(c *ui.Context, value float64, amount string) ui.Element {
	p := Colors(c)
	return ui.Row(c).Gap(Space(3)).AlignItems(ui.Center).Children(func() {
		Progress(c, value).Grow(1).MinWidth(0)
		ui.Text(c, amount).NoWrap().FontSize(TextCaption).FontFeatures(Tabular).TextColor(p.InkMuted).
			ContentTransition(ui.ContentTransition{Numeric: true})
	})
}

func Caption(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(TextCaption).TextColor(Colors(c).InkMuted)
}

func CaptionLink(c *ui.Context, s, link, url string) ui.Element {
	return Linked(c, s, link, url, TextCaption, Colors(c).InkMuted)
}

func Linked(c *ui.Context, s, link, url string, size float32, ink ui.Color) ui.Element {
	amber := Colors(c).LanternText
	before, after, ok := strings.Cut(s, link)
	switch {
	case link == "" || !ok:
		return ui.Text(c, s).FontSize(size).TextColor(ink)
	case before == "" && after == "":
		return ui.Link(c, link, url).FontSize(size).TextColor(amber)
	}
	return ui.RichText(c).FontSize(size).TextColor(ink).Children(func() {
		if before != "" {
			ui.Text(c, before)
		}
		ui.Link(c, link, url).TextColor(amber)
		if after != "" {
			ui.Text(c, after)
		}
	})
}

func Waiting(c *ui.Context, s string) ui.Element {
	return ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Children(func() {
		loading.Ring(c, loading.Options{Size: TextCaption, Color: Colors(c).Lantern}).Label(s)
		Caption(c, s)
	})
}

func Warning(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(TextCaption).TextColor(Colors(c).LanternText)
}
