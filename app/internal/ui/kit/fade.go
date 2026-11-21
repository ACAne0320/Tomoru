package kit

import "github.com/egoist/mygo/ui"

func FadeScroll(c *ui.Context, s *ui.ScrollState, fn func()) ui.Element {
	more := float32(0)
	if s.MaxY-s.Y > 1 {
		more = 1
	}
	outer := ui.Column(c).Shrink(1).MinHeight(0)
	return outer.Children(func() {
		ui.Scroll(c).Shrink(1).MinHeight(0).Padding(0, Space(1)).Margin(0, -Space(1)).TrackScroll(s).Children(fn)
		paper := Colors(c).Paper
		fade := ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(Space(8)).PassThrough().
			LinearGradient(ui.LinearGradient{From: paper.Alpha(0), To: paper, Angle: 180})
		fade.Opacity(fade.AnimateTo("more", more, Fade))
	})
}
