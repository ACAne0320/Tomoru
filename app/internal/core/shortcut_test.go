package core

import (
	"testing"
	"time"
)

// TestPress tells when a shortcut was pressed, and whether it is held: held, it does nothing else.
func TestPress(t *testing.T) {
	c, _ := newCore(t)
	before := time.Now().Add(-time.Second)
	if c.Press(StartStop) || !c.PressedSince(StartStop, before) || c.PressedSince(PictureShortcut, before) {
		t.Errorf("not held: pressed %v", c.Pressed)
	}
	c.Held[PictureShortcut] = true
	if !c.Press(PictureShortcut) || !c.PressedSince(PictureShortcut, before) {
		t.Errorf("held: pressed %v", c.Pressed)
	}
}
