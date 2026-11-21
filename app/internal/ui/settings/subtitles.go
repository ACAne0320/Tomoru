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
	basis := kit.ControlCardWidth(c, locales.StyleLabel(), locales.BoxOpacity(), locales.EdgeLabel(), locales.AlignLabel(), locales.FontLabel(),
		locales.SrcSize(), locales.TgtSize(), locales.TextOpacity(), locales.OrderLabel(), locales.ColorScheme())
	kit.Groups(c, func() {
		kit.Group(c, locales.StyleLook(), nil, kit.RowsCard, func() { s.styleLook(c, p) }).Grow(1).Basis(basis)
		kit.Group(c, locales.StyleText(), nil, kit.RowsCard, func() { s.styleText(c, p, custom) }).Grow(1).Basis(basis)
	})
	if custom {
		kit.Group(c, locales.SchemeCustom(), nil, kit.RowsCard, func() { styleColors(c, p) })
	}
	kit.Group(c, locales.StyleShow(), nil, kit.RowsCard, func() { s.styleShow(c, p) })
	s.fontDialog(c)
}

func (s *Settings) styleLook(c *ui.Context, p *prefs.Prefs) {
	kit.SettingRow(c, 0, locales.StyleLabel(), "", func() {
		kit.Control(c, func() {
			if kit.Segmented(c, (*int)(&p.Style), locales.StyleBox(), locales.StyleImmersive()).Grow(1).Label(locales.StyleLabel()).Changed() && p.Style == prefs.StyleImmersive && p.Edge == prefs.EdgeNone {
				p.Edge = prefs.EdgeOutline
			}
		})
	})
	kit.SettingRowOff(c, 1, locales.BoxOpacity(), "", p.Style == prefs.StyleImmersive, func() {
		kit.Control(c, func() { kit.Slider(c, &p.BoxOpacity, 0, 100, "%", locales.BoxOpacity()).Grow(1) })
	})
	kit.SettingRow(c, 2, locales.EdgeLabel(), "", func() {
		kit.Control(c, func() {
			kit.Segmented(c, (*int)(&p.Edge), locales.EdgeNone(), locales.EdgeOutline(), locales.EdgeShadow()).Grow(1).Label(locales.EdgeLabel())
		})
	})
	kit.SettingRow(c, 3, locales.AlignLabel(), "", func() {
		kit.Control(c, func() {
			kit.Segmented(c, (*int)(&p.Align), locales.AlignLeft(), locales.AlignCenter()).Grow(1).Label(locales.AlignLabel())
		})
	})
	kit.SettingRow(c, 4, locales.FontLabel(), "", func() {
		kit.Control(c, func() {
			font := subtitleFont(p.Font)
			kit.Picker(c, shared.FontName(font), s.fontMenu(font)).Grow(1).Shrink(1).MinWidth(0).Label(locales.FontLabel())
		})
	})
}

func (s *Settings) styleText(c *ui.Context, p *prefs.Prefs, custom bool) {
	tgtOnly, srcOnly := p.Display == prefs.DisplayTgt, p.Display == prefs.DisplaySrc
	slider := func(i int, label string, off bool, v *float64, lo, hi float64, unit string) {
		kit.SettingRowOff(c, i, label, "", off, func() {
			kit.Control(c, func() { kit.Slider(c, v, lo, hi, unit, label).Grow(1) })
		})
	}
	slider(0, locales.SrcSize(), tgtOnly, &p.SrcSize, 14, 36, "")
	slider(1, locales.TgtSize(), srcOnly, &p.TgtSize, 18, 48, "")
	slider(2, locales.TextOpacity(), false, &p.TextOpacity, prefs.MinTextOpacity, 100, "%")
	kit.SettingRowOff(c, 3, locales.OrderLabel(), "", tgtOnly || srcOnly, func() {
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
	kit.SettingRow(c, 4, locales.ColorScheme(), "", func() {
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

func styleColors(c *ui.Context, p *prefs.Prefs) {
	colors := func(i int, label string, v *string, from []string) {
		kit.SettingRow(c, i, label, "", func() { kit.Swatches(c, v, from) })
	}
	colors(0, locales.SrcColor(), &p.SrcColor, subtitle.TextColors)
	colors(1, locales.TgtColor(), &p.TgtColor, subtitle.TextColors)
	if p.Edge != prefs.EdgeNone || p.Style == prefs.StyleBox {
		colors(2, locales.EdgeColor(), &p.EdgeColor, subtitle.EdgeColors)
	}
}

func (s *Settings) styleShow(c *ui.Context, p *prefs.Prefs) {
	kit.SettingRow(c, 0, locales.DisplayLabel(), "", func() {
		kit.Segmented(c, (*int)(&p.Display), locales.DisplayBoth(), locales.DisplayTgt(), locales.DisplaySrc()).Label(locales.DisplayLabel())
	})
	kit.SettingRow(c, 1, locales.ShownLabel(), "", func() {
		kit.Stepper(c, &p.Shown, 1, prefs.MaxShown, locales.ShownFewer(), locales.ShownMore())
	})
	kit.SettingRowOff(c, 2, locales.OlderOpacity(), "", p.Shown == 1, func() {
		kit.Control(c, func() {
			kit.Slider(c, &p.OlderOpacity, prefs.MinOlderOpacity, 100, "%", locales.OlderOpacity()).Grow(1)
		})
	})
	kit.SettingRow(c, 3, locales.HideAfterLabel(), "", func() {
		hide := !p.KeepShown
		if kit.Switch(c, &hide, locales.HideAfterLabel()).Changed() {
			p.KeepShown = !hide
		}
	})
	i := 4
	if !p.KeepShown {
		kit.SettingRow(c, i, locales.HideWait(), "", func() {
			kit.Stepper(c, &p.HideAfterSeconds, 2, 10, locales.HideWaitLess(), locales.HideWaitMore())
		})
		i++
	}
	if !shared.HidesFromCapture {
		return
	}
	kit.SettingRow(c, i, locales.HideFromCapture(), "", func() {
		if kit.Switch(c, &p.HideFromCapture, locales.HideFromCapture()).Changed() {
			s.nav.SetHideFromCapture(p.HideFromCapture)
		}
	})
}

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
