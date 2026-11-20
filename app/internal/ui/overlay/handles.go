package overlay

import (
	"math"
	"slices"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/prefs"
	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

// What hovering offers: the toolbar, and handles to move and resize the subtitle area.

// watchPointer has the unlocked subtitle block, with its toolbar, take the mouse while the pointer
// rests on it, so it can be hovered and dragged: a window that lets clicks through hears no
// pointer at all. Passing by does nothing: the pointer has to stay still on the block for a
// moment, and the block has to show something (subtitles, or its frame while adjusting). Locked,
// resting there shows the lock above the block, which alone takes clicks; it goes
// again once the pointer stays still for a while, so that a pointer left over a video shows
// nothing, and shows only after the pointer has left the block it was locked from. Where the
// system tells where the pointer is only over the app's own windows (pointerUnseen), the unlocked
// block takes the mouse while it shows something, and tells by its hover that the pointer is on
// it, until the pointer has not moved for a while: XWayland does not tell an X11 window that the
// pointer left it for a Wayland one. Locked, it cannot show the lock. It runs on a goroutine of its
// own.
//
// While nothing can be grabbed (parks), it sleeps until View or SetLocked wakes it, and the
// window lets all of the mouse through, with no input region: an empty one still has MyGo watch
// every move of the mouse over any app on macOS, and ask where the cursor is at each tick of the
// system's clock on Windows. Asking where the pointer is every 50 ms, idle, the app used 1.4% of a
// core; parked, 0.0% (2026-11-20). It alone sets how the window takes the mouse, on the main
// thread, in order.
func (o *Overlay) watchPointer() {
	const tick = 50 * time.Millisecond // how often it looks, awake
	const dwell, still, linger, peekFor = 400 * time.Millisecond, 8, 500 * time.Millisecond, 3 * time.Second
	hover, peek, armed := false, false, false
	parked := true              // as the window starts
	var taking []mygo.Rectangle // the part of the overlay that takes the mouse, as last set
	var left, restAt, movedAt time.Time
	var restP, lastP mygo.Point
	ticker := time.NewTicker(tick)
	ticker.Stop()
	defer ticker.Stop()
	for {
		select {
		case <-o.core.Context().Done():
			return
		case <-o.woken:
		case <-ticker.C:
		}
		p := mygo.Screen.CursorScreenPoint()
		b, lock := o.blockRect(o.win.Bounds())
		locked, grabbable := o.locked.Load(), o.grabbable.Load()
		over := within(p, b)
		if p != lastP {
			lastP, movedAt = p, time.Now()
		}
		if pointerUnseen && !locked {
			over = o.pointerOn.Load() && time.Since(movedAt) < peekFor
		}
		in := over && (hover || peek || grabbable)
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
		park := parks(grabbable, h, pk)
		if park {
			// Asleep, it cannot tell that the pointer left: the lock may show once something does.
			armed = true
		}
		if park != parked {
			if park {
				ticker.Stop()
			} else {
				ticker.Reset(tick)
			}
		}
		// What takes the mouse: the system lets the rest through, and switches as the pointer comes
		// in or leaves, at once, rather than at the next tick here.
		region := inputRegion(h || pointerUnseen && !locked && grabbable, pk, o)
		if h != hover || pk != peek || park != parked || !slices.Equal(region, taking) {
			woke := parked && !park
			hover, peek, parked, taking = h, pk, park, region
			o.win.Update(func() {
				if o.core.Context().Err() != nil {
					return
				}
				o.hover, o.peek = h, pk
				// In this order, the window never takes all of the mouse: ignoring it wins over the
				// region, and no region is all of it.
				switch {
				case park:
					o.win.SetIgnoreMouseEvents(true)
					o.win.SetInputRegion(nil)
				case woke:
					o.win.SetInputRegion(region)
					o.win.SetIgnoreMouseEvents(false)
				default:
					o.win.SetInputRegion(region)
				}
			})
		}
	}
}

// parks tells whether the pointer watcher sleeps and the window lets all of the mouse through:
// while the block shows nothing to grab (adjusting, its frame is something), and is neither
// hovered nor showing the lock.
func parks(grabbable, hover, peek bool) bool { return !grabbable && !hover && !peek }

// wake has the pointer watcher look at once, as something to grab shows: it may be asleep. It
// never waits.
func (o *Overlay) wake() {
	select {
	case o.woken <- struct{}{}:
	default:
	}
}

func within(p mygo.Point, r mygo.Rectangle) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

// blockRect is the subtitle block on screen, within the overlay's bounds b: where the pointer
// finds it; and the lock shown above it while locked.
func (o *Overlay) blockRect(b mygo.Rectangle) (block, lock mygo.Rectangle) {
	hit, lock := o.hitArea()
	at := func(r mygo.Rectangle) mygo.Rectangle {
		return mygo.Rectangle{X: b.X + r.X, Y: b.Y + r.Y, Width: r.Width, Height: r.Height}
	}
	return at(hit), at(lock)
}

// hitArea is the subtitle block, with its toolbar while hovered, and the lock shown above it while
// locked, in the overlay, whole DIPs around them.
func (o *Overlay) hitArea() (block, lock mygo.Rectangle) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return around(o.block), around(o.lock)
}

