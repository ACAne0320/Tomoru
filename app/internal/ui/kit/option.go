package kit

import (
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/loading"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
)

// Option is a row of Options: a choice among a few, as an engine of a side.
type Option struct {
	Key    any    // the row's own, as an engine's ID: its state stays with it across frames
	On     bool   // chosen
	Title  string // its name
	Tag    string // a word beside the name, in amber if Accent: Recommended, Built in
	Accent bool
	Meta   string // what it is, under the name
	Action func() // what is done with it, at the row's end: Download, Delete, Edit
	Below  func() // what goes under its name and meta, across the row: a download's progress, a note
}

// The geometry of an Option's row: the dot, and the gap after it, where the name, its meta and
// what goes below them start.
const (
	dotSize = 20
	optGap  = 12
)

// Options lists rows, the one chosen on the band, a rule between two not chosen: each its dot,
// its name and tag over its meta, and its action at its end, the dot and the action centred on
// the name and meta together, whatever goes below them. Put it in an OptionsCard. It returns the
// index of the row whose name was clicked, or -1.
func Options(c *ui.Context, rows []Option) int {
	p := Colors(c)
	clicked := -1
	ui.Column(c).Children(func() {
		for i, o := range rows {
			if i > 0 {
				// A rule from the names' start to the row's end, apart from the band; there either way,
				// so that choosing doesn't move the rows.
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

// optionRow is a row of Options, returning the part that chooses it.
func optionRow(c *ui.Context, o Option) ui.Element {
	p := Colors(c)
	row := ui.Column(c).Padding(Space(3)).Gap(Space(2)).Radius(RadiusTile)
	face := p.Band.Alpha(0)
	if o.On {
		face = p.Band
	}
	var pick ui.Element
	row.Background(row.AnimateColor("face", face, Fade)).Children(func() {
		ui.Row(c).Gap(Space(3)).AlignItems(ui.Center).Children(func() {
			pick = ui.ButtonBase(c).Grow(1).MinWidth(0).Gap(optGap).Justify(ui.Start).AlignItems(ui.Center).
				Role(ui.RoleRadio).Checked(o.On).Label(o.Title).Cursor(ui.CursorPointer)
			pick.Children(func() {
				Dot(c, o.On)
				ui.Column(c).Grow(1).MinWidth(0).Gap(Space(1)).Children(func() {
					ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Wrap().Children(func() {
						ui.Text(c, o.Title).FontSize(TextBody).FontWeight(WeightBold).TextColor(p.Ink)
						switch {
						case o.Tag != "" && o.Accent:
							AccentTag(c, o.Tag)
						case o.Tag != "":
							Tag(c, o.Tag)
						}
					})
					if o.Meta != "" {
						ui.Text(c, o.Meta).FontSize(TextCaption).TextColor(p.InkMuted).MaxLines(2)
					}
				})
			})
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

// OptionsCard is the card a list of Options is in: its rows' bands 4 in from its edges, so that
// their corners follow its own.
func OptionsCard(c *ui.Context) ui.Element {
	return Card(c).Padding(Space(1)).Gap(0)
}

// AddRow ends a list of Options: a plus where their dots are, and label, adding one of one's own.
// Clicked reports a click.
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

// Dot is a radio button's dot: a ring, filling with the lantern's amber as it is chosen, a white
// heart left in it.
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

// OptionProgress is a download's progress under an Option's meta: its bar, across, and how far it
// is, at its end.
func OptionProgress(c *ui.Context, value float64, amount string) ui.Element {
	p := Colors(c)
	return ui.Row(c).Gap(Space(3)).AlignItems(ui.Center).Children(func() {
		Progress(c, value).Grow(1).MinWidth(0)
		ui.Text(c, amount).NoWrap().FontSize(TextCaption).FontFeatures(Tabular).TextColor(p.InkMuted).
			ContentTransition(ui.ContentTransition{Numeric: true})
	})
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
	return ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Children(func() {
		loading.Ring(c, loading.Options{Size: TextCaption, Color: Colors(c).Lantern}).Label(s)
		Caption(c, s)
	})
}

// Warning is a note to heed, in amber ink: not enough memory, a download that failed.
func Warning(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(TextCaption).TextColor(Colors(c).LanternText)
}
