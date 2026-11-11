package locales

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

var shipped = []string{"zh-Hans", "zh-Hant", "ja", "ko"}

// TestComplete checks that every catalog has every message of the source's: a missing one would
// say the English, which mygo i18n check only warns of.
func TestComplete(t *testing.T) {
	read := func(tag string) map[string]any {
		b, err := os.ReadFile(tag + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	en := read("en")
	for _, tag := range shipped {
		m := read(tag)
		for k := range en {
			if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "mygo.") && m[k] == nil {
				t.Errorf("%s has no %s", tag, k)
			}
		}
	}
}

// TestTranslated checks a few messages say their own words in each language, with their
// arguments.
func TestTranslated(t *testing.T) {
	en := In("en")
	for _, tag := range shipped {
		l := In(tag)
		for key, m := range map[string][2]string{
			"tagline":            {l.Tagline(), en.Tagline()},
			"statusNoPermission": {l.StatusNoPermission(), en.StatusNoPermission()},
			"accessNoteDenied":   {l.AccessNoteDenied(), en.AccessNoteDenied()},
			"hideWait":           {l.HideWait(), en.HideWait()},
			"sourceNotRunning":   {l.SourceNotRunning("Safari"), en.SourceNotRunning("Safari")},
			"noticeListening":    {l.NoticeListening("A → B", "⌥⇧T"), en.NoticeListening("A → B", "⌥⇧T")},
			"displaySrc":         {l.DisplaySrc(), en.DisplaySrc()},
		} {
			if m[0] == "" || m[0] == m[1] {
				t.Errorf("%s: %s is %q", tag, key, m[0])
			}
		}
		if !strings.Contains(l.SourceNotRunning("Safari"), "Safari") {
			t.Errorf("%s: arguments dropped", tag)
		}
	}
}

// TestLinked checks each link's words are in the note they link from, as written there: else the
// note shows with no link.
func TestLinked(t *testing.T) {
	for _, tag := range append([]string{"en"}, shipped...) {
		l := In(tag)
		for _, m := range [][2]string{
			{l.AsrCloudIntro(), l.AsrCloudConsole()},
			{l.AsrOpenAiIntro(), l.AsrOpenAiPlatform()},
			{l.MtHyMtIntro(), l.MtHyMtConsole()},
		} {
			if !strings.Contains(m[0], m[1]) {
				t.Errorf("%s: %q not in %q", tag, m[1], m[0])
			}
		}
	}
}
