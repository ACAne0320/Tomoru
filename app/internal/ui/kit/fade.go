package kit

import "github.com/egoist/mygo/ui"

var focusRingRoom = Space(1)

func roomForFocusRings(clip ui.Element) ui.Element {
	return clip.Padding(0, focusRingRoom).Margin(0, -focusRingRoom)
}

func FadeScroll(c *ui.Context, s *ui.ScrollState, fn func()) ui.Element {
	more := float32(0)
	if s.MaxY-s.Y > 1 {
		more = 1
	}
	outer := ui.Column(c).Shrink(1).MinHeight(0)
	return outer.Children(func() {
		roomForFocusRings(ui.Scroll(c).Shrink(1).MinHeight(0)).TrackScroll(s).Children(fn)
		paper := Colors(c).Paper
		fade := ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(Space(8)).PassThrough().
			LinearGradient(ui.LinearGradient{From: paper.Alpha(0), To: paper, Angle: 180})
		fade.Opacity(fade.AnimateTo("more", more, Fade))
	})
}
