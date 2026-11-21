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
	open         bool
	info         core.ServiceInfo
	service      prefs.Service
	key          string
	modelIndex   int
	test         string
	advancedOpen bool
}

var keyPages = map[string]string{
	core.QwenCloud: "https://bailian.console.aliyun.com/cn-beijing/model/settings/api-key",
	core.OpenAI:    "https://platform.openai.com/api-keys",
	core.HyMTCloud: "https://console.cloud.tencent.com/tokenhub/apikey",
}

func (d *serviceEdit) Open(k *core.Core, id string) {
	if d.open {
		k.ForgetTest(d.test)
	}
	info := core.ServiceOf(id)
	a := k.API(id)
	index := slices.IndexFunc(info.Models, func(m core.ServiceModel) bool { return m.ID == a.Model })
	*d = serviceEdit{open: true, info: info, service: a, modelIndex: max(0, index), test: k.NewTest(),
		advancedOpen: a.URL != "" || a.Model != "" && a.Model != info.Default()}
}

func (d *serviceEdit) keepsKey() bool { return strings.TrimSpace(d.key) == "" }

func (s *Settings) serviceForm(c *ui.Context, d *serviceEdit, setUp bool) {
	k := s.core
	var names []string
	if d.info.Labeled() {
		for _, m := range d.info.Models {
			names = append(names, m.Label)
		}
	}
	var keyNote, keyLink, hostNote string
	switch d.service.ID {
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
			if d.service.Key {
				in.Placeholder(locales.EndpointKeyKeep())
			}
			kit.FieldNote(c, keyNote, keyLink, keyPages[d.service.ID])
		})
		if kit.Disclosure(c, &d.advancedOpen, locales.ModelAndAddress()) {
			model := ui.Field(c, locales.EndpointModel(), func() {
				if names != nil {
					kit.Segmented(c, &d.modelIndex, names...).AlignSelf(ui.Start).Label(locales.EndpointModel())
				} else {
					kit.TextField(c, &d.service.Model).Placeholder(d.info.Default()).Label(locales.EndpointModel())
				}
			})
			if names == nil {
				model.Description(locales.AsrOpenAiModelDescription())
			}
			host := ui.Field(c, locales.AsrCloudHost(), func() {
				kit.TextField(c, &d.service.URL).Placeholder(d.info.URL).Label(locales.AsrCloudHost())
			})
			if hostNote != "" {
				host.Description(hostNote)
			}
		}
	})
	if names != nil {
		d.service.Model = d.info.Models[d.modelIndex].ID
	}
	valid := d.service.Key || !d.keepsKey()
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
			k.TestAPI(d.test, d.service, strings.TrimSpace(d.key))
		}
		if setUp && kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
			k.ForgetTest(d.test)
			*d = serviceEdit{}
			return
		}
		if kit.Off(c, kit.Button(c, kit.Primary, kit.Small, nil, locales.Save()), !valid).Clicked() {
			k.SaveAPI(d.service, strings.TrimSpace(d.key))
			c.Toast(locales.EndpointSaved(shared.EngineName(core.Engine{ID: d.service.ID})))
			k.ForgetTest(d.test)
			*d = serviceEdit{}
		}
	})
}
