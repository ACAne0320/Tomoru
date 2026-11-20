// Package overlay is the subtitle overlay: a transparent window over the screen, which lets clicks
// through, showing the subtitles at the bottom. Hovered and unlocked, the subtitle area can be
// moved and resized, and shows a toolbar; locked, only the text shows, and the lock to unlock it
// while the pointer rests there.
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

// The overlay covers the work area, and reaches below it by below: transparent room for the
// newest line sliding in at the bottom. The subtitle block moves within it, up and down as
// sideways, so the window never moves or resizes: with many sentences it was as tall as the
// screen, and a window cannot go above the screen's top to bring its bottom up.
const below = 80

// Overlay is the subtitle overlay. It is used on the main thread, but as noted.
type Overlay struct {
	core    *core.Core
	nav     shared.Nav
	win     *mygo.Window
	display mygo.Display   // the display the overlay is on
	area    mygo.Rectangle // its work area, which the overlay covers

	locked atomic.Bool // Prefs.Locked, read by the pointer watcher
	// grabbable tells the pointer watcher that the block shows something to grab: subtitles, or the
	// frame of adjusting. An empty block does not answer the pointer passing by.
	grabbable atomic.Bool
	adjusting atomic.Bool // the frame shows: the pointer need not rest to grab it
	pointerOn atomic.Bool // the overlay has the pointer, where the system tells no other (pointerUnseen)
	mu        sync.Mutex  // guards block, which the pointer watcher reads
	block     ui.Rect     // the subtitle block in the overlay, with its toolbar, as last laid out
	lock      ui.Rect     // the lock above the locked block while it shows, as last laid out
	// woken wakes the pointer watcher, asleep while nothing shows to grab (wake).
	woken chan struct{}

	adjust  time.Time // the block shows its frame, to be moved, until then
	hover   bool
	peek    bool      // locked, the pointer rests on the block: the lock shows to unlock it
	hoverH  float32   // the block's height while hovered, fixed when it showed
	toast   time.Time // the "locked" note shows until then
	sample  time.Time // sample subtitles show until then
	resize  *resize
	textTop float32   // the top of the subtitles in the overlay, as last laid out
	glass   panes     // under the block and the toolbar while they show
	barW    float32   // the width of the toolbar's capsule, as last laid out
	notes   int       // the notices shown: one that shows after none did is a new pill
	noted   time.Time // when the notice showing appeared, zero while none does
	noteBox ui.Rect   // the notice's pill, as last laid out: the glass goes under it
}

// New makes the overlay's window, on the display the subtitles were put on, and shows it.
func New(k *core.Core, nav shared.Nav) *Overlay {
	o := &Overlay{core: k, nav: nav, woken: make(chan struct{}, 1)}
	o.display = displayNamed(mygo.Screen.Displays(), k.Preferences().Screen)
	o.area = o.display.WorkArea
	o.locked.Store(k.Preferences().Locked)
	area := o.area
	o.win = mygo.NewWindow(windowOptions(area, ui.View(o.View)))
	toolWindow(o.win)
	o.win.SetVisibleOnAllWorkspaces(true)
	o.win.SetContentProtection(k.Preferences().HideFromCapture)
	o.win.ShowInactive()
	go o.watchPointer()
	// A display plugged in or out, or its resolution changed: the subtitles go to theirs, or
	// to the primary display while theirs is gone.
	mygo.Screen.OnDisplaysChanged(func() {
		o.win.Update(func() { o.moveTo(displayNamed(mygo.Screen.Displays(), o.core.Preferences().Screen)) })
	})
	return o
}

// windowOptions makes the overlay's window: the work area, plus below for the newest line sliding
// in, with content drawn there, letting the mouse through until something shows to grab
// (watchPointer). DisableFullScreen keeps the window out of full screen itself; with
// SetVisibleOnAllWorkspaces, MyGo then makes it an auxiliary of other apps' full-screen Spaces
// (egoist/mygo#309), the way macOS documents for showing over them.
func windowOptions(area mygo.Rectangle, content mygo.Content) mygo.WindowOptions {
	return mygo.WindowOptions{
		Title: "Tomoru", X: area.X, Y: area.Y,
		Width: area.Width, Height: area.Height + below, Hidden: true,
		Frameless: true, Transparent: true, AlwaysOnTop: true, SkipTaskbar: true, IgnoreMouseEvents: true,
		DisableFullScreen: true, DisableShadow: true, DisableResize: true,
		Content: content,
	}
}

