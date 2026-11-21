package coretest

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"

	"tomoru/internal/core"
	"tomoru/internal/engine"
	"tomoru/internal/live"
	"tomoru/internal/models"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/shared"
)

func New() *core.Core { return NewWith(nil) }

func NewWith(edit func(*core.Deps)) *core.Core {
	p := prefs.Default("zh-Hans")
	p.Src = "ja"
	d := core.Deps{
		Post:       func(func()) {},
		EnginePath: func() (string, error) { return "/engine", nil },
		StartSession: func(live.Config, func(live.State)) (core.Session, error) {
			return nil, errors.New("no sessions in tests")
		},
		RunEngine: func(context.Context, string, ...string) ([]engine.Event, error) { return nil, nil },
		Secrets:   Secrets{},
	}
	if edit != nil {
		edit(&d)
	}
	k := core.New(p, d)
	k.Access = "granted"
	k.CheckedAt = time.Now()
	k.ASR["ja"], k.ASR["zh"] = "ready", "ready"
	return k
}

type Secrets map[string][]byte

func (s Secrets) Get(k string) ([]byte, error) {
	if v, ok := s[k]; ok {
		return v, nil
	}
	return nil, errors.New("not found")
}
func (s Secrets) Set(k string, v []byte) error { s[k] = v; return nil }
func (s Secrets) Delete(k string) error        { delete(s, k); return nil }

type Nav struct {
	Went        []string
	Shortcut    string
	Image       string
	PanelHeight int
}

func (n *Nav) OpenPanel() { n.Went = append(n.Went, "panel") }

func (n *Nav) OpenPanelAbove(mygo.Rectangle)  { n.Went = append(n.Went, "panel") }
func (n *Nav) FitPanel(h int)                 { n.PanelHeight = h }
func (n *Nav) ShowSettings()                  { n.Went = append(n.Went, "settings") }
func (n *Nav) ShowSettingsPage(p shared.Page) { n.Went = append(n.Went, "settings "+string(p)) }
func (n *Nav) ShowGuide()                     { n.Went = append(n.Went, "guide") }
func (n *Nav) FinishGuide()                   { n.Went = append(n.Went, "finish") }
func (n *Nav) StartAdjust()                   { n.Went = append(n.Went, "adjust") }
func (n *Nav) TranslateImage()                { n.Went = append(n.Went, "image") }
func (n *Nav) SetLocked(bool)                 { n.Went = append(n.Went, "lock") }
func (n *Nav) SetHideFromCapture(bool)        { n.Went = append(n.Went, "capture") }
func (n *Nav) Focus()                         { n.Went = append(n.Went, "focus") }
func (n *Nav) SetAppearance(a string)         { n.Went = append(n.Went, "appearance "+a) }

func (n *Nav) SetShortcut(acc string) error {
	if acc == Taken {
		return errors.New("taken")
	}
	n.Shortcut = acc
	return nil
}

func (n *Nav) SetImageShortcut(acc string) error {
	if acc == Taken {
		return errors.New("taken")
	}
	n.Image = acc
	return nil
}

const Taken = "Cmd+Space"

type Models struct {
	St                 map[string]models.Status
	Downloads, Cancels []string
	MoveStatus         models.Move
	Moves              []string
}

func (m *Models) Status(id string) models.Status   { return m.St[id] }
func (m *Models) Path(id string) string            { return "/models/" + id }
func (m *Models) Download(id string)               { m.Downloads = append(m.Downloads, id) }
func (m *Models) Cancel(id string)                 { m.Cancels = append(m.Cancels, id) }
func (m *Models) Delete(id string) error           { delete(m.St, id); return nil }
func (m *Models) Dir() string                      { return "/models" }
func (m *Models) Moving() models.Move              { return m.MoveStatus }
func (m *Models) Move(to string, done func(error)) { m.Moves = append(m.Moves, to); done(nil) }

func GalleryDir(t testing.TB) string {
	dir := os.Getenv("TOMORU_GALLERY")
	if l := os.Getenv("TOMORU_GALLERY_LOCALE"); dir != "" && l != "" {
		i18n.SetLocaleFunc(func() string { return l })
		t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	}
	return dir
}
