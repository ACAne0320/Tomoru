package settings

import (
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

type feedbackEdit struct {
	open, shown bool
	kind        int
	what        string
	copied      time.Time
}

func (s *Settings) feedbackDialog(c *ui.Context) {
	d, k := &s.feedback, s.core
	if !d.open {
		return
	}
	p := kit.Colors(c)
	kind := core.FeedbackKind(d.kind)
	intro, example := locales.FeedbackIntroProblem(), locales.FeedbackPlaceholderProblem()
	if kind == core.FeedbackIdea {
		intro, example = locales.FeedbackIntroIdea(), locales.FeedbackPlaceholderIdea()
	}
	kit.Dialog(c, &d.open, locales.Feedback(), intro, func() {
		kit.Segmented(c, &d.kind, locales.FeedbackProblem(), locales.FeedbackIdea()).AlignSelf(ui.Start).Shrink(0).Label(locales.FeedbackKind())
		kit.TextBox(c, &d.what).Placeholder(example).Label(locales.Feedback()).Height(96).MinHeight(kit.Space(16)).AutoFocus()
		ui.Column(c).Gap(kit.Space(2)).Shrink(0).Children(func() {
			app, system := k.Versions()
			pr := k.Preferences()
			engines := engineName(k, core.ASR, pr.ASR) + " → " + engineName(k, core.MT, pr.MT)
			if !pr.Translates() {
				engines = engineName(k, core.ASR, pr.ASR) + " · " + locales.DisplaySrc()
			}
			feedbackFacts(c, locales.FeedbackSummary(), []kit.Fact{
				{Label: locales.FeedbackVersion(), Value: "Tomoru " + app},
				{Label: locales.FeedbackSystem(), Value: system},
				{Label: locales.FeedbackEngines(), Value: engines},
				{Label: locales.FeedbackLangs(), Value: shared.LangsOf(pr)},
			})
			kit.ResultMark(c, assets.Lock, locales.FeedbackPrivate(), true)
		})
		ui.Column(c).Gap(kit.Space(2)).MinHeight(0).Children(func() {
			if kit.Disclosure(c, &d.shown, locales.FeedbackShow()) {
				ui.Scroll(c).MaxHeight(200).MinHeight(kit.Space(16)).Padding(kit.Space(3)).Radius(kit.RadiusField).Border(1, p.Line).Children(func() {
					kit.Caption(c, k.Feedback(kind, d.what)).Selectable()
				})
			}
		})
		kit.DialogActions(c, func() {
			if kit.Button(c, kit.Plain, kit.Small, assets.External, locales.FeedbackOnGitHub()).Clicked() {
				c.OpenURL(k.IssueURL(kind, d.what))
			}
			ui.Spacer(c)
			label, icon := locales.FeedbackCopy(), assets.Copy
			if left := d.copied.Add(1500 * time.Millisecond).Sub(c.Now()); left > 0 {
				label, icon = locales.Copied(), assets.Check
				c.After(left)
			}
			if kit.Button(c, kit.Primary, kit.Small, icon, label).Clicked() {
				c.Services().WriteClipboard(k.Feedback(kind, d.what))
				d.copied = c.Now()
			}
		}).Margin(kit.Space(2), 0, 0).Shrink(0)
	})
}

func feedbackFacts(c *ui.Context, title string, facts []kit.Fact) ui.Element {
	p := kit.Colors(c)
	return ui.Column(c).Gap(kit.Space(2)).Padding(kit.Space(3), kit.Space(4)).Radius(kit.RadiusField).Background(p.Sand).Children(func() {
		ui.Text(c, title).FontSize(kit.TextCaption).FontWeight(kit.WeightBold).TextColor(p.Ink)
		for i, f := range facts {
			ui.Row(c.Key(i)).Gap(kit.Space(3)).AlignItems(ui.Start).Children(func() {
				ui.Text(c, f.Label).FontSize(kit.TextCaption).TextColor(p.InkMuted).Width(kit.Space(20)).Shrink(0)
				ui.Text(c, f.Value).FontSize(kit.TextCaption).FontWeight(kit.WeightMedium).TextColor(p.Ink).Grow(1).MinWidth(0).MaxLines(3)
			})
		}
	})
}

func engineName(k *core.Core, kind core.Kind, id string) string {
	e, ok := k.Engine(kind, id)
	if !ok {
		e.ID = id
	}
	if e.Endpoint != nil && e.Endpoint.Model != "" {
		return e.Endpoint.Model
	}
	return shared.EngineName(e)
}
