package core

import "time"

type Shortcut int

const (
	StartStop Shortcut = iota
	PictureShortcut
)

func (c *Core) Press(s Shortcut) (held bool) {
	c.Pressed[s] = c.now()
	c.changed()
	return c.held[s]
}

func (c *Core) Holds(s Shortcut) bool { return c.held[s] }

func (c *Core) HoldShortcut(s Shortcut, held bool) { c.held[s] = held }

func (c *Core) ReleaseShortcuts() { clear(c.held) }

func (c *Core) PressedSince(s Shortcut, t time.Time) bool { return c.Pressed[s].After(t) }
