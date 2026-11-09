package settings

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// about is the app: its icon, name and what it does; its version and checking for updates, the
// guide again, the logs and reporting a problem, as rows of a card; its license and what it is
// made with.
func (s *Settings) about(c *ui.Context) {
	p := kit.Colors(c)
	ui.Row(c).Gap(kit.Space(5)).AlignItems(ui.Center).Children(func() {
		ui.Image(c, assets.AppIcon).Size(104, 104).Margin(-8)
		ui.Column(c).Gap(kit.Space(1)).Children(func() {
			ui.Text(c, "Tomoru").FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink)
			ui.Text(c, locales.Tagline()).FontSize(kit.TextBody).TextColor(p.InkMuted)
		})
	})
	kit.RowsCard(c).Children(func() {
		kit.SettingRow(c, 0, locales.AboutVersion(mygo.App.Version()), "", func() {
			if kit.Button(c, kit.Plain, kit.Small, nil, locales.CheckForUpdates()).Clicked() {
				updater.CheckForUpdates()
			}
		})
		kit.SettingRow(c, 1, locales.AboutGuide(), "", func() {
			if kit.Button(c, kit.Plain, kit.Small, nil, locales.AboutRerun()).Clicked() {
				// As on a first launch: until it is done, it is kept where it was left, to come back to
				// it after a quit, or the reopening a permission asks for.
				s.core.UpdatePrefs(func(p *prefs.Prefs) { p.Onboarded, p.GuideStep = false, 0 })
				s.nav.ShowGuide()
			}
		})
		if path := s.core.LogPath(); path != "" {
			kit.SettingRow(c, 2, locales.AboutLogs(), "", func() {
				if kit.Button(c, kit.Plain, kit.Small, nil, shared.OnWindows(locales.ShowInFinder(), locales.ShowInExplorer())).Clicked() {
					mygo.Shell.ShowItemInFolder(path)
				}
			})
		}
		kit.SettingRow(c, 3, locales.ReportProblem(), locales.ReportHint(), func() {
			if kit.Button(c, kit.Plain, kit.Small, nil, locales.ReportOpen()).Clicked() {
				s.feedback.open = true
			}
		})
	})
	s.feedbackDialog(c)
	kit.SandCard(c).Gap(kit.Space(2)).Children(func() {
		ui.Text(c, locales.AboutLicense()).FontSize(kit.TextBody).FontWeight(kit.WeightMedium).TextColor(p.Ink)
		kit.Caption(c, locales.AboutCredits())
	})
}
