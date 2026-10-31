package kit

import (
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// OptionRow is a choice among a few listed, as the engines of a side: a dot, the title
// and a tag beside it, in amber if accent, what meta says under it; trailing builds what is done with it at the end,
// as a download. It returns the part that chooses it: Clicked reports the choice.
func OptionRow(c *ui.Context, on bool, title, tag string, accent bool, meta string, trailing func()) ui.Element {
	p := Colors(c)
	row := ui.Row(c).Gap(Space(2.5)).AlignItems(ui.Center).Padding(Space(1), Space(1.5), Space(1), Space(1)).Radius(RadiusTile)
	if on {
		row = row.Background(p.Paper).Border(2, p.Lantern)
	} else {
		row = row.Border(2, ui.Transparent)
	}
	var pick ui.Element
	row.Children(func() {
		pick = ui.ButtonBase(c).Grow(1).MinWidth(0).Padding(Space(2.5)).Gap(Space(3)).Justify(ui.Start).Radius(RadiusTile - 4).
			Role(ui.RoleRadio).Checked(on).Label(title).Cursor(ui.CursorPointer)
		pick.Children(func() {
			Dot(c, on)
			ui.Column(c).Grow(1).MinWidth(0).Gap(Space(0.75)).Children(func() {
				ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Wrap().Children(func() {
					ui.Text(c, title).FontSize(TextBody + 1).FontWeight(WeightBold).TextColor(p.Ink)
					switch {
					case tag != "" && accent:
						AccentTag(c, tag)
					case tag != "":
						Tag(c, tag)
					}
				})
				if meta != "" {
					ui.Text(c, meta).FontSize(TextCaption).TextColor(p.InkMuted).MaxLines(2)
				}
			})
		})
		if trailing != nil {
			trailing()
		}
	})
	return pick
}

// AddRow ends a list of OptionRows: a plus where their dots are, and label, adding one of one's
// own. Clicked reports a click.
func AddRow(c *ui.Context, label string) ui.Element {
	p := Colors(c)
	b := ui.ButtonBase(c).Padding(Space(2.5)).Margin(0, 0, 0, Space(1)).Gap(Space(3)).Justify(ui.Start).Radius(RadiusTile - 4).
		Label(label).Cursor(ui.CursorPointer).TextColor(p.InkMuted)
	if b.Hovered() {
		b = b.Background(p.Fill).TextColor(p.Ink)
	}
	return b.Children(func() {
		ui.Box(c).Size(20, 20).Center().Shrink(0).Children(func() { ui.Icon(c, assets.Plus).FontSize(TextSmall) })
		ui.Text(c, label).FontSize(TextBody).FontWeight(WeightBold).SingleLine()
	})
}

// Dot is a radio button's dot: a ring, filling with the lantern's amber as it is chosen.
func Dot(c *ui.Context, on bool) ui.Element {
	p := Colors(c)
	const d = 20
	ring, col := float32(2), p.LineStrong.Mix(p.InkWarm, .25)
	if on {
		ring, col = 6, p.Lantern
	}
	dot := ui.Box(c).Size(d, d).Radius(Pill(d)).Background(p.Card).Shrink(0)
	a := ui.Animation{Duration: 180 * time.Millisecond}
	return dot.Border(dot.AnimateTo("ring", ring, a), dot.AnimateColor("color", col, a))
}

// Caption is a note in muted ink.
func Caption(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(TextCaption).TextColor(Colors(c).InkMuted)
}

// CaptionLink is a caption with a link in it: link, a part of s, opens url.
func CaptionLink(c *ui.Context, s, link, url string) ui.Element {
	return Linked(c, s, link, url, TextCaption, Colors(c).InkMuted)
}

// Linked is text of size, in ink, with a link in it, in amber: link, a part of s, opens url.
// Without link ("", or not in s), s is text as it is.
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

// Waiting is a caption for work under way, a test or a translation, after a ring turning in amber.
func Waiting(c *ui.Context, s string) ui.Element {
	return ui.Row(c).Gap(Space(1.5)).AlignItems(ui.Center).Children(func() {
		loading.Ring(c, loading.Options{Size: TextCaption, Color: Colors(c).Lantern}).Label(s)
		Caption(c, s)
	})
}

// Warning is a note to heed, in amber ink: not enough memory, a download that failed.
func Warning(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(TextCaption).TextColor(Colors(c).LanternText)
}
