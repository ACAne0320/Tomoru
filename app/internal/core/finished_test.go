package core

import (
	"errors"
	"testing"

	"tomoru/internal/models"
)

func TestFinished(t *testing.T) {
	c, _, m, _, _ := engineCore(t)
	id := models.All[0].ID
	c.Changed()
	m.state = map[string]models.State{id: models.Downloading}
	c.Changed()
	if got := c.TakeFinished(); len(got) != 0 {
		t.Fatalf("finished while downloading: %+v", got)
	}
	delete(m.state, id)
	m.ready[id] = true
	c.Changed()
	if got := c.TakeFinished(); len(got) != 1 || got[0].What != ModelReady || got[0].Kind != MT || got[0].ID != id {
		t.Fatalf("downloaded: %+v", got)
	}
	c.Changed()
	if got := c.TakeFinished(); len(got) != 0 {
		t.Fatalf("ready twice: %+v", got)
	}

	m.moving = models.Move{Active: true}
	c.Changed()
	m.moving = models.Move{}
	c.Changed()
	if got := c.TakeFinished(); len(got) != 1 || got[0].What != ModelsMoved {
		t.Fatalf("moved: %+v", got)
	}
	m.moving = models.Move{Active: true}
	c.Changed()
	m.moving = models.Move{Err: errors.New("disk full")}
	c.Changed()
	if got := c.TakeFinished(); len(got) != 0 {
		t.Fatalf("a failed move finished: %+v", got)
	}
}
