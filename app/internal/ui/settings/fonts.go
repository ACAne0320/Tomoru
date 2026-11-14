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

// fontEdit is the list of the system's fonts, while it is open: what is typed to find one, as the
// list was last scrolled for, and the families as they were when it opened.
type fontEdit struct {
	open            bool
	query, scrolled string
	families        []string
	list            ui.ListState
}

// subtitleFont is the font the subtitles are in, as the picker shows it: font, or the system's
// for a family not installed, which then draws them.
func subtitleFont(font string) string {
	if slices.Contains(prefs.Fonts, font) || slices.ContainsFunc(ui.FontFamilies(), func(f ui.FontFamily) bool { return f.Name == font }) {
		return font
	}
	return prefs.Fonts[0]
}

// fontMenu is the font picker's menu: the presets and a family chosen from the list, then More
// fonts…, which opens it.
func (s *Settings) fontMenu(font string) func(m *ui.Menu) {
	fonts := prefs.Fonts
	if !slices.Contains(fonts, font) {
		fonts = append(slices.Clone(fonts), font)
	}
	choose := func(f string) { s.core.UpdatePrefs(func(p *prefs.Prefs) { p.Font = f }) }
	return func(m *ui.Menu) {
		kit.Choices(font, fonts, shared.FontName, choose)(m)
		m.Separator()
		if m.Item(locales.FontMore() + "…").Chosen() {
			s.openFonts()
		}
	}
}

// openFonts opens the list of the system's fonts at the one in use.
func (s *Settings) openFonts() {
	var names []string
	for _, f := range ui.FontFamilies() {
		if !f.Registered {
			names = append(names, f.Name)
		}
	}
	s.fonts = fontEdit{open: true, families: names}
	if i := slices.Index(names, s.core.Preferences().Font); i >= 0 {
		s.fonts.list.ScrollTo(i, ui.Center)
	}
}

// fontDialog lists the system's fonts, found by a part of their names: each a line in it, bold in
// the language on top of the subtitles, as they draw it, so a font without that language's script
// shows the system's it falls back to. A click uses one and closes the list.
func (s *Settings) fontDialog(c *ui.Context) {
	d := &s.fonts
	if !d.open {
		return
	}
	p := kit.Colors(c)
	pr := s.core.Preferences()
	// The boss coming: Latin among the language's own script.
	line := subtitle.Sample(pr, 8)
	sample, lang := line.Src, pr.Src
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
	if d.query != d.scrolled {
		d.scrolled = d.query
		d.list.ScrollTo(0, ui.Start)
	}
	kit.Dialog(c, &d.open, locales.FontLabel(), "", func() {
		kit.TextField(c, &d.query).Placeholder(locales.FontSearch()).Label(locales.FontSearch()).AutoFocus().Shrink(0)
		d.list.Key = func(i int) any { return shown[i] }
		d.list.Label = func(i int) string { return shown[i] }
		ui.List(c, &d.list, len(shown), func(i int) {
			if fontRow(c, shown[i], sample, lang, shown[i] == pr.Font).Clicked() {
				f := shown[i]
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

// fontRow is a family of the font list: sample in it, its name under it, and the mark of the one
// in use. It returns the part that chooses it.
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
			// A line as tall in every font, so that the rows are.
			ui.Text(c, sample).Font(name).Lang(lang).FontSize(kit.TextTitle).LineHeight(1.5).Bold().TextColor(p.Ink).MaxLines(1)
			kit.OneLine(c, name, kit.TextCaption, 0, p.InkMuted)
		})
		if on {
			kit.InUse(c)
		}
	})
}
