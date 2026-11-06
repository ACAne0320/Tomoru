package settings

import (
	"errors"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/translate"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// The dialog that adds or edits a translation API.

// endpointEdit is the dialog's state while it shows.
type endpointEdit struct {
	open bool
	e    prefs.Endpoint // as edited; a new one has no ID
	was  prefs.Endpoint // as it was
	key  string         // a new API key, or "" to keep the one kept
	test string         // the key of its tests in Core.Tests
	// asking asks, in place of the dialog's buttons, before its changes are thrown away.
	asking bool
}

func (s *Settings) editEndpoint(e prefs.Endpoint) {
	s.edits++
	s.edit = endpointEdit{open: true, e: e, was: e, test: fmt.Sprintf("edit-%d", s.edits)}
}

// changed tells whether the endpoint was edited.
func (d *endpointEdit) changed() bool { return d.e != d.was || d.key != "" }

// endpointDialog is the dialog: the endpoint's form, how its test went, and its buttons.
func (s *Settings) endpointDialog(c *ui.Context) {
	k, d := s.core, &s.edit
	if !d.open {
		return
	}
	title := locales.EndpointEditTitle()
	if d.e.ID == "" {
		title = locales.AddEndpoint()
	}
	dialog := kit.Dialog(c, &d.open, title, locales.EndpointIntro(), func() {
		endpointForm(c, d)
		if r, ok := k.Tests[d.test]; ok {
			testResult(c, r)
		}
		if d.asking {
			kit.Discard(c, &d.open, &d.asking)
		} else {
			s.endpointActions(c)
		}
	})
	kit.Guard(dialog, &d.open, &d.asking, d.changed())
	if !d.open {
		k.ForgetTest(d.test)
	}
}

// endpointForm is the endpoint's fields.
func endpointForm(c *ui.Context, d *endpointEdit) {
	ui.Form(c, func() {
		ui.Field(c, locales.EndpointName(), func() {
			kit.TextField(c, &d.e.Name).Placeholder("DeepSeek").Label(locales.EndpointName()).AutoFocus()
		})
		ui.Field(c, locales.EndpointUrl(), func() {
			kit.TextField(c, &d.e.URL).Placeholder("https://api.deepseek.com").Label(locales.EndpointUrl())
		})
		ui.Field(c, locales.EndpointModel(), func() {
			kit.TextField(c, &d.e.Model).Placeholder("deepseek-flash").Label(locales.EndpointModel())
		})
		ui.Field(c, locales.EndpointKey(), func() {
			in := kit.TextField(c, &d.key).Password().Label(locales.EndpointKey())
			if d.e.ID != "" {
				in.Placeholder(locales.EndpointKeyKeep())
			}
		})
		ui.Field(c, locales.EndpointFormat(), func() {
			format := 0
			if d.e.Index {
				format = 1
			}
			if kit.Segmented(c, &format, locales.EndpointFormatChat(), "Index-Translate").AlignSelf(ui.Start).Label(locales.EndpointFormat()).Changed() {
				d.e.Index = format == 1
			}
		})
	})
}

// valid tells whether the endpoint has what translating with it needs: a name, an address and a
// model.
func (d *endpointEdit) valid() bool {
	return strings.TrimSpace(d.e.Name) != "" && strings.HasPrefix(strings.TrimSpace(d.e.URL), "http") && strings.TrimSpace(d.e.Model) != ""
}

// testResult is whether the test worked and how long it took, and under it the translation, or
// why not: an answer without a translation did not work either. Under the translation, whether it
// read the picture sent with it, when that is known.
func testResult(c *ui.Context, r core.TestResult) {
	ms := fmt.Sprint(r.Took.Milliseconds())
	switch {
	case r.Running:
		kit.Waiting(c, locales.Testing())
	case r.Err != nil || r.Text == "":
		why := locales.TestNoTranslation()
		switch {
		case errors.Is(r.Err, translate.ErrOnlyThinking):
			why = locales.TestOnlyThinking()
		case r.Err != nil:
			why = r.Err.Error()
		}
		kit.Result(c, false, locales.TestFailed(ms), why)
	default:
		kit.Result(c, true, locales.TestSucceeded(ms), r.Text)
		switch r.Vision {
		case prefs.VisionYes:
			kit.ResultMark(c, assets.Picture, locales.ReadsPictures(), true)
		case prefs.VisionNo:
			kit.ResultMark(c, assets.NoPicture, locales.ReadsNoPictures(), false)
		}
		if r.ToCloud {
			kit.Caption(c, locales.PicturesToCloud(shared.EngineName(core.Engine{ID: "cloud"})))
		}
	}
}

// endpointActions is the dialog's buttons: Delete for an endpoint kept, which can be undone, Test,
// Cancel and Save.
func (s *Settings) endpointActions(c *ui.Context) {
	k, d := s.core, &s.edit
	valid := d.valid()
	kit.DialogActions(c, func() {
		if d.e.ID != "" && kit.Button(c, kit.Destructive(kit.Quiet), kit.Small, assets.Trash, locales.Delete()).Clicked() {
			deleted := locales.EndpointDeleted(d.was.Name)
			if k.Preferences().MT == prefs.CustomPrefix+d.e.ID {
				deleted = locales.EndpointDeletedInUse(d.was.Name)
			}
			c.ToastAction(deleted, locales.Undo(), k.DeleteEndpoint(d.e.ID))
			d.open = false
		}
		ui.Spacer(c)
		if kit.Off(c, kit.Button(c, kit.Plain, kit.Small, nil, locales.Test()), !valid || k.Tests[d.test].Running).Clicked() {
			k.TestEndpoint(d.test, d.e, d.key)
		}
		if kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
			d.open = false
		}
		if kit.Off(c, kit.Button(c, kit.Primary, kit.Small, nil, locales.Save()), !valid).Clicked() {
			d.e.Name, d.e.URL, d.e.Model = strings.TrimSpace(d.e.Name), strings.TrimSpace(d.e.URL), strings.TrimSpace(d.e.Model)
			k.SaveEndpoint(d.e, d.key)
			c.Toast(locales.EndpointSaved(d.e.Name))
			d.open = false
		}
	})
}
