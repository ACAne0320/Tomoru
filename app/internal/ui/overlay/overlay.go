// Package overlay is the subtitle overlay: a transparent window over the screen, which lets clicks
// through, showing the subtitles at the bottom. Hovered and unlocked, the subtitle area can be
// moved and resized, and shows a toolbar; locked, only the text shows.
package overlay

import (
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
	mu        sync.Mutex  // guards block, which the pointer watcher reads
	block     ui.Rect     // the subtitle block in the overlay, with its toolbar, as last laid out

	adjust  time.Time // the block shows its frame, to be moved, until then
	hover   bool
	hoverH  float32   // the block's height while hovered, fixed when it showed
	toast   time.Time // the "locked" note shows until then
	sample  time.Time // sample subtitles show until then
	resize  *resize
	textTop float32 // the top of the subtitles in the overlay, as last laid out
	glass   panes   // under the block and the toolbar while they show
	barW    float32 // the width of the toolbar's capsule, as last laid out
}

// New makes the overlay's window, on the display the subtitles were put on, and shows it.
func New(k *core.Core, nav shared.Nav) *Overlay {
	o := &Overlay{core: k, nav: nav}
	o.display = displayNamed(mygo.Screen.Displays(), k.Preferences().Screen)
	o.area = o.display.WorkArea
	o.locked.Store(k.Preferences().Locked)
	area := o.area
	o.win = mygo.NewWindow(mygo.WindowOptions{
		Title: "Tomoru", X: area.X, Y: area.Y,
		Width: area.Width, Height: area.Height + below, Hidden: true,
		Frameless: true, Transparent: true, AlwaysOnTop: true, SkipTaskbar: true,
		// No DisableFullScreen: it keeps the window out of other apps' full screen (egoist/mygo#309).
		DisableShadow: true, DisableResize: true,
		Content: ui.View(o.View),
	})
	toolWindow(o.win)
	o.win.SetIgnoreMouseEvents(true)
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
		o.win.SetIgnoreMouseEvents(true)
		o.toast = time.Now().Add(2600 * time.Millisecond)
	} else {
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
	hover := o.hover && !p.Locked
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
	o.grabbable.Store(show || adjusting || k.Paused && !k.Running)
	o.adjusting.Store(adjusting)
	ww, wh := c.Size()
	left, width := float32(p.AreaX-p.AreaW/2)*ww, float32(p.AreaW)*ww
	minH := subtitle.SlotHeight(p, 1, true) + 16 // the same size empty as with one sentence
	note := o.notice(c)
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
	// stays under the pointer: the text goes on rolling up, cut off at its top edge.
	switch {
	case !hover:
		o.hoverH, o.barW = 0, 0
	case o.hoverH == 0:
		o.hoverH = h
	}
	if hover {
		h = o.hoverH
	}
	var hit ui.Rect // what the pointer finds: the block, and the toolbar above it while hovered
	ui.Box(c).Absolute().Left(left).Width(width).Top(0).Bottom(0).Children(func() {
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
			case note != "":
				pill(c.Key("notice"), note).AlignSelf(subtitle.Align(p)).Margin(0, 0, 10, 0).Transition(fades).
					ContentTransition(ui.ContentTransition{}) // from getting ready to listening, say
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
		if !p.Locked {
			if r := o.toolbar(c, barBottom, hover).Bounds(); hover && r.H > 0 {
				hit.H += hit.Y - r.Y
				hit.Y = r.Y
			}
		}
		if until := time.Until(o.toast); until > 0 {
			c.After(until)
			ui.Row(c).Absolute().Left(0).Right(0).Bottom(bottom + h + 12).Justify(ui.Center).Transition(fades).Children(func() {
				pill(c, shared.OnWindows(locales.LockedToast(), locales.LockedToastWin()))
			})
		}
		// The glass under them, where they are this frame, not where they were laid out in the
		// last: dragged, the glass would trail them. The toolbar's capsule is centered over the
		// block, as wide as it was.
		var glass []pane
		if hover || adjusting {
			glass = append(glass, pane{ui.Rect{X: left, Y: wh - bottom - h, W: width, H: h}, 14})
		}
		if hover && o.barW > 0 {
			glass = append(glass, pane{ui.Rect{X: left + (width-o.barW)/2, Y: wh - barBottom - toolbarH, W: o.barW, H: toolbarH}, toolbarH / 2})
		}
		o.glass.set(o.win, wh, glass...)
		o.mu.Lock()
		o.block = hit
		o.mu.Unlock()
	})
}

// notice is what shows where the subtitles go, while none do, about translating: that it is
// getting ready, that it listens or stopped, for a while after, and what went wrong, until it is
// tried again. The eye is there, and the menu bar may hide the icon behind the notch. Listening
// with nobody speaking shows nothing.
// hoverShade is the hovered block's background, and the toolbar's over it: the closed-caption
// bands, now the block's, or for immersive subtitles a lighter one, only to grab the block by. On
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
	return ui.RGBA(0, 0, 0, .62*shade)
}

func (o *Overlay) notice(c *ui.Context) string {
	const lasts = 4 * time.Second
	k := o.core
	langs, keys := shared.LangsOf(k.Preferences()), kit.ShortcutText(k.Preferences().Shortcut)
	recent := time.Since(k.NoteAt) < lasts
	if recent {
		c.After(lasts - time.Since(k.NoteAt))
	}
	switch {
	case k.Err != nil:
		return locales.NoticeRetry(shared.ErrorText(k.Err), keys)
	case k.Paused && !k.Running:
		return locales.NoticePaused(keys)
	case k.Running && k.State.Phase == "downloading":
		return locales.NoticeStatus(locales.StatusDownloading(), langs)
	case k.Running && k.State.Phase == "loading":
		return locales.NoticeStatus(locales.StatusLoadingModel(), langs)
	case k.Running && k.State.Phase != "listening":
		return locales.NoticeStatus(locales.StatusPreparing(), langs)
	case k.Running && k.State.Slow:
		return locales.NoticeSlowNetwork()
	case recent && k.Running:
		return locales.NoticeListening(langs, keys)
	case recent && !k.Started:
		if !k.Preferences().Translates() {
			return locales.NoticeReadySubtitles(keys)
		}
		return locales.NoticeReady(keys)
	case recent:
		if !k.Preferences().Translates() {
			return locales.NoticeStoppedSubtitles(keys)
		}
		return locales.NoticeStopped(keys)
	}
	return ""
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
