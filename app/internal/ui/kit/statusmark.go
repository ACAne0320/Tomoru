package kit

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// MarkState is what a StatusMark shows.
type MarkState int

const (
	MarkReady   MarkState = iota // two eyes: opening, blinking, looking aside and back
	MarkWaiting                  // five dots, a slow light passing along them
	MarkLive                     // five bars, as loud as what is heard, from the middle out
	MarkPaused                   // the two bars of a pause
)

// The mark is five bars: dots at rest, as tall as the mark at the loudest.
const (
	markBarW, markBarGap = 2.6, 2.4
	markDotH, markMaxH   = 2.6, 15
	markW, markH         = 5*markBarW + 4*markBarGap, 16
)

var (
	// The middle bar hears now, the others a little before, outward, and reach less high.
	markDelays = [5]time.Duration{140 * time.Millisecond, 70 * time.Millisecond, 0, 70 * time.Millisecond, 140 * time.Millisecond}
	markReach  = [5]float32{.55, .82, 1, .82, .55}
	// How high each bar swells as listening breathes in.
	markBreath = [5]float32{.35, .7, 1, .7, .35}
)

// markMotion is what a StatusMark keeps from frame to frame: the state it showed and how often
// its dots spread, which building changes, and the bars as drawn, which drawing does.
type markMotion struct {
	was     MarkState
	spreads uint64
	last    time.Time  // when the bars were last drawn
	h       [5]float32 // the bars' heights, from a dot (0) to the mark's (1)
}

// StatusMark is a small mark, in col, of what listening does, to tell at a glance: Tomoru's eyes
// when she is ready, dots while she gets ready, bars moving with what she hears, a pause. since is
// when s began; level is how loud what was heard at a time is, from 0 to 1. Appearing, its dots
// spread from the middle; it moves from one state to the next rather than switching. With Reduce
// Motion it holds still: the eyes open, the dots and the pause at rest, and listening five dots.
func StatusMark(c *ui.Context, s MarkState, since time.Time, level func(time.Time) float32, col ui.Color) ui.Element {
	still := c.Preferences().ReduceMotion
	box := ui.Box(c).Size(markW, markH).Shrink(0)
	m := ui.Local(box, "motion", func() markMotion { return markMotion{was: s} })
	if s != m.was {
		if s == MarkWaiting && m.was != MarkReady {
			m.spreads++ // from the eyes, the dots are out already
		}
		m.was = s
	}
	spread := box.Timeline("spread", ui.TimelineOptions{Run: m.spreads}).
		Tween(ui.Tween{From: 0, To: 1, Duration: 520 * time.Millisecond, Ease: ui.CubicBezier(.2, .9, .25, 1.15)})
	// Ready, the second and fourth dots are Tomoru's eyes, which open as she shows; going on, they
	// become dots among the others again.
	eyes := box.AnimateTo("eyes", b2f(s == MarkReady), ui.Animation{Spring: ui.SpringOptions{Duration: 460 * time.Millisecond}})
	pause := box.AnimateTo("pause", b2f(s == MarkPaused), ui.Animation{Spring: ui.SpringOptions{Duration: 520 * time.Millisecond, Bounce: .12}})
	lit := box.AnimateTo("lit", b2f(s == MarkLive), ui.Animation{Duration: 300 * time.Millisecond, NonMotion: !still})
	return box.Draw(func(p *ui.Painter, r ui.Rect) {
		now := p.Now()
		var hs [5]float32
		if !still {
			if m.follow(now, level, s != MarkLive) {
				p.AnimationFrame()
			}
			hs = m.h
			// Listening begins with a breath in: the bars swell from the middle out and settle,
			// before what is heard moves them.
			if d := now.Sub(since); s == MarkLive && d < breathIn {
				b := breath(d)
				for i, k := range markBreath {
					hs[i] = max(hs[i], b*k*.62)
				}
				p.AnimationFrame()
			}
		}
		open, glance := float32(1), float32(0)
		if eyes > 0 && !still {
			var next time.Duration
			open, glance, next = look(now.Sub(since))
			switch {
			case next == 0 || eyes < 1:
				p.AnimationFrame()
			case next > 0:
				p.After(next)
			}
		}
		t := float64(now.UnixMilli()%1_000_000) / 1000
		mid := r.X + r.W/2
		for i := range 5 {
			x := mid + (float32(i)-2)*(markBarW+markBarGap)*spread
			h := markDotH + (markMaxH-markDotH)*hs[i]
			w := float32(markBarW)
			a := float32(.95)
			switch {
			case still:
			case s == MarkWaiting || lit < 1 && s == MarkLive:
				// A slow light passes along the dots, until listening lights them all.
				sweep := .45 + .5*float32(math.Pow(.5+.5*math.Sin(t*3.2-float64(i)*.7), 2))
				a = lerp(sweep, .95, lit)
				p.AnimationFrameRate(30)
			case s == MarkLive && hs[2] < .02 && pause < .01:
				// Quiet: the dots breathe together, slowly.
				a = .62 + .3*float32(.5+.5*math.Sin(t*2*math.Pi/3.6))
				p.AnimationFrameRate(30)
			}
			if pause > 0 {
				// The outer pairs close in on the two bars of a pause; the middle one goes.
				side := float32(-1)
				if i > 2 {
					side = 1
				}
				px, ph, pa := mid+side*3.4, float32(11), a
				if i == 2 {
					px, ph, pa = mid, 0, 0
				}
				x, h, w, a = lerp(x, px, pause), lerp(h, ph, pause), lerp(w, 3.2, pause), lerp(a, pa, min(1, pause))
			}
			if eyes > 0 {
				// The eyes, taller and rounder, open, blink and look aside; the other dots are not
				// there.
				if i == 1 || i == 3 {
					ex := mid + (float32(i)-2)*3.6 + glance
					x, w, h = lerp(x, ex, eyes), lerp(w, 3.3, eyes), lerp(h, lerp(.8, 5.6, open), eyes)
				} else {
					a *= 1 - eyes
				}
			}
			reach := spread
			if eyes > 0 {
				reach = max(spread, eyes)
			}
			if fill := col.Alpha(a * min(1, reach*1.6)); h > .2 && fill.A > 2 {
				p.Fill(ui.Rect{X: x - w/2, Y: r.Y + (r.H-h)/2, W: w, H: h}, fill, w/2)
			}
		}
	})
}

