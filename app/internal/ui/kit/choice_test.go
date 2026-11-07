package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestChoiceList shows a row clicked on its title, uses it clicked on its radio, and does nothing
// with the radio of a row that can't be used; in a SplitCard, beside what it shows.
func TestChoiceList(t *testing.T) {
	rows := []Choice{
		{Key: "a", Head: "On device", Title: "SenseVoice", Use: "Use SenseVoice", On: true, Meta: "Small", Progress: -1},
		{Key: "b", Title: "Qwen3-ASR", Use: "Use Qwen3-ASR", Off: true, Meta: "2 sizes · not downloaded", Progress: .4},
		{Key: "c", Head: "Cloud", Title: "OpenAI", Use: "Use OpenAI", Progress: -1},
	}
	shown, used := 0, -1
	tt := ui.NewTester(func(c *ui.Context) {
		Paper(c)
		ui.Column(c).Fill().Padding(Space(4)).Children(func() {
			SplitCard(c).Children(func() {
				SplitList(c, func() {
					for i := range rows {
						rows[i].Shown = i == shown
					}
					s, u := ChoiceList(c, "Speech recognition", rows)
					if s >= 0 {
						shown = s
					}
					if u >= 0 {
						used = u
					}
				})
				SplitDetail(c, func() { ui.Text(c, "Detail of "+rows[shown].Title) })
			})
		})
	}, 640, 480)
	tt.Frame()
	if err := tt.Click("Qwen3-ASR"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if shown != 1 || used != -1 || !tt.HasText("Detail of Qwen3-ASR") {
		t.Fatalf("shown %d, used %d, texts %q", shown, used, tt.Texts())
	}
	if err := tt.Click("Use Qwen3-ASR"); err != nil {
		t.Fatal(err)
	}
	if used != -1 {
		t.Errorf("used %d, which can't be", used)
	}
	if err := tt.Click("Use OpenAI"); err != nil {
		t.Fatal(err)
	}
	if used != 2 || shown != 1 {
		t.Errorf("clicking a radio: used %d, shown %d", used, shown)
	}
}

// TestOptionOff can't choose a row that is Off, and chooses the others.
func TestOptionOff(t *testing.T) {
	clicked := -2
	tt := ui.NewTester(func(c *ui.Context) {
		Paper(c)
		OptionsCard(c).Children(func() {
			clicked = max(clicked, Options(c, []Option{
				{Key: "2b", Title: "2B", Done: "Downloaded", Meta: "1.6 GB"},
				{Key: "9b", Title: "9B", Off: true, Warn: true, Meta: "Needs 16 GB of memory"},
			}))
		})
	}, 480, 320)
	tt.Frame()
	if err := tt.Click("9B"); err != nil {
		t.Fatal(err)
	}
	if clicked != -1 {
		t.Fatalf("an Off row chosen: %d", clicked)
	}
	if err := tt.Click("2B"); err != nil {
		t.Fatal(err)
	}
	if clicked != 0 {
		t.Errorf("chosen %d", clicked)
	}
}
