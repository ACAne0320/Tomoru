package kit

import "github.com/egoist/mygo/ui"

type Choice struct {
	Key      any
	Head     string
	Shown    bool
	On       bool
	Off      bool
	Use      string
	Title    string
	Tag      string
	Accent   bool
	Meta     string
	Warn     bool
	Progress float64
}

func ChoiceList(c *ui.Context, label string, rows []Choice) (shown, used int) {
	p := Colors(c)
	shown, used = -1, -1
	ui.Column(c).Role(ui.RoleTabList).Label(label).Children(func() {
		for i, r := range rows {
			if r.Head != "" {
				if i > 0 {
					ui.Box(c.Key([2]any{"rule", r.Key})).Height(1).Margin(Space(2), Space(3), 0).Background(p.LineSoft)
				}
				ui.Text(c.Key([2]any{"head", r.Key}), r.Head).FontSize(TextCaption).FontWeight(WeightBold).TextColor(p.InkMuted).
					Padding(Space(2), Space(3), Space(1)).MaxLines(1)
			}
			show, use := choiceRow(c.Key(r.Key), r)
			if show {
				shown = i
			}
			if use {
				used = i
			}
		}
	})
	return shown, used
}

func choiceRow(c *ui.Context, r Choice) (show, use bool) {
	p := Colors(c)
	row := ui.Column(c).Padding(Space(3)).Gap(Space(2)).Radius(RadiusTile)
	face := p.Band.Alpha(0)
	switch {
	case r.Shown:
		face = p.Shown
	case row.Hovered():
		face = p.Paper
	}
	row.Background(row.AnimateColor("face", face, hover)).Children(func() {
		ui.Row(c).Gap(optGap).AlignItems(ui.Center).Children(func() {
			radio := ui.ButtonBase(c).Shrink(0).Radius(Pill(dotSize)).Role(ui.RoleRadio).Checked(r.On).Label(r.Use).Disabled(r.Off)
			if !r.Off {
				radio = radio.Cursor(ui.CursorPointer)
			}
			use = radio.Children(func() {
				if r.Off {
					dotOff(c)
				} else {
					Dot(c, r.On)
				}
			}).Clicked()
			tab := ui.ButtonBase(c).Grow(1).MinWidth(0).Justify(ui.Start).Role(ui.RoleTab).Selected(r.Shown).
				Background(ui.Transparent).Cursor(ui.CursorPointer)
			show = tab.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(Space(1)).Children(func() {
					ui.Row(c).Gap(Space(2)).AlignItems(ui.Center).Wrap().Children(func() {
						ui.Text(c, r.Title).FontSize(TextBody).FontWeight(WeightBold).TextColor(p.Ink)
						switch {
						case r.Tag != "" && r.Accent:
							AccentTag(c, r.Tag)
						case r.Tag != "":
							Tag(c, r.Tag)
						}
					})
					if r.Meta != "" {
						ink := p.InkMuted
						if r.Warn {
							ink = p.Danger
						}
						ui.Text(c, r.Meta).FontSize(TextCaption).TextColor(ink).MaxLines(2)
					}
				})
			}).Clicked()
		})
		if r.Progress >= 0 {
			Progress(c, r.Progress).Margin(0, 0, 0, dotSize+optGap)
		}
	})
	return show, use
}
