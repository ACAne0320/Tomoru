package overlay

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/ui/subtitle"
	"tomoru/locales"
)

const below = 80

type Overlay struct {
	core    *core.Core
	nav     shared.Nav
	win     *mygo.Window
	display mygo.Display
	area    mygo.Rectangle

	pointerShared

	adjustUntil   time.Time
	hover         bool
	showingUnlock bool
	hoverH        float32
	toastUntil    time.Time
	sampleUntil   time.Time
	resize        *resize
	textTop       float32
	barW          float32
	noticeCount   int
	noted         time.Time

	fit
}

type pointerShared struct {
	locked    atomic.Bool
	grabbable atomic.Bool
	adjusting atomic.Bool
	pointerOn atomic.Bool
	woken     chan struct{}

	mu    sync.Mutex
	block ui.Rect
	lock  ui.Rect
}

func New(k *core.Core, nav shared.Nav) *Overlay {
	o := &Overlay{core: k, nav: nav, pointerShared: pointerShared{woken: make(chan struct{}, 1)}}
	o.display = displayNamed(mygo.Screen.Displays(), k.Preferences().Screen)
	o.area = o.display.WorkArea
	o.locked.Store(k.Preferences().Locked)
	o.frame = o.whole()
	o.asked = placement{frame: o.frame}
	o.win = mygo.NewWindow(windowOptions(o.area, ui.View(o.View)))
	toolWindow(o.win)
	looksActive(o.win)
	o.win.SetVisibleOnAllWorkspaces(true)
	o.win.SetContentProtection(k.Preferences().HideFromCapture)
	o.win.ShowInactive()
	go o.watchPointer()
	mygo.Screen.OnDisplaysChanged(func() {
		o.win.Update(func() { o.moveTo(displayNamed(mygo.Screen.Displays(), o.core.Preferences().Screen)) })
	})
	return o
}

func windowOptions(area mygo.Rectangle, content mygo.Content) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title: "Tomoru", X: area.X, Y: area.Y,
		Width: area.Width, Height: area.Height + below, Hidden: true,
		Frameless: true, Transparent: true, AlwaysOnTop: true, SkipTaskbar: true, IgnoreMouseEvents: true,
		DisableFullScreen: true, DisableShadow: true, DisableResize: true,
		Content: content,
	}
}

func displayNamed(ds []mygo.Display, name string) mygo.Display {
	for _, d := range ds {
		if name != prefs.ScreenPrimary && d.Label == name {
			return d
		}
	}
	for _, d := range ds {
		if d.Primary {
			return d
		}
	}
	if len(ds) > 0 {
		return ds[0]
	}
	return mygo.Screen.PrimaryDisplay()
}

func screenName(d mygo.Display) string {
	if d.Primary {
		return prefs.ScreenPrimary
	}
	return d.Label
}

func (o *Overlay) moveTo(d mygo.Display) {
	if d.ID == o.display.ID && d.WorkArea == o.area {
		return
	}
	o.display, o.area = d, d.WorkArea
	o.frame, o.fitted = o.whole(), false
	o.asked = placement{frame: o.frame, hidden: o.hidden}
	if o.win != nil {
		o.win.SetBounds(o.frame)
	}
}

func (o *Overlay) Window() *mygo.Window { return o.win }

func (o *Overlay) Redraw() { o.win.Update(func() {}) }

func (o *Overlay) SetHideFromCapture(hide bool) { o.win.SetContentProtection(hide) }

func (o *Overlay) ShowSample(d time.Duration) {
	o.sampleUntil = time.Now().Add(d)
	o.Redraw()
}

func (o *Overlay) SetLocked(v bool) {
	o.core.UpdatePrefs(func(p *prefs.Prefs) { p.Locked = v })
	o.locked.Store(v)
	if v {
		o.hover, o.adjustUntil = false, time.Time{}
		o.wake()
		o.toastUntil = time.Now().Add(2600 * time.Millisecond)
	} else {
		o.showingUnlock = false
		o.StartAdjust()
	}
	o.Redraw()
}

func (o *Overlay) StartAdjust() {
	if o.core.Preferences().Locked {
		o.SetLocked(false)
		return
	}
	o.adjustUntil = time.Now().Add(15 * time.Second)
	o.Redraw()
}

