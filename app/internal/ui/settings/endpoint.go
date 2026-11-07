package settings

import (
	"cmp"
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

// The form that adds or edits a translation API: in place on the engines page, and in a dialog
// where learning names needs one added.

// endpointEdit is the form's state while it shows.
type endpointEdit struct {
	open bool
	e    prefs.Endpoint // as edited; a new one has no ID
	was  prefs.Endpoint // as it was
	key  string         // a new API key, or "" to keep the one kept
	test string         // the key of its tests in Core.Tests
	// asking asks, in place of the dialog's buttons, before its changes are thrown away.
	asking bool
}

// next numbers a form opened, to tell its tests from those of the forms before.
func (s *Settings) next() int {
	s.edits++
	return s.edits
}

// editAPI opens d on e.
func (s *Settings) editAPI(d *endpointEdit, e prefs.Endpoint) {
	if d.open {
		s.core.ForgetTest(d.test)
	}
	*d = endpointEdit{open: true, e: e, was: e, test: fmt.Sprintf("edit-%d", s.next())}
}

// closeAPI closes the engines page's form, its test forgotten.
func (s *Settings) closeAPI() {
	if s.api.open {
		s.core.ForgetTest(s.api.test)
	}
	s.api, s.addingAPI = endpointEdit{}, false
}

// changed tells whether the endpoint was edited.
func (d *endpointEdit) changed() bool { return d.e != d.was || d.key != "" }

// apiPane is the detail of translation's list while it adds an API: what APIs work, and the form.
func (s *Settings) apiPane(c *ui.Context) {
	ui.Text(c, locales.AddApi()).FontSize(kit.TextTitle).FontWeight(kit.WeightHeavy).TextColor(kit.Colors(c).Ink).MaxLines(1)
	kit.Caption(c, locales.EndpointIntro()).Margin(-kit.Space(2), 0, 0)
	s.apiForm(c, &s.api, func() { s.closeAPI() })
}

// apiForm is d's fields, how its test went, and its buttons; closed by close. Saved, a new API
// shows on the engines page.
func (s *Settings) apiForm(c *ui.Context, d *endpointEdit, close func()) {
	endpointForm(c, d)
	if r, ok := s.core.Tests[d.test]; ok {
		testResult(c, r)
	}
	ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() { s.endpointActions(c, d, close) })
}

// endpointDialog is the dialog learning names opens to add an API: the form, in a dialog.
func (s *Settings) endpointDialog(c *ui.Context) {
	k, d := s.core, &s.edit
	if !d.open {
		return
	}
	title := strings.TrimSuffix(locales.AddEndpoint(), "…") // the button's label: a title has no ellipsis
	dialog := kit.Dialog(c, &d.open, title, locales.EndpointIntro(), func() {
		endpointForm(c, d)
		if r, ok := k.Tests[d.test]; ok {
			testResult(c, r)
		}
		if d.asking {
			kit.Discard(c, &d.open, &d.asking)
		} else {
			kit.DialogActions(c, func() { s.endpointActions(c, d, func() { d.open = false }) })
		}
	})
	kit.Guard(dialog, &d.open, &d.asking, d.changed())
	if !d.open {
		k.ForgetTest(d.test)
	}
}

// endpointForm is the endpoint's fields, each under its label: the detail's pane is narrow.
func endpointForm(c *ui.Context, d *endpointEdit) {
	ui.Column(c).Gap(kit.Space(3)).Children(func() {
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
			// A menu: the formats' names are too long for a segmented control in the pane.
			ids := make([]string, len(translate.Formats))
			for i, f := range translate.Formats {
				ids[i] = f.ID
			}
			name := func(id string) string {
				f, _ := translate.FormatByID(id)
				return cmp.Or(f.Name, locales.EndpointFormatChat())
			}
			id := formatOf(d.e)
			kit.Picker(c, name(id), kit.Choices(id, ids, name, func(id string) { d.e.Format = id })).AlignSelf(ui.Start).Label(locales.EndpointFormat())
		})
	})
}

// formatOf is the ID of the format e is prompted in: the chat model's unless it names another.
func formatOf(e prefs.Endpoint) string {
	if f, ok := translate.FormatByID(e.Format); ok {
		return f.ID
	}
	return translate.Chat.ID
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

// deleteAPI deletes the endpoint e, its form closed if it is open, and says so with Undo.
func (s *Settings) deleteAPI(c *ui.Context, e prefs.Endpoint) {
	k := s.core
	deleted := locales.EndpointDeleted(e.Name)
	if k.Preferences().MT == prefs.CustomPrefix+e.ID {
		deleted = locales.EndpointDeletedInUse(e.Name)
	}
	if s.api.open && s.api.e.ID == e.ID {
		s.closeAPI()
	}
	c.ToastAction(deleted, locales.Undo(), k.DeleteEndpoint(e.ID))
}

// endpointActions is the form's buttons: Test, Cancel and Save, after which close closes it.
func (s *Settings) endpointActions(c *ui.Context, d *endpointEdit, close func()) {
	k := s.core
	valid := d.valid()
	ui.Spacer(c)
	if kit.Off(c, kit.Button(c, kit.Plain, kit.Small, nil, locales.Test()), !valid || k.Tests[d.test].Running).Clicked() {
		k.TestEndpoint(d.test, d.e, d.key)
	}
	if kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
		close()
		return
	}
	if kit.Off(c, kit.Button(c, kit.Primary, kit.Small, nil, locales.Save()), !valid).Clicked() {
		d.e.Name, d.e.URL, d.e.Model = strings.TrimSpace(d.e.Name), strings.TrimSpace(d.e.URL), strings.TrimSpace(d.e.Model)
		id := k.SaveEndpoint(d.e, d.key)
		c.Toast(locales.EndpointSaved(d.e.Name))
		close()
		s.shown[core.MT] = prefs.CustomPrefix + id
	}
}
