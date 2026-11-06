package settings

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/proxy"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// appearances are the choices of Appearance, as prefs keeps them.
var appearances = []string{"", "light", "dark"}

// general is Preferences: how the app looks and the language it shows; what it does at launch, and
// its shortcut. What it hears and the languages it translates are the home page's.
func (s *Settings) general(c *ui.Context) {
	k := s.core
	before := k.Preferences()
	draft := before.Clone()
	defer func() { k.ApplyPreferences(before, draft) }()
	p := kit.Colors(c)
	ui.Text(c, locales.TabGeneral()).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink)
	kit.Groups(c, func() {
		kit.Group(c, locales.PrefsLook(), nil, kit.RowsCard, func() {
			kit.SettingRow(c, 0, locales.Appearance(), "", func() {
				at := 0
				for i, a := range appearances {
					if a == draft.Appearance {
						at = i
					}
				}
				if kit.Segmented(c, &at, locales.AppearanceSystem(), locales.AppearanceLight(), locales.AppearanceDark()).Label(locales.Appearance()).Changed() {
					draft.Appearance = appearances[at]
					s.nav.SetAppearance(draft.Appearance)
				}
			})
			kit.SettingRow(c, 1, locales.UiLanguage(), "", func() { s.uiLanguage(c) })
		}).Grow(1).Basis(360)
		kit.Group(c, locales.PrefsStart(), nil, kit.RowsCard, func() {
			kit.SettingRow(c, 0, locales.AutoStart(), "", func() { kit.Switch(c, &draft.AutoStart, locales.AutoStart()) })
			kit.SettingRow(c, 1, locales.OpenAtLogin(), k.LoginErr, func() {
				login := k.AtLogin
				if kit.Switch(c, &login, locales.OpenAtLogin()).Changed() {
					k.SetOpenAtLogin(login)
				}
			})
			kit.SettingRow(c, 2, locales.Shortcut(), "", func() {
				kit.Control(c, func() { shared.ShortcutField(c, k, s.nav, core.StartStop, locales.Shortcut(), &s.keys) })
			})
			kit.SettingRow(c, 3, locales.ImageShortcut(), "", func() {
				kit.Control(c, func() { shared.ShortcutField(c, k, s.nav, core.PictureShortcut, locales.ImageShortcut(), &s.imageKeys) })
			})
		}).Grow(1).Basis(360)
		s.network(c, &draft)
	})
}

// network is how the app reaches the cloud: through a proxy, the user's or else the system's, or
// directly.
func (s *Settings) network(c *ui.Context, draft *prefs.Prefs) {
	k := s.core
	k.RefreshSystemProxy()
	c.After(3 * time.Second) // the system's settings change
	kit.Group(c, locales.PrefsNetwork(), nil, kit.RowsCard, func() {
		kit.SettingRow(c, 0, locales.UseProxy(), locales.UseProxyDesc(), func() {
			on := !draft.NoProxy
			if kit.Switch(c, &on, locales.UseProxy()).Changed() {
				draft.NoProxy = !on
			}
		})
		if draft.NoProxy {
			return
		}
		desc, placeholder := locales.ProxySystemNone(), "127.0.0.1:7890"
		if k.SystemProxy != "" {
			desc, placeholder = locales.ProxySystem(k.SystemProxy), k.SystemProxy
		}
		if _, err := proxy.Parse(draft.ProxyURL); err != nil {
			desc = locales.ProxyInvalid()
		}
		kit.SettingRow(c, 1, locales.ProxyAddress(), desc, func() {
			kit.TextField(c, &draft.ProxyURL).Placeholder(placeholder).Width(kit.Space(52)).Label(locales.ProxyAddress())
		})
	}).Grow(1).Basis(360)
}

// uiLanguage chooses the language the app shows: the system's, or one it is written in, each
// named in itself.
func (s *Settings) uiLanguage(c *ui.Context) {
	p := s.core.Preferences()
	options := append([]string{""}, mygo.App.Languages()...)
	name := func(tag string) string {
		if tag == "" {
			return locales.UiLanguageSystem()
		}
		return i18n.Autonym(tag)
	}
	kit.Picker(c, name(p.UILang), kit.Choices(p.UILang, options, name, func(tag string) {
		s.core.UpdatePrefs(func(p *prefs.Prefs) { p.UILang = tag })
		mygo.App.SetLocale(tag)
	})).Label(locales.UiLanguage())
}
