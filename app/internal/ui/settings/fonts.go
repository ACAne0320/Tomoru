package settings

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

type fontEdit struct {
	open     bool
	query    string
	families []string
	list     ui.ListState
}

func subtitleFont(font prefs.Font) prefs.Font {
	if font.Builtin() || slices.ContainsFunc(ui.FontFamilies(), func(f ui.FontFamily) bool { return f.Name == string(font) }) {
		return font
	}
	return prefs.FontSystem
}

func (s *Settings) fontMenu(font prefs.Font) func(m *ui.Menu) {
	fonts := slices.Clone(prefs.Fonts)
	if !font.Builtin() {
		fonts = append(fonts, font)
	}
	keys := make([]string, len(fonts))
	for i, f := range fonts {
		keys[i] = string(f)
	}
	name := func(key string) string { return shared.FontName(prefs.Font(key)) }
	choose := func(key string) { s.core.UpdatePrefs(func(p *prefs.Prefs) { p.Font = prefs.Font(key) }) }
	return func(m *ui.Menu) {
		kit.Choices(string(font), [][]string{keys}, name, choose)(m)
		m.Separator()
		if m.Item(locales.FontMore() + "…").Chosen() {
			s.openFonts()
		}
	}
}

func (s *Settings) openFonts() {
	var names []string
	for _, f := range ui.FontFamilies() {
		if !f.Registered {
			names = append(names, f.Name)
		}
	}
	s.fonts = fontEdit{open: true, families: names}
	if i := slices.Index(names, string(s.core.Preferences().Font)); i >= 0 {
		s.fonts.list.ScrollTo(i, ui.Center)
	}
}

func (s *Settings) fontDialog(c *ui.Context) {
	d := &s.fonts
	if !d.open {
		return
	}
	p := kit.Colors(c)
	pr := s.core.Preferences()
	line := subtitle.MixedSample(pr)
	sample, lang := line.Src, line.Lang
	if pr.Display == prefs.DisplayTgt || pr.Display == prefs.DisplayBoth && pr.TgtFirst {
		sample, lang = line.Tgt, pr.Tgt
	}
	q := strings.ToLower(strings.TrimSpace(d.query))
	var shown []string
	for _, f := range d.families {
		if strings.Contains(strings.ToLower(f), q) {
			shown = append(shown, f)
		}
	}
	kit.Dialog(c, &d.open, locales.FontLabel(), "", func() {
		if kit.TextField(c, &d.query).Placeholder(locales.FontSearch()).Label(locales.FontSearch()).AutoFocus().Shrink(0).Changed() {
			d.list.ScrollTo(0, ui.Start)
		}
		d.list.Key = func(i int) any { return shown[i] }
		d.list.Label = func(i int) string { return shown[i] }
		ui.List(c, &d.list, len(shown), func(i int) {
			if fontRow(c, shown[i], sample, lang, prefs.Font(shown[i]) == pr.Font).Clicked() {
				f := prefs.Font(shown[i])
				s.core.UpdatePrefs(func(p *prefs.Prefs) { p.Font = f })
				d.open = false
			}
		}).Height(400).MinHeight(kit.Space(30)).Shrink(1).Margin(0, -kit.Space(3)).Children(func() {
			if len(shown) == 0 {
				ui.Text(c, locales.FontNoMatch()).FontSize(kit.TextBody).TextColor(p.InkMuted).Padding(kit.Space(4))
			}
		})
	})
}

func fontRow(c *ui.Context, name, sample, lang string, on bool) ui.Element {
	p := kit.Colors(c)
	b := ui.ButtonBase(c).Padding(kit.Space(3)).Gap(kit.Space(3)).Justify(ui.Start).AlignItems(ui.Center).Radius(kit.RadiusTile).
		Role(ui.RoleRadio).Checked(on).Label(name).Cursor(ui.CursorPointer)
	face := p.Band.Alpha(0)
	switch {
	case on:
		face = p.Band
	case b.Hovered():
		face = p.Paper
	}
	return b.Background(b.AnimateColor("face", face, kit.Fade)).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(kit.Space(1)).Children(func() {
			ui.Text(c, sample).Font(name).Lang(lang).FontSize(kit.TextTitle).LineHeight(1.5).Bold().TextColor(p.Ink).MaxLines(1)
			kit.OneLine(c, name, kit.TextCaption, 0, p.InkMuted)
		})
		if on {
			kit.InUse(c)
		}
	})
}