// follow moves the bars toward how loud what was heard is, rising quickly and falling slowly, as a
// breath does, or lets them go while quiet. It tells whether they still move.
func (m *markMotion) follow(now time.Time, level func(time.Time) float32, quiet bool) bool {
	dt := float32(0)
	if !m.last.IsZero() {
		dt = float32(min(now.Sub(m.last), 50*time.Millisecond).Seconds())
	}
	m.last = now
	moving := false
	for i := range m.h {
		target, tau := float32(0), float32(.07)
		if !quiet {
			target, tau = float32(math.Pow(float64(level(now.Add(-markDelays[i]))), 1.3))*markReach[i], .16
		}
		if target > m.h[i] {
			tau = .045
		}
		m.h[i] += (target - m.h[i]) * (1 - float32(math.Exp(float64(-dt/tau))))
		if math.Abs(float64(target-m.h[i])) > .002 || target > .002 {
			moving = true
		}
	}
	return moving
}

// Listening's breath in swells for swell, then settles until breathIn.
const (
	swell    = 260 * time.Millisecond
	breathIn = swell + 420*time.Millisecond
)

// breath is how far the bars have swollen at d into listening, from 0 to 1 and back.
func breath(d time.Duration) float32 {
	in := ui.CubicBezier(.3, 0, .2, 1)(clamp01(float32(d) / float32(swell)))
	out := ui.CubicBezier(.4, 0, .2, 1)(clamp01(float32(d-swell) / float32(breathIn-swell)))
	return in * (1 - out)
}

// look is Tomoru's eyes at t into being ready: how open they are, how far aside they look, and how
// long until they next move (0 while they move, -1 when they never will). She wakes, blinks, looks
// left and right, and blinks twice.
func look(t time.Duration) (open, glance float32, next time.Duration) {
	ms := float32(t.Milliseconds())
	open = smoothstep((ms - 150) / 300)
	for _, at := range []float32{1000, 3300, 3520} { // blinks: shut in 70 ms, open in 120
		if d := ms - at; d >= 0 && d < 190 {
			if d < 70 {
				open *= 1 - .85*smoothstep(d/70)
			} else {
				open *= .15 + .85*smoothstep((d-70)/120)
			}
		}
	}
	from := float32(0)
	for _, g := range []struct{ at, to float32 }{{1600, -1.4}, {2300, 1.4}, {2900, 0}} {
		if ms >= g.at {
			glance = lerp(from, g.to, smoothstep((ms-g.at)/220))
		}
		from = g.to
	}
	moves := []float32{150, 450, 1000, 1190, 1600, 1820, 2300, 2520, 2900, 3120, 3300, 3710}
	for i := 0; i < len(moves); i += 2 {
		if ms >= moves[i] && ms < moves[i+1] {
			return open, glance, 0
		}
		if ms < moves[i] {
			return open, glance, time.Duration(moves[i]-ms) * time.Millisecond
		}
	}
	return open, glance, -1
}

func smoothstep(x float32) float32 {
	x = clamp01(x)
	return x * x * (3 - 2*x)
}

func clamp01(x float32) float32 { return max(0, min(1, x)) }

func lerp(a, b, k float32) float32 { return a + (b-a)*k }
