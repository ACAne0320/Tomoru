package shell

import (
	"slices"
	"testing"

	"github.com/egoist/mygo"

	"tomoru/internal/core/coretest"
	"tomoru/internal/prefs"
	"tomoru/locales"
)

// TestTrayMenu has the menu bar icon's menu start translating, or stop it, with its shortcut shown,
// translate a part of the screen, open the app or say what it is, and quit, apart.
func TestTrayMenu(t *testing.T) {
	k := coretest.New()
	s := &Shell{core: k}
	labels := func(m *mygo.Menu) (l []string) {
		for _, it := range m.Items() {
			l = append(l, it.Label)
		}
		return l
	}
	m := s.trayMenu()
	want := []string{locales.StartTranslating(), locales.TranslateScreen(), "", locales.OpenTomoru(), locales.AboutTomoru(), "", locales.QuitTomoru()}
	if got := labels(m); !slices.Equal(got, want) {
		t.Errorf("menu %q, want %q", got, want)
	}
	if acc := m.Items()[0].Accelerator; acc != k.Preferences().Shortcut {
		t.Errorf("shows %q, the shortcut %q", acc, k.Preferences().Shortcut)
	}
	m.Items()[0].Click(m.Items()[0], nil)
	if !k.Started {
		t.Error("not started")
	}
	k.Running = true
	if got := s.trayMenu().Items()[0].Label; got != locales.Pause() {
		t.Errorf("translating: %q", got)
	}
	// With the original only, it starts and pauses subtitles.
	k.Prefs.Display = prefs.DisplaySrc
	if got := s.trayMenu().Items()[0].Label; got != locales.PauseSubtitles() {
		t.Errorf("subtitles: %q", got)
	}
	k.Running = false
	if got := s.trayMenu().Items()[0].Label; got != locales.StartSubtitles() {
		t.Errorf("subtitles paused: %q", got)
	}
}

// TestTrayMenuOnly has the menu of an icon that only has its menu, Linux's, show the panel first.
func TestTrayMenuOnly(t *testing.T) {
	defer func(was bool) { menuOnly = was }(menuOnly)
	menuOnly = true
	s := &Shell{core: coretest.New()}
	m := s.trayMenu()
	if got := m.Items()[0].Label; got != locales.OpenPanel() {
		t.Fatalf("first %q, want %q", got, locales.OpenPanel())
	}
	if got := m.Items()[1].Label; got != locales.StartTranslating() {
		t.Errorf("then %q, want %q", got, locales.StartTranslating())
	}
}
