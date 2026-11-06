package settings

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

// about is the app: its icon, name, version and what it does; checking for updates, the guide
// again, and the logs; its license and what it is made with.
func (s *Settings) about(c *ui.Context) {
	p := kit.Colors(c)
	ui.Row(c).Gap(kit.Space(5)).AlignItems(ui.Center).Children(func() {
		ui.Image(c, assets.AppIcon).Size(104, 104).Margin(-8)
		ui.Column(c).Gap(kit.Space(1)).Children(func() {
			ui.Text(c, "Tomoru").FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink)
			ui.Text(c, locales.AboutVersion(mygo.App.Version())).FontSize(kit.TextControl).FontFeatures(kit.Tabular).TextColor(p.InkMuted)
			ui.Text(c, locales.Tagline()).FontSize(kit.TextBody).TextColor(p.Ink)
		})
	})
	ui.Row(c).Gap(kit.Space(2)).Wrap().Children(func() {
		if kit.Button(c, kit.Plain, kit.Medium, assets.Download, locales.CheckForUpdates()).Clicked() {
			updater.CheckForUpdates()
		}
		if kit.Button(c, kit.Plain, kit.Medium, assets.Play, locales.RerunGuide()).Clicked() {
			// As on a first launch: until it is done, it is kept where it was left, to come back to
			// it after a quit, or the reopening a permission asks for.
			s.core.UpdatePrefs(func(p *prefs.Prefs) { p.Onboarded, p.GuideStep = false, 0 })
			s.nav.ShowGuide()
		}
		if path := s.core.LogPath(); path != "" && kit.Button(c, kit.Plain, kit.Medium, assets.Arrow, locales.ShowLogs()).Clicked() {
			mygo.Shell.ShowItemInFolder(path)
		}
	})
	kit.SandCard(c).Gap(kit.Space(2)).Children(func() {
		ui.Text(c, locales.AboutLicense()).FontSize(kit.TextBody).FontWeight(kit.WeightMedium).TextColor(p.Ink)
		kit.Caption(c, locales.AboutCredits())
	})
}