// inputRegion is the part of the overlay that takes the mouse: the block with its toolbar while
// hovered, or unlocked where the pointer is unseen, the lock alone while it shows, else none.
func inputRegion(hover, peek bool, o *Overlay) []mygo.Rectangle {
	hit, lock := o.hitArea()
	switch {
	case hover:
		return []mygo.Rectangle{hit}
	case peek:
		return []mygo.Rectangle{lock}
	}
	return []mygo.Rectangle{}
}

// around is r in whole DIPs, out to the pixels it touches.
func around(r ui.Rect) mygo.Rectangle {
	x, y := int(math.Floor(float64(r.X))), int(math.Floor(float64(r.Y)))
	return mygo.Rectangle{X: x, Y: y, Width: int(math.Ceil(float64(r.X+r.W))) - x, Height: int(math.Ceil(float64(r.Y+r.H))) - y}
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
				k.Stop()
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
		row := capsule(o.core.Preferences(), ui.Row(c).Height(toolbarH).Padding(0, 4).Gap(2).AlignItems(ui.Center).Radius(toolbarH/2))
		o.barW = row.Bounds().W
		row.Children(children)
	})
}

// capsule gives e, a capsule over the block, the block's look: on glass where the system has it
// (glass_darwin.go), shaded as dark to the eye. The system lightens a pane as thin as a capsule more
// than the block, all of it the glass's curved rim: over one color, at the block's shade the toolbar
// was (160, 154, 167) to the block's (134, 125, 145) (2026-10-21).
func capsule(p prefs.Prefs, e ui.Element) ui.Element {
	e = e.Border(1, ui.RGBA(255, 255, 255, .12))
	if hasGlass() {
		bg := hoverShade(p)
		return e.Material(onGlass{ui.RGBA(bg.R, bg.G, bg.B, float32(bg.A)/255+.12)})
	}
	return e.Background(ui.RGBA(24, 24, 28, .72)).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, .28))
}

// onGlass fills an element with the system's clear glass and shade over it, a ui.Material. MyGo
// puts the glass, a view of the system's, under the frame where the element is, and has the frame
// present with Core Animation's transaction: placed by hand as the overlay was built, the glass
// moved before the frame, presented on its own, showed, and dragged, it ran ahead of the block and
// the toolbar (2026-11-20).
type onGlass struct{ shade ui.Color }

// NativeGlass asks for the clear glass, as over photos and video: what plays shows through, barely
// frosted.
func (onGlass) NativeGlass() (ui.NativeGlassSpec, bool) { return ui.NativeGlassSpec{Style: 1}, true }

func (g onGlass) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if p.NativeGlassSupported() && !p.Preferences().ReduceTransparency {
		p.Hole(box, radii)
	}
	if g.shade.A > 0 {
		p.FillRadii(box, radii, g.shade)
	}
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
		ui.Text(c, shared.LangsOf(pr)).FontSize(13).TextColor(ui.RGBA(255, 255, 255, .62)).SingleLine()
		// The menu's mark, as the app's other menus draw it: a "▾" took a fallback font's taller line
		// where the system's font has no such glyph, and the row, centered, set the languages higher
		// than the scene's name (Noto Sans on Linux, 2026-11-20).
		ui.Icon(c, assets.Chevron).FontSize(12).TextColor(ui.RGBA(255, 255, 255, .62)).Margin(0, 0, 0, -2)
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
// once, the handle following a frame behind.
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