func (o *Overlay) blockBottom() int {
	area, p := o.area, o.core.Preferences()
	y := area.Y + area.Height - 24
	if p.Bottom != 0 {
		y = o.display.Bounds.Y + p.Bottom
	}
	return o.onScreen(y)
}

func (o *Overlay) onScreen(y int) int {
	area := o.area
	return min(max(y, area.Y+int(subtitle.SlotHeight(o.core.Preferences(), 1, true))+60), area.Y+area.Height)
}

var (
	fades   = ui.ElementTransition{Colors: true, Duration: 160 * time.Millisecond, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
	appears = ui.ElementTransition{Colors: true, Duration: 240 * time.Millisecond, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
)

func (o *Overlay) View(c *ui.Context) {
	k, p := o.core, o.core.Preferences()
	c.Root().Background(ui.Transparent)
	hover, peek := o.hover && !p.Locked, o.showingUnlock && p.Locked
	entries := subtitle.Visible(k.State, k.Since, p)
	now := c.Now()
	idle := now.Sub(k.Updated)
	hideAfter := k.HideAfter()
	show := k.Running && len(entries) > 0 && (hideAfter == 0 || idle < hideAfter)
	if show && hideAfter > 0 {
		c.After(hideAfter - idle)
	}
	sample := o.sampleUntil.Sub(now)
	if !show && sample > 0 {
		show, entries = true, subtitle.Samples(p)
		c.After(sample)
	}
	adjust := o.adjustUntil.Sub(now)
	adjusting := adjust > 0 && !p.Locked
	if adjusting {
		c.After(adjust)
	}
	o.adjusting.Store(adjusting)
	if g := show || adjusting; o.grabbable.Swap(g) != g && g {
		o.wake()
	}
	o.moveAhead(p)
	ww, wh := c.Size()
	fw, ox, oy := o.canvas(ww)
	canvasLeft, width := float32(p.AreaX-p.AreaW/2)*fw, float32(p.AreaW)*fw
	left := canvasLeft - ox
	minH := subtitle.SlotHeight(p, 1, true) + 16
	note := o.notice(c)
	if shown := !show && note.title != ""; !shown {
		o.noted = time.Time{}
	} else if o.noted.IsZero() {
		o.noticeCount++
		o.noted = c.Now()
	}
	winTop, blockBottom := float32(o.area.Y+oy), float32(o.blockBottom())
	bottom := winTop + wh - blockBottom
	canvasH := winTop + wh - float32(o.area.Y)

	h := minH
	if show && o.textTop != 0 {
		h = max(h, min(blockBottom-o.textTop+8, canvasH-bottom-26))
	}
	switch {
	case !hover && !peek:
		o.hoverH, o.barW = 0, 0
	case o.hoverH == 0:
		o.hoverH = h
	}
	if hover || peek {
		h = o.hoverH
	}
	var hit ui.Rect
	var drawn span
	toast := false
	column := ui.Box(c.Key("column")).Absolute().Left(left).Width(width).Top(0).Bottom(0)
	column.Children(func() {
		if hasGlass() && (hover || adjusting) {
			ui.Box(c.Key("glass")).Absolute().Left(0).Right(0).Bottom(bottom).Height(h).Radius(14).Material(onGlass{})
		}
		block := ui.Box(c.Key("block")).Absolute().Left(0).Right(0).Bottom(bottom).Height(h).Radius(14).
			Background(ui.Transparent).Border(1, ui.Transparent).Transition(subtitle.Follows)
		switch {
		case hover:
			block.Background(hoverShade(p)).Border(1, ui.RGBA(255, 255, 255, .12))
		case adjusting:
			block.Background(ui.RGBA(0, 0, 0, .22)).Border(1.5, ui.RGBA(255, 255, 255, .7))
		}
		if show {
			clip := float32(0)
			if hover {
				clip = wh - bottom - h
			}
			ui.Box(c).Absolute().Left(0).Right(0).Top(clip).Bottom(0).Clip().Children(func() {
				ui.Column(c.Key("captions")).Absolute().Left(12).Right(12).Bottom(bottom + 8).Height(canvasH).
					Justify(ui.End).Gap(8).Transition(appears).Children(func() {
					if sample > 0 {
						r := ui.Row(c).Justify(ui.Center).Margin(0, 0, 4, 0)
						drawn.add(o.top(r, blockBottom-h-40), blockBottom)
						r.Children(func() { pill(c, o.sampleNote()) })
					}
					bareBelow := float32(subtitle.NoBlock)
					if hover {
						bareBelow = clip
					}
					o.fitToText(c, subtitle.Draw(c, entries, p, 1, width-24, bareBelow))
				})
			})
		}
		ui.Column(c.Key("controls")).Absolute().Left(0).Right(0).Bottom(bottom).Height(h).
			Justify(ui.End).AlignItems(ui.Center).Padding(8, 12).Children(func() {
			switch {
			case show:
			case hover:
				ui.Text(c.Key("hint"), locales.DragHint()).FontSize(14).TextColor(ui.RGBA(255, 255, 255, .85)).
					Margin(0, 0, 8, 0).Transition(fades)
			case adjusting:
				ui.Text(c.Key("hint"), locales.AdjustHint()).FontSize(14).TextColor(ui.RGBA(255, 255, 255, .85)).
					Margin(0, 0, 8, 0).Transition(fades)
			}
			if hover {
				drag := ui.ButtonBase(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Label(locales.DragSubtitles()).Cursor(ui.CursorMove)
				o.follow(c, drag, moveArea)
				o.handles(c)
			}
		})
		hit = block.Bounds()
		barBottom := min(bottom+h+8, canvasH-toolbarH)
		var bar, lock ui.Rect
		if !p.Locked {
			if r := o.toolbar(c, barBottom, hover).Bounds(); hover {
				bar = r
			}
		} else if r := o.unlock(c, barBottom, peek).Bounds(); peek {
			bar, lock = r, ui.Rect{X: r.X + (r.W-o.barW)/2, Y: r.Y, W: o.barW, H: r.H}
		}
		if bar.H > 0 {
			hit.H += hit.Y - bar.Y
			hit.Y = bar.Y
		}
		if hover || peek {
			o.chromeUntil = now.Add(fitLinger)
		}
		if until := o.chromeUntil.Sub(now); until > 0 {
			c.After(until)
			drawn.add(winTop+wh-barBottom-toolbarH, blockBottom)
		}
		noteBottom := barBottom + (toolbarH-30)/2
		if !show && note.title != "" {
			up := float32(0)
			if hover || peek {
				up = 1
			}
			lift := column.AnimateTo("lift", up, ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond}})
			noteBottom += lift * (toolbarH + 8)
			r := ui.Row(c).Absolute().Left(0).Right(0).Bottom(noteBottom).Justify(ui.Center)
			drawn.add(o.top(r, winTop+wh-noteBottom-30), winTop+wh-noteBottom)
			r.Children(func() { o.notePill(c, note) })
		}
		toast = o.toastUntil.Sub(now) > 0
		if until := o.toastUntil.Sub(now); toast {
			drawn.add(blockBottom-h-12-30, blockBottom-h-12)
			c.After(until)
			ui.Row(c).Absolute().Left(0).Right(0).Bottom(bottom + h + 12).Justify(ui.Center).Transition(fades).Children(func() {
				pill(c, locales.LockedToast())
			})
		}
		o.mu.Lock()
		o.block, o.lock = hit, lock
		o.mu.Unlock()
	})
	o.pointerOn.Store(column.Hovered())
	if show || adjusting {
		drawn.add(blockBottom-h, blockBottom)
	}
	if drawn.set {
		o.reach = span{top: drawn.top - blockBottom, bottom: drawn.bottom - blockBottom, set: true}
	}
	o.place(c, show || adjusting || hover || peek || note.title != "" || toast,
		ui.Rect{X: canvasLeft, Y: drawn.top, W: width, H: drawn.bottom - drawn.top})
	o.lastTop = winTop
}

func (o *Overlay) fitToText(c *ui.Context, top float32) {
	if top != 0 {
		top += o.lastTop
	}
	if top != o.textTop {
		o.textTop = top
		c.After(16 * time.Millisecond)
	}
}

func hoverShade(p prefs.Prefs) ui.Color {
	shade := float32(1)
	if hasGlass() {
		shade = .65
	}
	if p.Style == prefs.StyleImmersive {
		return ui.RGBA(10, 10, 14, .42*shade)
	}
	return ui.RGBA(0, 0, 0, max(.42, float32(p.BoxOpacity)/100)*shade)
}

type note struct {
	mark   kit.MarkState
	marked bool
	title  string
	detail string
	since  time.Time
}

func (o *Overlay) notice(c *ui.Context) note {
	const lasts = 4 * time.Second
	k := o.core
	langs := shared.LangsOf(k.Preferences())
	since := c.Now().Sub(k.NoteAt)
	recent := since < lasts
	if recent {
		c.After(lasts - since)
	}
	mark := func(m kit.MarkState, title, detail string) note {
		return note{mark: m, marked: true, title: title, detail: detail, since: k.NoteAt}
	}
	switch {
	case k.Err != nil:
		return note{title: locales.NoticeRetry(shared.ErrorText(k.Err), kit.ShortcutText(k.Preferences().Shortcut))}
	case k.Running && k.State.Phase == "downloading":
		return mark(kit.MarkWaiting, locales.StatusDownloading(), langs)
	case k.Running && k.State.Phase == "loading":
		return mark(kit.MarkWaiting, locales.StatusLoadingModel(), langs)
	case k.Running && k.State.Phase != "listening":
		return mark(kit.MarkWaiting, locales.StatusPreparing(), langs)
	case k.Running && k.State.Slow:
		return note{title: locales.NoticeSlowNetwork()}
	case recent && k.Running:
		return mark(kit.MarkLive, locales.NoticeListening(), "")
	case recent && !k.Started:
		return mark(kit.MarkReady, locales.NoticeReady(), "")
	case recent:
		return mark(kit.MarkPaused, locales.NoticePaused(), "")
	}
	return note{}
}

var (
	fits  = ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond}}
	swaps = ui.ElementTransition{
		Animation:    &ui.Animation{Spring: ui.SpringOptions{Duration: 380 * time.Millisecond}},
		ExitDuration: 90 * time.Millisecond, ExitEase: ui.EaseIn,
		Enter: &ui.Motion{Y: 5, Blur: 5}, Exit: &ui.Motion{Y: -5, Blur: 5},
		AfterExits: true,
	}
)

