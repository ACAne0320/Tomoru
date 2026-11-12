package overlay

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

// What hovering offers: the toolbar, and handles to move and resize the subtitle area.

// watchPointer turns click-through off while the pointer rests on the unlocked subtitle block, so
// it can be hovered and dragged: a window that lets clicks through hears no pointer at all.
// Passing by does nothing: the pointer has to stay still on the block for a moment, and the block
// has to show something (subtitles, or its frame while adjusting). Locked, resting there shows the
// lock above the block, which alone takes clicks, and only while the pointer is on it; it goes
// again once the pointer stays still for a while, so that a pointer left over a video shows
// nothing, and shows only after the pointer has left the block it was locked from. It runs on a
// goroutine of its own.
func (o *Overlay) watchPointer() {
	const dwell, still, linger, peekFor = 400 * time.Millisecond, 8, 500 * time.Millisecond, 3 * time.Second
	hover, peek, through, armed := false, false, true, false
	var left, restAt time.Time
	var restP mygo.Point
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-o.core.Context().Done():
			return
		case <-ticker.C:
		}
		p := mygo.Screen.CursorScreenPoint()
		b, lock := o.blockRect(o.win.Bounds())
		locked := o.locked.Load()
		over := within(p, b)
		in := over && (hover || peek || o.grabbable.Load())
		switch {
		case !in:
			restAt = time.Time{}
		case restAt.IsZero() || max(p.X-restP.X, restP.X-p.X)+max(p.Y-restP.Y, restP.Y-p.Y) > still:
			restAt, restP = time.Now(), p // moving: start the wait again from here
		}
		if in {
			left = time.Time{}
		} else if (hover || peek) && left.IsZero() {
			left = time.Now()
		}
		rested := time.Since(restAt) >= dwell
		h, pk, onLock := false, false, false
		if locked {
			armed = armed || !over
			onLock = peek && within(p, lock)
			pk = armed && (in && (peek || rested) && (onLock || time.Since(restAt) < dwell+peekFor) ||
				peek && !in && time.Since(left) < linger)
		} else {
			armed = false
			h = (in && (hover || rested || o.adjusting.Load())) || (hover && time.Since(left) < linger)
		}
		th := !h && !(pk && onLock)
		if h != hover || pk != peek || th != through {
			hover, peek, through = h, pk, th
			o.win.Update(func() {
				if o.core.Context().Err() != nil {
					return
				}
				o.hover, o.peek = h, pk
				o.win.SetIgnoreMouseEvents(th)
			})
		}
	}
}

func within(p mygo.Point, r mygo.Rectangle) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

// blockRect is the subtitle block on screen, within the overlay's bounds b: where the pointer
// finds it; and the lock shown above it while locked.
func (o *Overlay) blockRect(b mygo.Rectangle) (block, lock mygo.Rectangle) {
	o.mu.Lock()
	defer o.mu.Unlock()
	at := func(r ui.Rect) mygo.Rectangle {
		return mygo.Rectangle{X: b.X + int(r.X), Y: b.Y + int(r.Y), Width: int(r.W), Height: int(r.H)}
	}
	return at(o.block), at(o.lock)
}

// toolbarH is the toolbar's height.
const toolbarH = 36

// toolbar is the hover toolbar, centered over the subtitle block, its bottom bottom up from the
// overlay's: one capsule, in groups (playing, the scene and its languages, then locking and the
// settings, which leave the subtitles), floating clear of the text. Not shown, it fades out and
// stays, unseen, for the menu of its scene.
func (o *Overlay) toolbar(c *ui.Context, bottom float32, shown bool) ui.Element {
	k := o.core
	divider := func() { ui.Box(c).Width(1).Height(16).Margin(0, 4).Background(ui.RGBA(255, 255, 255, .16)) }
	return o.capsule(c, "toolbar", bottom, shown, func() {
		if k.Running {
			if barIcon(c, assets.Pause, shared.PauseText(k.Preferences())).Clicked() {
				k.Pause()
			}
		} else if barIcon(c, assets.Play, shared.StartText(k.Preferences())).Clicked() {
			k.Start()
		}
		divider()
		o.sceneMenu(c)
		divider()
		if barIcon(c, assets.Lock, locales.LockTooltip()).Clicked() {
			o.SetLocked(true)
		}
		if b := barIcon(c, assets.Settings, locales.Settings()); b.Clicked() {
			r := b.Bounds()
			o.nav.OpenPanel(&mygo.Rectangle{X: int(r.X), Y: int(r.Y), Width: int(r.W), Height: int(r.H)})
		}
	})
}

