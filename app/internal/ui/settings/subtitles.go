package settings

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

// subtitles is the subtitles' style: a preview of them, as they roll in, and what they look like,
// their text and colors, and what shows when, each a card (docs/design-system.md).
func (s *Settings) subtitles(c *ui.Context) {
	before := s.core.Preferences()
	draft := before.Clone()
	p := &draft
	defer func() { s.core.ApplyPreferences(before, draft) }()
	pal := kit.Colors(c)
	ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.TabSubtitles()).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(pal.Ink).Grow(1).MaxLines(1)
		if kit.Button(c, kit.Plain, kit.Medium, assets.Move, locales.AdjustPosition()).Clicked() {
			s.nav.StartAdjust()
		}
		if kit.IconButton(c, kit.Plain, kit.Medium, assets.Reset, locales.ResetStyle()).Clicked() {
			p.ResetStyle()
			s.customColors = false
		}
	})
	samples, next := subtitle.Rolling(*p, c.Now())
	c.After(next)
	w, _ := c.Size()
	kit.Stage(c).Height(250).Padding(0, kit.Space(10), kit.Space(7)).Children(func() {
		ui.Column(c).Gap(kit.Space(2)).AlignSelf(ui.Stretch).Children(func() {
			subtitle.Draw(c, samples, *p, .8, max(320, w-s.navWidth-kit.Space(44)), subtitle.NoBlock)
		})
	})
	kit.Groups(c, func() {
		card := func(title string, rows func()) {
			kit.Group(c, title, nil, kit.RowsCard, rows).Grow(1).Basis(300)
		}
		card(locales.StyleLook(), func() { s.styleLook(c, p) })
		card(locales.StyleText(), func() { s.styleText(c, p) })
		card(locales.StyleShow(), func() { s.styleShow(c, p) })
	})
}

// styleLook is the background, the edge, the alignment and the font.
func (s *Settings) styleLook(c *ui.Context, p *prefs.Prefs) {
	kit.SettingRow(c, 0, locales.StyleLabel(), "", func() {
		// Text alone needs an edge to stay readable over a plain or bright picture.
		if kit.Segmented(c, &p.Style, locales.StyleBox(), locales.StyleImmersive()).Label(locales.StyleLabel()).Changed() && p.Style == 1 && p.Edge == 0 {
			p.Edge = 1
		}
	})
	kit.SettingRow(c, 1, locales.EdgeLabel(), "", func() {
		kit.Segmented(c, &p.Edge, locales.EdgeNone(), locales.EdgeOutline(), locales.EdgeShadow()).Label(locales.EdgeLabel())
	})
	kit.SettingRow(c, 2, locales.AlignLabel(), "", func() {
		kit.Segmented(c, &p.Align, locales.AlignLeft(), locales.AlignCenter()).Label(locales.AlignLabel())
	})
	kit.SettingRow(c, 3, locales.FontLabel(), "", func() {
		kit.Picker(c, shared.FontName(p.Font), kit.Choices(p.Font, prefs.Fonts, shared.FontName, func(f string) { s.core.UpdatePrefs(func(p *prefs.Prefs) { p.Font = f }) })).Label(locales.FontLabel())
	})
}

// styleText is the sizes, which comes first, and the colors: a scheme, or colors of one's own.
func (s *Settings) styleText(c *ui.Context, p *prefs.Prefs) {
	size := func(i int, label string, v *float64, lo, hi float64) {
		kit.SettingRow(c, i, label, "", func() {
			ui.Slider(c, v, lo, hi).Step(1).Width(kit.Space(30)).Label(label)
			ui.Text(c, strconv.Itoa(int(*v))).FontSize(kit.TextControl).FontWeight(kit.WeightBold).FontFeatures(kit.Tabular).
				TextColor(kit.Colors(c).InkMuted).MinWidth(kit.Space(6)).TextAlign(ui.End)
		})
	}
	size(0, locales.SrcSize(), &p.SrcSize, 14, 36)
	size(1, locales.TgtSize(), &p.TgtSize, 18, 48)
	i := 2
	if p.Display == 0 { // with the translation only, there is no order
		kit.SettingRow(c, i, locales.OrderLabel(), "", func() {
			order := 0
			if p.TgtFirst {
				order = 1
			}
			if kit.Segmented(c, &order, locales.OrderSrcFirst(), locales.OrderTgtFirst()).Label(locales.OrderLabel()).Changed() {
				p.TgtFirst = order == 1
			}
		})
		i++
	}
	custom := s.customColors || schemeOf(p) == ""
	kit.SettingBlock(c, i, locales.ColorScheme(), func() {
		ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Wrap().Children(func() {
			for _, sc := range subtitle.Schemes {
				if swatch(c.Key(sc.ID), schemeName(sc.ID), ui.Hex(sc.Edge), ui.Hex(sc.Src), ui.Hex(sc.Tgt), !custom && schemeOf(p) == sc.ID).Clicked() {
					p.SrcColor, p.TgtColor, p.EdgeColor = sc.Src, sc.Tgt, sc.Edge
					s.customColors = false
				}
			}
			if kit.CustomSwatch(c.Key("custom"), locales.SchemeCustom(), custom).Clicked() {
				s.customColors = true
			}
		})
	})
	if !custom {
		return
	}
	colors := func(i int, label string, v *string, from []string) {
		kit.SettingBlock(c, i, label, func() { kit.Swatches(c, v, from) })
	}
	colors(i+1, locales.SrcColor(), &p.SrcColor, subtitle.TextColors)
	colors(i+2, locales.TgtColor(), &p.TgtColor, subtitle.TextColors)
	if p.Edge != 0 || p.Style == 0 {
		colors(i+3, locales.EdgeColor(), &p.EdgeColor, subtitle.EdgeColors)
	}
}

