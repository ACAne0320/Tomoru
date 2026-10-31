package core

import "time"

// Shortcut is one of the app's global shortcuts.
type Shortcut int

const (
	// StartStop starts and stops translating.
	StartStop Shortcut = iota
	// PictureShortcut translates a picture: a part of the screen selected, or the clipboard's.
	PictureShortcut
)

// Press tells that shortcut s was pressed: Pressed says when. It reports whether it is held, as the
// guide holds a shortcut while it checks it: then it does nothing else.
func (c *Core) Press(s Shortcut) (held bool) {
	c.Pressed[s] = c.now()
	c.changed()
	return c.Held[s]
}

// PressedSince tells whether shortcut s was pressed after t.
func (c *Core) PressedSince(s Shortcut, t time.Time) bool { return c.Pressed[s].After(t) }
