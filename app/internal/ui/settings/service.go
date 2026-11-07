package settings

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/locales"
)

// The form that sets up a cloud service in place on the engines page, of recognition, Qwen's or
// OpenAI's, or of translation: its model, its address and its API key.

// serviceEdit is the form's state while it shows.
type serviceEdit struct {
	open   bool
	url    string        // the service's own address
	models []string      // the service's models, the first its default
	a      prefs.Service // as edited
	key    string        // a new API key, or "" to keep the one kept
	model  int           // of models, for a service of models of its own
	test   string        // the key of its tests in Core.Tests
	more   bool          // its model and address show: the service's own do without them
}

// keyPages are where each service's API keys are made: its note under the key links there.
var keyPages = map[string]string{
	core.QwenCloud: "https://bailian.console.aliyun.com/cn-beijing/model/settings/api-key",
	core.OpenAI:    "https://platform.openai.com/api-keys",
}

// modelNames name, in its form, the models of a service of models of its own, as core.ServiceOf
// lists them; OpenAI's, a compatible service naming its own, are typed.
var modelNames = map[string][]string{
	core.QwenCloud: {"3.1", "3.0"},
}

// Open opens the form on the service id as k has it. n tells its tests from those of the forms
// before.
func (d *serviceEdit) Open(k *core.Core, id string, n int) {
	if d.open {
		k.ForgetTest(d.test)
	}
	url, models := core.ServiceOf(id)
	a := k.API(id)
	*d = serviceEdit{open: true, url: url, models: models, a: a, model: max(0, slices.Index(models, a.Model)), test: fmt.Sprintf("api-%d", n),
		more: a.URL != "" || a.Model != "" && a.Model != models[0]}
}

// serviceForm is the form of the service set up, or being set up: its API key first, all a
// service needs, then, folded unless they were changed, its model and address; Cancel only for
// one set up. Saving uses nothing: Use does.
func (s *Settings) serviceForm(c *ui.Context, d *serviceEdit, setUp bool) {
	k := s.core
	names := modelNames[d.a.ID]
	var keyNote, keyLink, hostNote string
	switch d.a.ID {
	case core.QwenCloud:
		keyNote, keyLink, hostNote = locales.AsrCloudIntro(), locales.AsrCloudConsole(), locales.AsrCloudHostDescription()
	case core.OpenAI:
		keyNote, keyLink, hostNote = locales.AsrOpenAiIntro(), locales.AsrOpenAiPlatform(), locales.AsrOpenAiHostDescription()
	}
	ui.Column(c).Gap(kit.Space(3)).Children(func() { // each under its label: the pane is narrow
		ui.Field(c, locales.EndpointKey(), func() {
			in := kit.TextField(c, &d.key).Password().Label(locales.EndpointKey())
			if d.a.Key {
				in.Placeholder(locales.EndpointKeyKeep())
			}
			// As a description is, in its ink, with its link: a field's description is text alone.
			kit.Linked(c, keyNote, keyLink, keyPages[d.a.ID], kit.TextCaption, kit.Colors(c).Faint)
		})
		if kit.Disclosure(c, &d.more, locales.ModelAndAddress()) {
			model := ui.Field(c, locales.EndpointModel(), func() {
				if names != nil {
					kit.Segmented(c, &d.model, names...).AlignSelf(ui.Start).Label(locales.EndpointModel())
				} else {
					// A compatible service names its own models.
					kit.TextField(c, &d.a.Model).Placeholder(d.models[0]).Label(locales.EndpointModel())
				}
			})
			if names == nil {
				model.Description(locales.AsrOpenAiModelDescription())
			}
			host := ui.Field(c, locales.AsrCloudHost(), func() {
				kit.TextField(c, &d.a.URL).Placeholder(d.url).Label(locales.AsrCloudHost())
			})
			if hostNote != "" {
				host.Description(hostNote)
			}
		}
	})
	if names != nil {
		d.a.Model = d.models[d.model]
	}
	valid := d.a.Key || strings.TrimSpace(d.key) != ""
	if r, ok := k.Tests[d.test]; ok {
		ms := fmt.Sprint(r.Took.Milliseconds())
		switch {
		case r.Running:
			kit.Waiting(c, locales.Testing())
		case r.Err != nil:
			kit.Result(c, false, locales.TestFailed(ms), shared.ErrorText(r.Err))
		default:
			kit.Result(c, true, locales.TestSucceeded(ms), "")
		}
	}
	ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
		ui.Spacer(c)
		if kit.Off(c, kit.Button(c, kit.Plain, kit.Small, nil, locales.Test()), !valid || k.Tests[d.test].Running).Clicked() {
			k.TestAPI(d.test, d.a, strings.TrimSpace(d.key))
		}
		if setUp && kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
			k.ForgetTest(d.test)
			*d = serviceEdit{}
			return
		}
		if kit.Off(c, kit.Button(c, kit.Primary, kit.Small, nil, locales.Save()), !valid).Clicked() {
			k.SaveAPI(d.a, strings.TrimSpace(d.key))
			c.Toast(locales.EndpointSaved(shared.EngineName(core.Engine{ID: d.a.ID})))
			k.ForgetTest(d.test)
			*d = serviceEdit{}
		}
	})
}
