package settings

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// about is the app: its icon, name and what it does; its version and checking for updates, the
// guide again, the logs and feedback, a problem or an idea, as rows of a card; its license, the works
// it is built on, with links to them, and the licenses of all.
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
		kit.SettingRow(c, 3, locales.Feedback(), locales.FeedbackHint(), func() {
			if kit.Button(c, kit.Plain, kit.Small, nil, locales.FeedbackOpen()).Clicked() {
				s.feedback.open = true
			}
		})
	})
	s.feedbackDialog(c)
	kit.SandCard(c).Children(func() {
		ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Children(func() {
			kit.Linked(c, locales.AboutLicense(), "GPL-3.0", core.Repo+"/blob/main/LICENSE", kit.TextBody, p.Ink).
				FontWeight(kit.WeightMedium).Grow(1)
			if dir := s.core.LicensesDir(); dir != "" {
				if kit.Button(c, kit.Plain, kit.Small, nil, locales.AboutLicenses()).Clicked() {
					mygo.Shell.OpenPath(dir)
				}
			}
		})
		// A row each, its line across the card: what it does, its name and its GitHub account sharing
		// the room, its license at the end.
		ui.Grid(c).ColumnTracks(ui.FitContent(), ui.Fr(1), ui.Fr(1), ui.FitContent()).Children(func() {
			for i, w := range credits() {
				cell := func(body func()) ui.Element {
					e := ui.Row(c).AlignItems(ui.Center).Padding(kit.Space(2), kit.Space(6), kit.Space(2), 0)
					if i > 0 {
						e = e.BorderWidth(1, 0, 0).BorderColor(p.Line)
					}
					return e.Children(body)
				}
				text := func(s string) func() {
					return func() { ui.Text(c, s).FontSize(kit.TextBody).TextColor(p.InkMuted) }
				}
				cell(text(w.role))
				cell(func() {
					ui.Link(c, w.name, w.url).FontSize(kit.TextBody).FontWeight(kit.WeightMedium).TextColor(p.LanternText)
				})
				cell(func() {
					ui.Link(c, w.owner, "https://github.com/"+w.owner).FontSize(kit.TextBody).TextColor(p.LanternText)
				})
				cell(text(w.license)).Padding(kit.Space(2), 0).Justify(ui.End)
			}
		})
	})
}

// credit is a work the app is built on: what it does in the app, its name and home, the GitHub
// account it is under and its license.
type credit struct{ role, name, url, owner, license string }

// credits are the works the app is built on that matter most to it, the licenses of all in the
// directory the app ships them in.
func credits() []credit {
	return []credit{
		{locales.CreditTranslation(), "Index-Translate", core.CloudRepo, "bilibili", "Apache-2.0"},
		{locales.CreditLocalModels(), "llama.cpp", "https://github.com/ggml-org/llama.cpp", "ggml-org", "MIT"},
		{locales.CreditRecognition(), "sherpa-onnx", "https://github.com/k2-fsa/sherpa-onnx", "k2-fsa", "Apache-2.0"},
		{locales.CreditVoice(), "Silero VAD", "https://github.com/snakers4/silero-vad", "snakers4", "MIT"},
		{locales.CreditUI(), "MyGo", "https://github.com/egoist/mygo", "egoist", "MIT"},
		{locales.CreditIcons(), "Lucide", "https://github.com/lucide-icons/lucide", "lucide-icons", "ISC"},
	}
}
