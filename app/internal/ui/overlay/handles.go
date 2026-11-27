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

func (o *Overlay) watchPointer() {
	const tick = 50 * time.Millisecond
	const dwell, still, linger, peekFor = 400 * time.Millisecond, 8, 500 * time.Millisecond, 3 * time.Second
	hover, peek, leftSinceLock := false, false, false
	parked := true
	var taking []mygo.Rectangle
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
			restAt, restP = time.Now(), p
		}
		if in {
			left = time.Time{}
		} else if (hover || peek) && left.IsZero() {
			left = time.Now()
		}
		rested := time.Since(restAt) >= dwell
		h, pk, onLock := false, false, false
		if locked {
			leftSinceLock = leftSinceLock || !over
			onLock = peek && within(p, lock)
			pk = leftSinceLock && (in && (peek || rested) && (onLock || time.Since(restAt) < dwell+peekFor) ||
				peek && !in && time.Since(left) < linger)
		} else {
			leftSinceLock = false
			h = (in && (hover || rested || o.adjusting.Load())) || (hover && time.Since(left) < linger)
		}
		park := parks(grabbable, h, pk)
		if park {
			leftSinceLock = true
		}
		if park != parked {
			if park {
				ticker.Stop()
			} else {
				ticker.Reset(tick)
			}
		}
		region := inputRegion(h || pointerUnseen && !locked && grabbable, pk, o)
		if h != hover || pk != peek || park != parked || !slices.Equal(region, taking) {
			woke := parked && !park
			hover, peek, parked, taking = h, pk, park, region
			o.win.Update(func() {
				if o.core.Context().Err() != nil {
					return
				}
				o.hover, o.showingUnlock = h, pk
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

func parks(grabbable, hover, peek bool) bool { return !grabbable && !hover && !peek }

func (o *Overlay) wake() {
	select {
	case o.woken <- struct{}{}:
	default:
	}
}

func within(p mygo.Point, r mygo.Rectangle) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

func (o *Overlay) blockRect(b mygo.Rectangle) (block, lock mygo.Rectangle) {
	hit, lock := o.hitArea()
	at := func(r mygo.Rectangle) mygo.Rectangle {
		return mygo.Rectangle{X: b.X + r.X, Y: b.Y + r.Y, Width: r.Width, Height: r.Height}
	}
	return at(hit), at(lock)
}

func (o *Overlay) hitArea() (block, lock mygo.Rectangle) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return around(o.block), around(o.lock)
}

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

func around(r ui.Rect) mygo.Rectangle {
	x, y := int(math.Floor(float64(r.X))), int(math.Floor(float64(r.Y)))
	return mygo.Rectangle{X: x, Y: y, Width: int(math.Ceil(float64(r.X+r.W))) - x, Height: int(math.Ceil(float64(r.Y+r.H))) - y}
}

const toolbarH = 36

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
			o.nav.OpenPanelAbove(mygo.Rectangle{X: int(r.X), Y: int(r.Y), Width: int(r.W), Height: int(r.H)})
		}
	})
}

func (o *Overlay) unlock(c *ui.Context, bottom float32, shown bool) ui.Element {
	return o.capsule(c, "unlock", bottom, shown, func() {
		if barIcon(c, assets.Lock, locales.UnlockSubtitles()).Clicked() {
			o.SetLocked(false)
		}
	})
}

func (o *Overlay) capsule(c *ui.Context, key string, bottom float32, shown bool, children func()) ui.Element {
	bar := ui.Row(c.Key(key)).Absolute().Left(0).Right(0).Bottom(bottom).Justify(ui.Center)
	target := float32(0)
	if shown {
		target = 1
	}
	if op := bar.AnimateTo("shown", target, ui.Animation{Duration: fades.Duration}); op > 0 {
		bar.Opacity(op)
	} else {
		bar.Invisible()
	}
	return bar.Children(func() {
		row := capsule(o.core.Preferences(), ui.Row(c).Height(toolbarH).Padding(0, 4).Gap(2).AlignItems(ui.Center).Radius(toolbarH/2))
		o.barW = row.Bounds().W
		row.Children(children)
	})
}