func (o *Overlay) notePill(c *ui.Context, n note) ui.Element {
	const wordsAfter = 320 * time.Millisecond
	words := c.Preferences().ReduceMotion || !n.marked
	if !words {
		if d := c.Now().Sub(o.noted); d < wordsAfter {
			c.After(wordsAfter - d)
		} else {
			words = true
		}
	}
	pill := ui.Row(c.Key(noticeKey(o.noticeCount))).MinHeight(30).Radius(15).Padding(0, 14, 0, 12)
	if !n.marked {
		pill.Padding(6, 14)
	}
	return capsule(o.core.Preferences(), pill).Gap(10).AlignItems(ui.Center).Clip().
		Transition(ui.ElementTransition{Animation: &fits, Position: true, Size: true, Enter: &ui.Motion{Blur: 4}, Exit: &ui.Motion{Blur: 4}}).
		Children(func() {
			if n.marked {
				kit.StatusMark(c, n.mark, n.since, o.level, ui.RGB(255, 255, 255))
			}
			if !words {
				return
			}
			ui.Row(c.Key(n.title)).Gap(8).AlignItems(ui.Center).Transition(swaps).Children(func() {
				title := ui.Text(c, n.title).FontSize(12.5).FontWeight(600).TextColor(ui.RGB(255, 255, 255))
				if n.marked {
					title.NoWrap()
				}
				if n.detail != "" {
					ui.Text(c, n.detail).NoWrap().FontSize(12.5).TextColor(ui.RGBA(255, 255, 255, .58))
				}
			})
		})
}

type noticeKey int

func (o *Overlay) level(t time.Time) float32 {
	if !o.core.State.Voice {
		return 0
	}
	s := float64(t.UnixMilli()%1_000_000) / 1000
	v := .5 + .5*math.Sin(s*4.3)*math.Sin(s*1.7+.6)*math.Sin(s*.9+1.3)
	return .25 + .6*float32(v)
}

func (o *Overlay) sampleNote() string {
	if o.core.Running {
		return locales.SampleNoteRunning()
	}
	keys := kit.ShortcutText(o.core.Preferences().Shortcut)
	if !o.core.Preferences().Translates() {
		return locales.SampleNoteIdleSubtitles(keys)
	}
	return locales.SampleNoteIdle(keys)
}

func pill(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontSize(12).TextColor(ui.RGBA(255, 255, 255, 1)).Background(ui.RGBA(0, 0, 0, .55)).Radius(99).Padding(4, 12)
}
