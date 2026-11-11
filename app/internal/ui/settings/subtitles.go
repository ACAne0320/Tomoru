package settings

import (
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
	custom := s.customColors || schemeOf(p) == ""
	// The two cards side by side, as wide as each other, their controls in one column: they wrap
	// before a name would be cut.
	basis := kit.ControlCardWidth(c, locales.StyleLabel(), locales.EdgeLabel(), locales.AlignLabel(), locales.FontLabel(),
		locales.SrcSize(), locales.TgtSize(), locales.OrderLabel(), locales.ColorScheme())
	kit.Groups(c, func() {
		kit.Group(c, locales.StyleLook(), nil, kit.RowsCard, func() { s.styleLook(c, p) }).Grow(1).Basis(basis)
		kit.Group(c, locales.StyleText(), nil, kit.RowsCard, func() { s.styleText(c, p, custom) }).Grow(1).Basis(basis)
	})
	if custom {
		kit.Group(c, locales.SchemeCustom(), nil, kit.RowsCard, func() { styleColors(c, p) })
	}
	kit.Group(c, locales.StyleShow(), nil, kit.RowsCard, func() { s.styleShow(c, p) })
}

// styleLook is the background, the edge, the alignment and the font.
func (s *Settings) styleLook(c *ui.Context, p *prefs.Prefs) {
	kit.SettingRow(c, 0, locales.StyleLabel(), "", func() {
		kit.Control(c, func() {
			// Text alone needs an edge to stay readable over a plain or bright picture.
			if kit.Segmented(c, &p.Style, locales.StyleBox(), locales.StyleImmersive()).Grow(1).Label(locales.StyleLabel()).Changed() && p.Style == 1 && p.Edge == 0 {
				p.Edge = 1
			}
		})
	})
	kit.SettingRow(c, 1, locales.EdgeLabel(), "", func() {
		kit.Control(c, func() {
			kit.Segmented(c, &p.Edge, locales.EdgeNone(), locales.EdgeOutline(), locales.EdgeShadow()).Grow(1).Label(locales.EdgeLabel())
		})
	})
	kit.SettingRow(c, 2, locales.AlignLabel(), "", func() {
		kit.Control(c, func() {
			kit.Segmented(c, &p.Align, locales.AlignLeft(), locales.AlignCenter()).Grow(1).Label(locales.AlignLabel())
		})
	})
	kit.SettingRow(c, 3, locales.FontLabel(), "", func() {
		kit.Control(c, func() {
			kit.Picker(c, shared.FontName(p.Font), kit.Choices(p.Font, prefs.Fonts, shared.FontName, func(f string) { s.core.UpdatePrefs(func(p *prefs.Prefs) { p.Font = f }) })).
				Grow(1).Shrink(1).MinWidth(0).Label(locales.FontLabel())
		})
	})
}

// styleText is the sizes and the order of the two lines, and their colors: a scheme, or with
// custom, colors of one's own, which styleColors sets under the cards.
func (s *Settings) styleText(c *ui.Context, p *prefs.Prefs, custom bool) {
	// With one of the two only, the other's size and the order do nothing: they stay, greyed.
	tgtOnly, srcOnly := p.Display == prefs.DisplayTgt, p.Display == prefs.DisplaySrc
	size := func(i int, label string, off bool, v *float64, lo, hi float64) {
		kit.SettingRowOff(c, i, label, "", off, func() {
			kit.Control(c, func() { kit.Slider(c, v, lo, hi, label).Grow(1) })
		})
	}
	size(0, locales.SrcSize(), tgtOnly, &p.SrcSize, 14, 36)
	size(1, locales.TgtSize(), srcOnly, &p.TgtSize, 18, 48)
	kit.SettingRowOff(c, 2, locales.OrderLabel(), "", tgtOnly || srcOnly, func() {
		kit.Control(c, func() {
			order := 0
			if p.TgtFirst {
				order = 1
			}
			if kit.Segmented(c, &order, locales.OrderSrcFirst(), locales.OrderTgtFirst()).Grow(1).Label(locales.OrderLabel()).Changed() {
				p.TgtFirst = order == 1
			}
		})
	})
	kit.SettingRow(c, 3, locales.ColorScheme(), "", func() {
		kit.Control(c, func() {
			for _, sc := range subtitle.Schemes {
				if kit.SchemeSwatch(c.Key(sc.ID), schemeName(sc.ID), ui.Hex(sc.Edge), ui.Hex(sc.Src), ui.Hex(sc.Tgt), !custom && schemeOf(p) == sc.ID).Clicked() {
					p.SrcColor, p.TgtColor, p.EdgeColor = sc.Src, sc.Tgt, sc.Edge
					s.customColors = false
				}
			}
			if kit.CustomSwatch(c.Key("custom"), locales.SchemeCustom(), custom).Clicked() {
				s.customColors = true
			}
		}).Justify(ui.SpaceBetween)
	})
}

// styleColors is colors of one's own: the original's, the translation's and, when there is an
// edge or a box, the edge's, each a color of the list or any from the system's picker.
func styleColors(c *ui.Context, p *prefs.Prefs) {
	colors := func(i int, label string, v *string, from []string) {
		kit.SettingRow(c, i, label, "", func() { kit.Swatches(c, v, from) })
	}
	colors(0, locales.SrcColor(), &p.SrcColor, subtitle.TextColors)
	colors(1, locales.TgtColor(), &p.TgtColor, subtitle.TextColors)
	if p.Edge != 0 || p.Style == 0 {
		colors(2, locales.EdgeColor(), &p.EdgeColor, subtitle.EdgeColors)
	}
}

// styleShow is what shows, how many sentences, when they go, and whether recordings show them.
func (s *Settings) styleShow(c *ui.Context, p *prefs.Prefs) {
	kit.SettingRow(c, 0, locales.DisplayLabel(), "", func() {
		kit.Segmented(c, &p.Display, locales.DisplayBoth(), locales.DisplayTgt(), locales.DisplaySrc()).Label(locales.DisplayLabel())
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