// unlock is the lock that shows above the locked subtitle block, where the toolbar would, while
// the pointer rests on the block: shut, as the panel's while locked, and unlocking them.
func (o *Overlay) unlock(c *ui.Context, bottom float32, shown bool) ui.Element {
	return o.capsule(c, "unlock", bottom, shown, func() {
		if barIcon(c, assets.Lock, locales.UnlockSubtitles()).Clicked() {
			o.SetLocked(false)
		}
	})
}

// capsule is the toolbar's capsule, centered over the subtitle block, its bottom bottom up from the
// overlay's, holding what children adds. Not shown, it fades out.
func (o *Overlay) capsule(c *ui.Context, key string, bottom float32, shown bool, children func()) ui.Element {
	bar := ui.Row(c.Key(key)).Absolute().Left(0).Right(0).Bottom(bottom).Justify(ui.Center)
	target := float32(0)
	if shown {
		target = 1
	}
	if op := bar.AnimateTo("shown", target, ui.Animation{Duration: fades.Duration}); op > 0 {
		bar.Opacity(op)
	} else {
		bar.Invisible() // gone: it takes neither the pointer nor the focus
	}
	return bar.Children(func() {
		// As the block: on glass where the system has it (glass_darwin.go), shaded as dark to the
		// eye. The system lightens a pane as thin as the toolbar more than the block, all of it the
		// glass's curved rim: over one color, at the block's shade it was (160, 154, 167) to the
		// block's (134, 125, 145) (2026-10-21).
		capsule := ui.Row(c).Height(toolbarH).Padding(0, 4).Gap(2).AlignItems(ui.Center).Radius(toolbarH/2).
			Border(1, ui.RGBA(255, 255, 255, .12))
		if hasGlass() {
			bg := hoverShade(o.core.Preferences())
			capsule.Background(ui.RGBA(bg.R, bg.G, bg.B, float32(bg.A)/255+.12))
		} else {
			capsule.Background(ui.RGBA(24, 24, 28, .72)).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, .28))
		}
		o.barW = capsule.Bounds().W
		capsule.Children(children)
	})
}

// barIcon is a button of the capsule: round, lit on hover, the capsule its background.
func barIcon(c *ui.Context, svg *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(28, 28).Radius(14).
		Justify(ui.Center).AlignItems(ui.Center).TextColor(ui.RGBA(255, 255, 255, .78))
	if b.Hovered() {
		b = b.Background(ui.RGBA(255, 255, 255, .16)).TextColor(ui.RGBA(255, 255, 255, 1))
	}
	return b.Children(func() { ui.Icon(c, svg).FontSize(16) })
}

// handles are the side handles that resize the subtitle area: faint, lit when the pointer comes
// to the edge, where the cursor tells them too.
func (o *Overlay) handles(c *ui.Context) {
	for _, kind := range []int{resizeLeft, resizeRight} {
		h := ui.ButtonBase(c.Key(kind)).Absolute().Top(0).Bottom(0).Width(14).Label(locales.ResizeWidth()).Cursor(ui.CursorResizeEW).
			Justify(ui.Center).AlignItems(ui.Center).Transition(fades)
		if kind == resizeRight {
			h = h.Right(0)
		} else {
			h = h.Left(0)
		}
		lit := float32(.25)
		if h.Hovered() || h.Pressed() {
			lit = .7
		}
		h.Children(func() {
			ui.Box(c).Width(3).Height(28).Radius(1.5).Background(ui.RGBA(255, 255, 255, lit)).Transition(subtitle.Follows)
		})
		o.follow(c, h, kind)
	}
}