// styleShow is what shows, how many sentences, when they go, and whether recordings show them.
func (s *Settings) styleShow(c *ui.Context, p *prefs.Prefs) {
	kit.SettingRow(c, 0, locales.DisplayLabel(), "", func() {
		kit.Segmented(c, &p.Display, locales.DisplayBoth(), locales.DisplayTgt()).Label(locales.DisplayLabel())
	})
	kit.SettingRow(c, 1, locales.ShownLabel(), "", func() {
		kit.Stepper(c, &p.Shown, 1, prefs.MaxShown, locales.ShownFewer(), locales.ShownMore())
	})
	kit.SettingRow(c, 2, locales.HideAfterLabel(), "", func() {
		hide := !p.KeepShown
		if kit.Switch(c, &hide, locales.HideAfterLabel()).Changed() {
			p.KeepShown = !hide
		}
	})
	i := 3
	if !p.KeepShown {
		kit.SettingRow(c, i, locales.HideWait(), "", func() {
			kit.Stepper(c, &p.HideAfter, 2, 10, locales.HideWaitLess(), locales.HideWaitMore())
		})
		i++
	}
	kit.SettingRow(c, i, locales.HideFromCapture(), "", func() {
		if kit.Switch(c, &p.HideFromCapture, locales.HideFromCapture()).Changed() {
			s.nav.SetHideFromCapture(p.HideFromCapture)
		}
	})
}

// swatch is a color scheme's button: its text colors on its edge's, as outlined text reads, named
// in its tooltip; ringed in the lantern's amber when chosen.
func swatch(c *ui.Context, name string, ground, src, tgt ui.Color, on bool) ui.Element {
	pal := kit.Colors(c)
	const d = 40
	b := ui.ButtonBase(c).Selected(on).Size(d, d).Radius(kit.Pill(d)).Background(ground).Label(name).Tooltip(name).
		Column().Justify(ui.Center).AlignItems(ui.Center).Cursor(ui.CursorPointer)
	if on {
		b = b.Border(2, pal.Lantern)
	} else {
		b = b.Border(1, pal.LineStrong)
	}
	return b.Children(func() {
		ui.Text(c, "Aa").FontSize(kit.TextMicro).FontWeight(kit.WeightBold).TextColor(src)
		ui.Text(c, "Aa").FontSize(kit.TextControl).FontWeight(kit.WeightHeavy).TextColor(tgt)
	})
}

// schemeOf is the ID of the scheme the colors are, "" when they are none.
func schemeOf(p *prefs.Prefs) string {
	for _, sc := range subtitle.Schemes {
		if strings.EqualFold(p.SrcColor, sc.Src) && strings.EqualFold(p.TgtColor, sc.Tgt) && strings.EqualFold(p.EdgeColor, sc.Edge) {
			return sc.ID
		}
	}
	return ""
}

func schemeName(id string) string {
	switch id {
	case "classic":
		return locales.SchemeClassic()
	case "ice":
		return locales.SchemeIce()
	case "amber":
		return locales.SchemeAmber()
	case "sakura":
		return locales.SchemeSakura()
	case "mint":
		return locales.SchemeMint()
	case "focus":
		return locales.SchemeFocus()
	}
	return id
}
