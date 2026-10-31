package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShortcutText(t *testing.T) {
	defer func(m bool) { macKeys = m }(macKeys)
	for _, c := range []struct{ acc, mac, windows string }{
		{"Alt+Shift+T", "⌥⇧T", "Alt+Shift+T"},
		{"Shift+Cmd+K", "⇧⌘K", "Shift+Win+K"},
		{"Ctrl+Alt+Space", "⌃⌥Space", "Ctrl+Alt+Space"},
		{"Super+Left", "⌘←", "Win+←"},
		{"Ctrl+Enter", "⌃↩", "Ctrl+Enter"},
		{"F5", "F5", "F5"},
	} {
		macKeys = true
		if got := ShortcutText(c.acc); got != c.mac {
			t.Errorf("%s on macOS: %s, want %s", c.acc, got, c.mac)
		}
		macKeys = false
		if got := ShortcutText(c.acc); got != c.windows {
			t.Errorf("%s on Windows: %s, want %s", c.acc, got, c.windows)
		}
	}
}

func TestAccelerator(t *testing.T) {
	for _, c := range []struct {
		mods ui.Modifiers
		key  ui.Key
		want string
	}{
		{ui.Alt | ui.Shift, ui.KeyT, "Alt+Shift+T"},
		{ui.Super | ui.Ctrl, ui.Key1, "Ctrl+Cmd+1"},
		{0, ui.KeyF5, "F5"},
		{ui.Shift, ui.KeyT, ""}, // Shift alone types a capital
		{0, ui.KeyT, ""},
		{ui.Super, ui.KeyUnknown, ""},
	} {
		got, ok := Accelerator(c.mods, c.key)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%v %v: %q %v, want %q", c.mods, c.key, got, ok, c.want)
		}
		if ok && ShortcutText(got) == "" {
			t.Errorf("%s shows as nothing", got)
		}
	}
}

func TestShortcutMods(t *testing.T) {
	got := ShortcutMods("Alt+Shift+T")
	if len(got) != 3 || got[0] != ui.Alt || got[1] != ui.Shift || got[2] != 0 {
		t.Errorf("Alt+Shift+T: %v", got)
	}
	if got := ShortcutMods("F5"); len(got) != 1 || got[0] != 0 {
		t.Errorf("F5: %v", got)
	}
}
