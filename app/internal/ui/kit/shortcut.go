package kit

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// The global shortcut, as the settings keep it: an accelerator ("Alt+Shift+T"), which
// mygo.GlobalShortcut registers. It shows as the system writes shortcuts: on macOS its modifiers'
// symbols in their order (⌃⌥⇧⌘) before its key, elsewhere their names, as Windows does
// ("Ctrl+Alt+T").

// macKeys tells whether shortcuts show as macOS shows them.
var macKeys = runtime.GOOS == "darwin"

// mods are the modifiers of accelerators, in the order they show, with their symbols on macOS and
// their names elsewhere.
var mods = []struct {
	name, symbol, label string
	mod                 ui.Modifiers
}{
	{"Ctrl", "⌃", "Ctrl", ui.Ctrl}, {"Alt", "⌥", "Alt", ui.Alt}, {"Shift", "⇧", "Shift", ui.Shift}, {"Cmd", "⌘", "Win", ui.Super},
}

// keyNames are the keys a global shortcut can end with, by their names in accelerators. Letters
// and digits are themselves.
var keyNames = map[ui.Key]string{
	ui.KeySpace: "Space", ui.KeyEnter: "Enter", ui.KeyTab: "Tab", ui.KeyBackspace: "Backspace",
	ui.KeyDelete: "Delete", ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PageUp",
	ui.KeyPageDown: "PageDown", ui.KeyLeft: "Left", ui.KeyRight: "Right", ui.KeyUp: "Up", ui.KeyDown: "Down",
	ui.KeyF1: "F1", ui.KeyF2: "F2", ui.KeyF3: "F3", ui.KeyF4: "F4", ui.KeyF5: "F5", ui.KeyF6: "F6",
	ui.KeyF7: "F7", ui.KeyF8: "F8", ui.KeyF9: "F9", ui.KeyF10: "F10", ui.KeyF11: "F11", ui.KeyF12: "F12",
	ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".", ui.KeySlash: "/",
	ui.KeySemicolon: ";", ui.KeyQuote: "'", ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]",
	ui.KeyBackslash: "\\", ui.KeyBackquote: "`",
}

func init() {
	for i := range 26 {
		keyNames[ui.KeyA+ui.Key(i)] = string(rune('A' + i))
	}
	for i := range 10 {
		keyNames[ui.Key0+ui.Key(i)] = string(rune('0' + i))
	}
}

// arrows are the keys that show as their symbols on every system.
var arrows = map[string]bool{"Left": true, "Right": true, "Up": true, "Down": true}

// keySymbols are how keys with a symbol of their own show on macOS.
var keySymbols = map[string]string{
	"Space": "Space", "Enter": "↩", "Tab": "⇥", "Backspace": "⌫", "Delete": "⌦", "Home": "↖", "End": "↘",
	"PageUp": "⇞", "PageDown": "⇟", "Left": "←", "Right": "→", "Up": "↑", "Down": "↓",
}

// ShortcutKeys are the keys of an accelerator as they show: its modifiers' symbols, then its key.
func ShortcutKeys(acc string) []string {
	keys, _ := shortcutKeys(acc)
	return keys
}

// ShortcutMods are the modifiers of an accelerator's keys, as ShortcutKeys has them: each
// modifier's, then 0 for its key.
func ShortcutMods(acc string) []ui.Modifiers {
	_, ms := shortcutKeys(acc)
	return ms
}

// shortcutKeys are the keys of an accelerator as they show, and the modifier each is.
func shortcutKeys(acc string) ([]string, []ui.Modifiers) {
	parts := strings.Split(acc, "+")
	key := parts[len(parts)-1]
	if strings.HasSuffix(acc, "++") {
		parts, key = parts[:len(parts)-1], "+"
	}
	var keys []string
	var ms []ui.Modifiers
	for _, m := range mods {
		for _, p := range parts[:len(parts)-1] {
			if strings.EqualFold(p, m.name) || m.name == "Cmd" && (strings.EqualFold(p, "Super") || strings.EqualFold(p, "Command")) ||
				m.name == "Alt" && strings.EqualFold(p, "Option") {
				label := m.label
				if macKeys {
					label = m.symbol
				}
				keys, ms = append(keys, label), append(ms, m.mod)
				break
			}
		}
	}
	if s, ok := keySymbols[key]; ok && (macKeys || arrows[key]) {
		key = s
	} else if len(key) == 1 {
		key = strings.ToUpper(key)
	}
	return append(keys, key), append(ms, 0)
}

// ShortcutText is an accelerator as a line of text says it: "⌥⇧T" on macOS, "Alt+Shift+T"
// elsewhere.
func ShortcutText(acc string) string {
	if macKeys {
		return strings.Join(ShortcutKeys(acc), "")
	}
	return strings.Join(ShortcutKeys(acc), "+")
}

// ShortcutKey tells whether a global shortcut can end with key.
func ShortcutKey(key ui.Key) bool {
	_, ok := keyNames[key]
	return ok
}

// Accelerator is the accelerator of a key pressed with mods, if it makes a global shortcut: the
// key with Control, Option or Command (Shift alone would take a capital letter from every app), or
// a function key alone.
func Accelerator(m ui.Modifiers, key ui.Key) (string, bool) {
	name, ok := keyNames[key]
	if !ok {
		return "", false
	}
	if m&(ui.Ctrl|ui.Alt|ui.Super) == 0 && !(key >= ui.KeyF1 && key <= ui.KeyF12) {
		return "", false
	}
	var parts []string
	for _, x := range mods {
		if m&x.mod != 0 {
			parts = append(parts, x.name)
		}
	}
	return strings.Join(append(parts, name), "+"), true
}

// KeyCaps shows an accelerator's keys as the keys of a keyboard, one cap each.
func KeyCaps(c *ui.Context, acc string) {
	t := c.Theme()
	ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
		for _, k := range ShortcutKeys(acc) {
			ui.Text(c, k).FontSize(12).Bold().TextAlign(ui.Center).MinWidth(22).Height(22).Padding(0, 6).Radius(5).
				Background(t.Background).Border(1, t.Border).Shadow(0, 1, 0, 0, t.Border).Justify(ui.Center).AlignItems(ui.Center)
		}
	})
}
