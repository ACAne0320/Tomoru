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

type serviceEdit struct {
	open   bool
	url    string
	models []string
	a      prefs.Service
	key    string
	model  int
	test   string
	more   bool
}

var keyPages = map[string]string{
	core.QwenCloud: "https://bailian.console.aliyun.com/cn-beijing/model/settings/api-key",
	core.OpenAI:    "https://platform.openai.com/api-keys",
	core.HyMTCloud: "https://console.cloud.tencent.com/tokenhub/apikey",
}

var modelNames = map[string][]string{
	core.QwenCloud: {"3.1", "3.0"},
	core.HyMTCloud: {"Plus", "Pro", "Lite"},
}

func (d *serviceEdit) Open(k *core.Core, id string, n int) {
	if d.open {
		k.ForgetTest(d.test)
	}
	url, models := core.ServiceOf(id)
	a := k.API(id)
	*d = serviceEdit{open: true, url: url, models: models, a: a, model: max(0, slices.Index(models, a.Model)), test: fmt.Sprintf("api-%d", n),
		more: a.URL != "" || a.Model != "" && a.Model != models[0]}
}

func (s *Settings) serviceForm(c *ui.Context, d *serviceEdit, setUp bool) {
	k := s.core
	names := modelNames[d.a.ID]
	var keyNote, keyLink, hostNote string
	switch d.a.ID {
	case core.QwenCloud:
		keyNote, keyLink, hostNote = locales.AsrCloudIntro(), locales.AsrCloudConsole(), locales.AsrCloudHostDescription()
	case core.OpenAI:
		keyNote, keyLink, hostNote = locales.AsrOpenAiIntro(), locales.AsrOpenAiPlatform(), locales.AsrOpenAiHostDescription()
	case core.HyMTCloud:
		keyNote, keyLink = locales.MtHyMtIntro(), locales.MtHyMtConsole()
	}
	ui.Column(c).Gap(kit.Space(3)).Children(func() {
		ui.Field(c, locales.EndpointKey(), func() {
			in := kit.TextField(c, &d.key).Password().Label(locales.EndpointKey())
			if d.a.Key {
				in.Placeholder(locales.EndpointKeyKeep())
			}
			kit.Linked(c, keyNote, keyLink, keyPages[d.a.ID], kit.TextCaption, kit.Colors(c).Faint)
		})
		if kit.Disclosure(c, &d.more, locales.ModelAndAddress()) {
			model := ui.Field(c, locales.EndpointModel(), func() {
				if names != nil {
					kit.Segmented(c, &d.model, names...).AlignSelf(ui.Start).Label(locales.EndpointModel())
				} else {
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
