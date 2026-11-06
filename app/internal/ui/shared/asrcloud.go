package shared

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/asr"
	"tomoru/internal/asr/openai"
	"tomoru/internal/asr/qwencloud"
	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

// The dialog that sets up a cloud recognition service, Qwen's or OpenAI's: its API key, its
// address and its model. In the settings, and in the first-run guide where the system recognizes
// nothing.

// ASRCloudEdit is the dialog's state while it shows.
type ASRCloudEdit struct {
	open  bool
	spec  asr.Spec
	a     prefs.ASRAPI // as edited
	key   string       // a new API key, or "" to keep the one kept
	model int          // Qwen's, of its Models
	test  string       // the key of its tests in Core.Tests
	was   prefs.ASRAPI // as it was, with the model the dialog shows at first
	// asking asks, in place of the dialog's buttons, before its changes are thrown away.
	asking bool
}

// Open opens the dialog on the service id as k has it. n tells its tests from those of the dialogs
// before.
func (d *ASRCloudEdit) Open(k *core.Core, id string, n int) {
	spec, _ := asr.ByID(id)
	a := k.ASRAPI(id)
	*d = ASRCloudEdit{open: true, spec: spec, a: a, model: max(0, slices.Index(spec.Models, a.Model)), test: fmt.Sprintf("asr-%d", n), was: a}
	if a.ID == core.QwenCloud {
		d.was.Model = spec.Models[d.model]
	}
}

// changed tells whether the service was edited.
func (d *ASRCloudEdit) changed() bool { return d.a != d.was || d.key != "" }

// ASRCloudDialog is the dialog, while d is open. Saved, the service is used, if it was chosen.
func ASRCloudDialog(c *ui.Context, k *core.Core, d *ASRCloudEdit) {
	if !d.open {
		return
	}
	qwen := d.a.ID == core.QwenCloud
	intro, hostNote, placeholder := locales.AsrOpenAiIntro(), locales.AsrOpenAiHostDescription(), d.spec.URL
	if qwen {
		intro, hostNote = locales.AsrCloudIntro(), locales.AsrCloudHostDescription()
	}
	dialog := kit.Dialog(c, &d.open, locales.AsrCloudTitle(), intro, func() {
		ui.Form(c, func() {
			ui.Field(c, locales.EndpointKey(), func() {
				in := kit.TextField(c, &d.key).Password().Label(locales.EndpointKey())
				if d.a.Key {
					in.Placeholder(locales.EndpointKeyKeep())
				}
			})
			ui.Field(c, locales.AsrCloudHost(), func() {
				kit.TextField(c, &d.a.URL).Placeholder(placeholder).Label(locales.AsrCloudHost())
			}).Description(hostNote)
			model := ui.Field(c, locales.EndpointModel(), func() {
				if qwen {
					kit.Segmented(c, &d.model, "3.1", "3.0").AlignSelf(ui.Start).Label(locales.EndpointModel())
				} else {
					// A compatible service names its own models.
					kit.TextField(c, &d.a.Model).Placeholder(d.spec.Models[0]).Label(locales.EndpointModel())
				}
			})
			if !qwen {
				model.Description(locales.AsrOpenAiModelDescription())
			}
		})
		if qwen {
			d.a.Model = d.spec.Models[d.model]
		}
		valid := d.a.Key || strings.TrimSpace(d.key) != ""
		if r, ok := k.Tests[d.test]; ok {
			ms := fmt.Sprint(r.Took.Milliseconds())
			switch {
			case r.Running:
				kit.Waiting(c, locales.Testing())
			case r.Err != nil:
				kit.Result(c, false, locales.TestFailed(ms), ErrorText(r.Err))
			default:
				kit.Result(c, true, locales.TestSucceeded(ms), "")
			}
		}
		if d.asking {
			kit.Discard(c, &d.open, &d.asking)
			return
		}
		kit.DialogActions(c, func() {
			ui.Spacer(c)
			if kit.Off(c, kit.Button(c, kit.Plain, kit.Small, nil, locales.Test()), !valid || k.Tests[d.test].Running).Clicked() {
				k.TestASRAPI(d.test, d.a, strings.TrimSpace(d.key))
			}
			if kit.Button(c, kit.Plain, kit.Small, nil, locales.Cancel()).Clicked() {
				d.open = false
			}
			if kit.Off(c, kit.Button(c, kit.Primary, kit.Small, nil, locales.Save()), !valid).Clicked() {
				k.SaveASRAPI(d.a, strings.TrimSpace(d.key))
				c.Toast(locales.EndpointSaved(EngineName(core.Engine{ID: d.a.ID, Kind: core.ASR})))
				d.open = false
			}
		})
	})
	kit.Guard(dialog, &d.open, &d.asking, d.changed())
	if !d.open {
		k.ForgetTest(d.test)
	}
}

// asrCloudError says why a cloud recognition service failed, or "" for an error not its own.
func asrCloudError(err error) string {
	var qe *qwencloud.Error
	var oe *openai.Error
	switch {
	case errors.Is(err, core.ErrASRCloudKey):
		return locales.ErrAsrCloudKey()
	case errors.Is(err, qwencloud.ErrAuth):
		return locales.ErrAsrCloudAuth()
	case errors.Is(err, openai.ErrAuth):
		return locales.ErrAsrApiAuth()
	case errors.As(err, &qe):
		return locales.ErrAsrCloud(qe.Message)
	case errors.As(err, &oe):
		return locales.ErrAsrCloud(oe.Message)
	}
	return ""
}