// displayNamed is the display of ds named name, or the primary one: the first, as
// mygo.Screen.Displays lists them.
func displayNamed(ds []mygo.Display, name string) mygo.Display {
	for _, d := range ds {
		if name != "" && d.Label == name {
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

// screenName is how Prefs.Screen names d: "" for the primary display, which the subtitles follow
// to whichever display becomes it.
func screenName(d mygo.Display) string {
	if d.Primary {
		return ""
	}
	return d.Label
}

// moveTo puts the overlay over d's work area, unless it is there.
func (o *Overlay) moveTo(d mygo.Display) {
	if d.ID == o.display.ID && d.WorkArea == o.area {
		return
	}
	o.display, o.area = d, d.WorkArea
	if o.win != nil {
		o.win.SetBounds(mygo.Rectangle{X: d.WorkArea.X, Y: d.WorkArea.Y, Width: d.WorkArea.Width, Height: d.WorkArea.Height + below})
	}
}

// Window is the overlay's window.
func (o *Overlay) Window() *mygo.Window { return o.win }

// Redraw redraws the overlay.
func (o *Overlay) Redraw() { o.win.Update(func() {}) }

// SetHideFromCapture keeps the subtitles out of screenshots and recordings, or not.
func (o *Overlay) SetHideFromCapture(hide bool) { o.win.SetContentProtection(hide) }

// ShowSample shows sample subtitles for d, to show where they go.
func (o *Overlay) ShowSample(d time.Duration) {
	o.sample = time.Now().Add(d)
	o.Redraw()
}

// SetLocked locks the subtitles, or unlocks them and shows where they are, to move them now.
func (o *Overlay) SetLocked(v bool) {
	o.core.UpdatePrefs(func(p *prefs.Prefs) { p.Locked = v })
	o.locked.Store(v)
	if v {
		o.hover, o.adjust = false, time.Time{}
		o.wake() // to stop taking the mouse
		o.toast = time.Now().Add(2600 * time.Millisecond)
	} else {
		o.peek = false
		o.StartAdjust()
	}
	o.Redraw()
}

// StartAdjust shows the subtitle block's frame for a while, even without subtitles, so that it
// can be found and moved.
func (o *Overlay) StartAdjust() {
	if o.core.Preferences().Locked {
		o.SetLocked(false)
		return
	}
	o.adjust = time.Now().Add(15 * time.Second)
	o.Redraw()
}

// blockBottom is the subtitle block's bottom edge on screen: where it was put, or 24 above the
// work area's bottom edge; with room for the block and its toolbar above it.
func (o *Overlay) blockBottom() int {
	area, p := o.area, o.core.Preferences()
	y := area.Y + area.Height - 24
	if p.Bottom != 0 {
		y = o.display.Bounds.Y + p.Bottom
	}
	return o.onScreen(y)
}

// onScreen is the block's bottom edge y kept within the work area, with room for the block and its
// toolbar above it.
func (o *Overlay) onScreen(y int) int {
	area := o.area
	return min(max(y, area.Y+int(subtitle.SlotHeight(o.core.Preferences(), 1, true))+60), area.Y+area.Height)
}

var (
	// What shows over the block fades in and out, and moves with it at once: animating its
	// position too, as a transition does unless told what to animate, it trailed the block dragged.
	fades = ui.ElementTransition{Colors: true, Duration: 160 * time.Millisecond, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
	// The subtitles come and go fading in place, all together: nothing slides in or out of view.
	appears = ui.ElementTransition{Colors: true, Duration: 240 * time.Millisecond, Enter: &ui.Motion{}, Exit: &ui.Motion{}}
)

// View draws the subtitles at the bottom of the subtitle area, and under them the block: as wide
// as the area, as tall as the subtitles (at least one sentence, so that the pointer can find it
// empty). One layer at a time: the closed-caption bands hug the text; hovered, the block takes the
// background, and the text on it drops its bands and stays within it, the toolbar floating above
// it, clear of the text.
func (o *Overlay) View(c *ui.Context) {
	k, p := o.core, o.core.Preferences()
	c.Root().Background(ui.Transparent)
	hover, peek := o.hover && !p.Locked, o.peek && p.Locked
	entries := subtitle.Visible(k.State, k.Since, p)
	idle := time.Since(k.Updated)
	hideAfter := k.HideAfter()
	show := k.Running && len(entries) > 0 && (hideAfter == 0 || idle < hideAfter)
	if show && hideAfter > 0 {
		c.After(hideAfter - idle) // hide once no new subtitles came for a while
	}
	sample := time.Until(o.sample)
	if !show && sample > 0 {
		show, entries = true, subtitle.Samples(p)
		c.After(sample)
	}
	adjust := time.Until(o.adjust)
	adjusting := adjust > 0 && !p.Locked
	if adjusting {
		c.After(adjust)
	}
	o.adjusting.Store(adjusting)
	if g := show || adjusting; o.grabbable.Swap(g) != g && g {
		o.wake()
	}
	ww, wh := c.Size()
	left, width := float32(p.AreaX-p.AreaW/2)*ww, float32(p.AreaW)*ww
	minH := subtitle.SlotHeight(p, 1, true) + 16 // the same size empty as with one sentence
	note := o.notice(c)
	if shown := !show && note.title != ""; !shown {
		o.noted, o.noteBox = time.Time{}, ui.Rect{}
	} else if o.noted.IsZero() {
		o.notes++
		o.noted = c.Now()
	}
	// The block's bottom, from the overlay's bottom edge.
	bottom := float32(o.area.Y + int(wh) - o.blockBottom())

	// The block is as tall as the text was in the last frame: the text lays itself out in a column
	// of its own, the overlay's height, so that what it adds at the bottom pushes the rest up, and
	// what leaves at the top moves nothing.
	h := minH
	if show && o.textTop != 0 {
		h = max(h, min(wh-bottom-o.textTop+8, wh-bottom-26)) // the oldest may be cut off at the top
	}
	// Hovered, the block keeps the height it took when it showed, so that the toolbar above it
	// stays under the pointer: the text goes on rolling up, cut off at its top edge. Locked, so
	// does the lock, over the text rolling on.
	switch {
	case !hover && !peek:
		o.hoverH, o.barW = 0, 0
	case o.hoverH == 0:
		o.hoverH = h
	}
	if hover || peek {
		h = o.hoverH
	}
	var hit ui.Rect // what the pointer finds: the block, and the toolbar above it while hovered
	column := ui.Box(c.Key("column")).Absolute().Left(left).Width(width).Top(0).Bottom(0)
	column.Children(func() {
		block := ui.Box(c.Key("block")).Absolute().Left(0).Right(0).Bottom(bottom).Height(h).Radius(14).
			Background(ui.Transparent).Border(1, ui.Transparent).Transition(subtitle.Follows)
		switch {
		case hover:
			block.Background(hoverShade(p)).Border(1, ui.RGBA(255, 255, 255, .12))
		case adjusting:
			block.Background(ui.RGBA(0, 0, 0, .22)).Border(1.5, ui.RGBA(255, 255, 255, .7))
		}
		if show {
			// Hovered, the text stays within the block: rolling up, it goes under its top edge, not
			// the toolbar. The column keeps its height, which the text's own takes nothing from.
			clip := float32(0)
			if hover {
				clip = wh - bottom - h // the block's top
			}
			ui.Box(c).Absolute().Left(0).Right(0).Top(clip).Bottom(0).Clip().Children(func() {
				ui.Column(c.Key("captions")).Absolute().Left(12).Right(12).Top(-clip).Bottom(bottom + 8).
					Justify(ui.End).Gap(8).Transition(appears).Children(func() {
					if sample > 0 {
						ui.Row(c).Justify(ui.Center).Margin(0, 0, 4, 0).Children(func() { pill(c, o.sampleNote()) })
					}
					bareBelow := float32(subtitle.NoBlock)
					if hover {
						bareBelow = clip
					}
					if top := subtitle.Draw(c, entries, p, 1, width-24, bareBelow); top != o.textTop {
						o.textTop = top
						c.After(16 * time.Millisecond) // the block follows
					}
				})
			})
		}
		// Over the text: what hovering and adjusting show, and the handles to grab, in the block's
		// place.
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
		// Above the block, or over its top where the overlay has no room above it.
		barBottom := min(bottom+h+8, wh-toolbarH)
		// The toolbar is built while it is gone too, unseen: a menu of it may still be open, and
		// the item chosen from it comes in a frame that builds its button. The scene menu reaches
		// below the block, where the pointer leaving it ends the hover before the choice.
		var bar, lock ui.Rect
		if !p.Locked {
			if r := o.toolbar(c, barBottom, hover).Bounds(); hover {
				bar = r
			}
		} else if r := o.unlock(c, barBottom, peek).Bounds(); peek {
			// The lock alone takes clicks, not the row it is centered in.
			bar, lock = r, ui.Rect{X: r.X + (r.W-o.barW)/2, Y: r.Y, W: o.barW, H: r.H}
		}
		if bar.H > 0 {
			hit.H += hit.Y - bar.Y
			hit.Y = bar.Y
		}
		// The notice floats on its own, centered where the toolbar comes, never in the block: with the
		// toolbar there, above it. It takes no clicks.
		noteBottom := barBottom + (toolbarH-30)/2
		if !show && note.title != "" {
			up := float32(0)
			if hover || peek {
				up = 1
			}
			lift := column.AnimateTo("lift", up, ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond}})
			noteBottom += lift * (toolbarH + 8)
			ui.Row(c).Absolute().Left(0).Right(0).Bottom(noteBottom).Justify(ui.Center).Children(func() {
				o.noteBox = o.notePill(c, note).Bounds()
			})
		}
		if until := time.Until(o.toast); until > 0 {
			c.After(until)
			ui.Row(c).Absolute().Left(0).Right(0).Bottom(bottom + h + 12).Justify(ui.Center).Transition(fades).Children(func() {
				pill(c, locales.LockedToast())
			})
		}
		// The glass under them, where they are this frame, not where they were laid out in the
		// last: dragged, the glass would trail them. The toolbar's capsule is centered over the
		// block, as wide as it was.
		var glass []pane
		if hover || adjusting {
			glass = append(glass, pane{ui.Rect{X: left, Y: wh - bottom - h, W: width, H: h}, 14})
		}
		// The notice's, as wide and tall as it was.
		if r := o.noteBox; r.W > 0 {
			glass = append(glass, pane{ui.Rect{X: left + (width-r.W)/2, Y: wh - noteBottom - r.H, W: r.W, H: r.H}, r.H / 2})
		}
		if (hover || peek) && o.barW > 0 {
			glass = append(glass, pane{ui.Rect{X: left + (width-o.barW)/2, Y: wh - barBottom - toolbarH, W: o.barW, H: toolbarH}, toolbarH / 2})
		}
		o.glass.set(o.win, wh, glass...)
		o.mu.Lock()
		o.block, o.lock = hit, lock
		o.mu.Unlock()
	})
	o.pointerOn.Store(column.Hovered()) // it takes the mouse only where the block is
}

// hoverShade is the hovered block's background, and the toolbar's over it: the block's shade, on
// glass where the system has it, which shows what plays under it, lighter: only for the text to
// read on.
func hoverShade(p prefs.Prefs) ui.Color {
	shade := float32(1)
	if hasGlass() {
		shade = .65
	}
	if p.Style == 1 {
		return ui.RGBA(10, 10, 14, .42*shade)
	}
	return ui.RGBA(0, 0, 0, max(.42, float32(p.BoxOpacity)/100)*shade)
}

// note is what floats over the subtitles while none show.
type note struct {
	mark   kit.MarkState
	marked bool // errors and a slow network say so in words alone
	title  string
	detail string // what to know beside it, as the languages while preparing
	since  time.Time
}

// notice is what floats over the subtitles while none show, about translating: that it is getting
// ready, that it listens or paused, for a while after, from wherever it was paused, and what went
// wrong, until it is tried again. The eye is there, and the menu bar may
// hide the icon behind the notch. Listening with nobody speaking shows nothing.
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
	// The notice's pill follows its words' width closely, without bouncing past it.
	fits = ui.Animation{Spring: ui.SpringOptions{Duration: 420 * time.Millisecond}}
	// One state's words at a time: the old ones lift away blurring, quickly; then the new ones rise
	// into place from a blur.
	swaps = ui.ElementTransition{
		Animation:    &ui.Animation{Spring: ui.SpringOptions{Duration: 380 * time.Millisecond}},
		ExitDuration: 90 * time.Millisecond, ExitEase: ui.EaseIn,
		Enter: &ui.Motion{Y: 5, Blur: 5}, Exit: &ui.Motion{Y: -5, Blur: 5},
		AfterExits: true,
	}
)

// notePill is the notice in a pill of its own, each time it appears: the mark first, then its
// words, apart from what to know beside them by weight and color alone. While it shows, its words
// change in place.
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
	pill := ui.Row(c.Key(appearance(o.notes))).MinHeight(30).Radius(15).Padding(0, 14, 0, 12)
	if !n.marked {
		pill.Padding(6, 14) // what went wrong can take more than a line
	}
	// A capsule as the toolbar is. Its place in its row moves with its width, so that it stays
	// centered over its glass as it grows; the row, not the pill, goes with the block. It comes and
	// goes only through a blur: the glass under it does not scale.
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

// appearance keys the notice's pill by how many showed before it, apart from the keys of the
// handles beside it.
type appearance int

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