// sceneMenu is the scene on the toolbar, with its languages, which opens shared.SceneMenu.
func (o *Overlay) sceneMenu(c *ui.Context) {
	k := o.core
	pr := k.Preferences()
	name := shared.CurrentSceneName(k)
	b := ui.ButtonBase(c).Label(locales.Scene()+": "+name).Tooltip(locales.Scene()).Height(28).Padding(0, 10).Radius(14).
		AlignItems(ui.Center).Gap(6).TextColor(ui.RGBA(255, 255, 255, .9))
	if b.Hovered() {
		b = b.Background(ui.RGBA(255, 255, 255, .16)).TextColor(ui.RGBA(255, 255, 255, 1))
	}
	b.Children(func() {
		ui.Text(c, name).FontSize(13).Bold().MaxLines(1).MaxWidth(160)
		ui.Text(c, shared.LangsOf(pr)+" ▾").FontSize(13).TextColor(ui.RGBA(255, 255, 255, .62)).SingleLine()
	})
	b.Menu(shared.SceneMenu(k, true, func() { o.nav.ShowSettingsPage("scenes") }))
}

const (
	moveArea = iota
	resizeLeft
	resizeRight
)

type resize struct {
	kind         int
	x0, y0       float32 // the pointer in the overlay as the drag began
	areaX, areaW float64
	// Moving, where the pointer held the block, on screen: from its center, and up from its bottom.
	dx, dy int
}

// follow moves the subtitle area after the pointer while h is held, to the display the pointer
// goes to, the overlay with it; or resizes it around its center from a side handle. Held, not
// pressed: a press ends as the pointer leaves the element, which it does a narrow side handle at
// once, the handle following a frame behind. (Element.DragWindow hands the drag to AppKit, which
// ignored synthesized drags in testing; following the pointer also keeps the area on screen.)
func (o *Overlay) follow(c *ui.Context, h ui.Element, kind int) {
	lx, ly, _ := h.PointerPosition()
	if _, _, held := h.Dragged(); !held {
		if o.resize != nil && o.resize.kind == kind {
			o.resize = nil
		}
		return
	}
	hb := h.Bounds()
	x, y := hb.X+lx, hb.Y+ly
	ww, _ := c.Size()
	before := o.core.Preferences()
	p := before.Clone()
	defer func() { o.core.ApplyPreferences(before, p) }()
	var at mygo.Point // the pointer on screen, which moving follows from display to display
	if kind == moveArea {
		at = mygo.Screen.CursorScreenPoint()
	}
	if o.resize == nil {
		o.resize = &resize{kind: kind, x0: x, y0: y, areaX: p.AreaX, areaW: p.AreaW,
			dx: at.X - o.area.X - int(p.AreaX*float64(o.area.Width)), dy: o.blockBottom() - at.Y}
	}
	r := o.resize
	dx := float64((x - r.x0) / ww)
	switch kind {
	case moveArea:
		if d := mygo.Screen.DisplayNearestPoint(at); d.ID != o.display.ID {
			o.moveTo(d)
			p.Screen = screenName(d)
		}
		p.AreaX = float64(at.X-r.dx-o.area.X) / float64(o.area.Width)
		p.Bottom = o.onScreen(at.Y+r.dy) - o.display.Bounds.Y
	case resizeLeft:
		p.AreaW = min(.95, max(.3, r.areaW-2*dx))
	case resizeRight:
		p.AreaW = min(.95, max(.3, r.areaW+2*dx))
	}
	p.AreaX = min(1-p.AreaW/2, max(p.AreaW/2, p.AreaX))
	c.After(16 * time.Millisecond) // keep following while pressed
}
