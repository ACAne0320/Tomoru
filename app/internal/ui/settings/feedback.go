package settings

import (
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

// feedbackEdit is reporting a problem: the user's words, kept until sent or cleared, whether the
// text copied shows, and when it was last copied.
type feedbackEdit struct {
	open, shown bool
	what        string
	copied      time.Time
}

// feedbackDialog asks what went wrong, and copies it with the app's setup and a guide, for an AI
// agent to make an issue of it with the user; or opens the issue form, started with the same.
func (s *Settings) feedbackDialog(c *ui.Context) {
	d, k := &s.feedback, s.core
	if !d.open {
		return
	}
	kit.Dialog(c, &d.open, locales.ReportProblem(), locales.ReportIntro(), func() {
		kit.TextBox(c, &d.what).Placeholder(locales.ReportPlaceholder()).Label(locales.ReportProblem()).Height(96).AutoFocus()
		version, system, engines := k.Setup()
		kit.Caption(c, locales.ReportIncluded("Tomoru "+version+" · "+system+" · "+engines))
		if kit.Disclosure(c, &d.shown, locales.ReportShow()) {
			ui.Scroll(c).MaxHeight(220).Children(func() {
				kit.Caption(c, k.Feedback(d.what)).Selectable()
			})
		}
		kit.DialogActions(c, func() {
			if kit.Button(c, kit.Plain, kit.Small, assets.External, locales.ReportOnGitHub()).Clicked() {
				c.OpenURL(k.IssueURL(d.what))
			}
			ui.Spacer(c)
			label, icon := locales.ReportCopy(), assets.Copy
			if left := d.copied.Add(1500 * time.Millisecond).Sub(c.Now()); left > 0 {
				label, icon = locales.Copied(), assets.Check
				c.After(left)
			}
			if kit.Button(c, kit.Primary, kit.Small, icon, label).Clicked() {
				c.Services().WriteClipboard(k.Feedback(d.what))
				d.copied = c.Now()
			}
		})
	})
}