func capsule(p prefs.Prefs, e ui.Element) ui.Element {
	e = e.Border(1, ui.RGBA(255, 255, 255, .12))
	if hasGlass() {
		bg := hoverShade(p)
		return e.Material(onGlass{ui.RGBA(bg.R, bg.G, bg.B, float32(bg.A)/255+.12)})
	}
	return e.Background(ui.RGBA(24, 24, 28, .72)).Shadow(0, 6, 20, 0, ui.RGBA(0, 0, 0, .28))
}

type onGlass struct{ shade ui.Color }

const clearGlass = 1

func (onGlass) NativeGlass() (ui.NativeGlassSpec, bool) {
	return ui.NativeGlassSpec{Style: clearGlass}, true
}

func (g onGlass) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	if p.NativeGlassSupported() && !p.Preferences().ReduceTransparency {
		p.Hole(box, radii)
	}
	if g.shade.A > 0 {
		p.FillRadii(box, radii, g.shade)
	}
}

func barIcon(c *ui.Context, svg *ui.SVG, label string) ui.Element {
	b := ui.ButtonBase(c).Label(label).Tooltip(label).Size(28, 28).Radius(14).
		Justify(ui.Center).AlignItems(ui.Center).TextColor(ui.RGBA(255, 255, 255, .78))
	if b.Hovered() {
		b = b.Background(ui.RGBA(255, 255, 255, .16)).TextColor(ui.RGBA(255, 255, 255, 1))
	}
	return b.Children(func() { ui.Icon(c, svg).FontSize(16) })
}

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
		ui.Icon(c, assets.Chevron).FontSize(12).TextColor(ui.RGBA(255, 255, 255, .62)).Margin(0, 0, 0, -2)
	})
	b.Menu(shared.SceneMenu(k, true, func() { o.nav.ShowSettingsPage(shared.PageScenes) }))
}

const (
	moveArea = iota
	resizeLeft
	resizeRight
)

type resize struct {
	kind                            int
	x0, y0                          float32
	areaX, areaW                    float64
	grabFromCenter, grabAboveBottom int
}

func (o *Overlay) follow(c *ui.Context, h ui.Element, kind int) {
	lx, ly, _ := h.PointerPosition()
	if _, _, held := h.Dragged(); !held {
		if o.resize != nil && o.resize.kind == kind {
			o.resize = nil
		}
		return
	}
	ww, _ := c.Size()
	fw, ox, _ := o.canvas(ww)
	hb := h.Bounds()
	x, y := hb.X+lx+ox, hb.Y+ly
	before := o.core.Preferences()
	p := before.Clone()
	defer func() { o.core.ApplyPreferences(before, p) }()
	var at mygo.Point
	if kind == moveArea {
		at = mygo.Screen.CursorScreenPoint()
	}
	if o.resize == nil {
		o.resize = &resize{kind: kind, x0: x, y0: y, areaX: p.AreaX, areaW: p.AreaW,
			grabFromCenter: at.X - o.area.X - int(p.AreaX*float64(o.area.Width)), grabAboveBottom: o.blockBottom() - at.Y}
	}
	r := o.resize
	dx := float64((x - r.x0) / fw)
	switch kind {
	case moveArea:
		if d := mygo.Screen.DisplayNearestPoint(at); d.ID != o.display.ID {
			o.moveTo(d)
			p.Screen = screenName(d)
		}
		p.AreaX = float64(at.X-r.grabFromCenter-o.area.X) / float64(o.area.Width)
		p.Bottom = o.onScreen(at.Y+r.grabAboveBottom) - o.display.Bounds.Y
	case resizeLeft:
		p.AreaW = min(.95, max(.3, r.areaW-2*dx))
	case resizeRight:
		p.AreaW = min(.95, max(.3, r.areaW+2*dx))
	}
	p.AreaX = min(1-p.AreaW/2, max(p.AreaW/2, p.AreaX))
	c.After(16 * time.Millisecond)
}
