package settings

import (
	"cmp"
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"tomoru/internal/core"
	"tomoru/internal/prefs"
	"tomoru/internal/ui/kit"
	"tomoru/internal/ui/shared"
	"tomoru/internal/usage"
	"tomoru/locales"
)

// statsEdit is what the statistics page shows: the period, the kind of engine its chart and list
// are of, the engine picked out of the chart, and the bar chosen on it; the latency looked into,
// whether by its spread or day by day, and the bin or day chosen on that chart.
type statsEdit struct {
	span    int    // 7 days, 30 days, or all
	kind    int    // recognition or translation
	engine  string // the id of the engine picked, if picked
	picked  bool
	bar     int // -1 for none
	lat     int // of statsLatencies
	latView int // the spread, or day by day
	latBar  int // -1 for none
}

// statsSpans are the days of the periods the page shows, all of them for 0.
var statsSpans = [...]int{7, 30, 0}

// monthly is the most days the chart of all of them shows a bar each of: more, it shows months.
const monthly = 45

// statsKind is a kind of engine the page shows the use of, as Screen Time shows apps': what an
// engine of it is, the value the chart stacks and the list ranks by, how it says that value, and
// what else an engine did.
type statsKind struct {
	id       func(usage.Use) string
	value    func(usage.Counts) float64
	tick     func(float64) string // a value on the chart's axis
	tip      func(float64) string // a bar's value, chosen
	describe func(usage.Use) engineUse
}

// statsKinds are recognition, by the hours it listened, and translation, by the lines.
var statsKinds = [...]statsKind{
	{
		id:       func(u usage.Use) string { return u.ASR },
		value:    func(n usage.Counts) float64 { return n.Run / 3600 },
		tick:     func(v float64) string { return locales.Hours(kit.Number(v)) },
		tip:      func(v float64) string { return listened(v * 3600) },
		describe: asrUse,
	},
	{
		id:       func(u usage.Use) string { return u.MT },
		value:    func(n usage.Counts) float64 { return float64(n.Lines) },
		tick:     func(v float64) string { return locales.HistoryLines(int(v)) },
		tip:      func(v float64) string { return locales.HistoryLines(int(v)) },
		describe: mtUse,
	},
}

// stats is the statistics page (docs/design-system.md): over a period, how much recognition or
// translation did, day by day in a chart stacked by engine and in a list of the engines; and how
// long the steps took.
func (s *Settings) stats(c *ui.Context) {
	k, e := s.core, &s.statsEdit
	p := kit.Colors(c)
	if k.Running {
		c.After(time.Second) // the numbers count on
	}
	ui.Row(c).Gap(kit.Space(3)).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Text(c, locales.TabStats()).FontSize(kit.TextDisplay).FontWeight(kit.WeightHeavy).TextColor(p.Ink).Grow(1).MaxLines(1)
		if kit.Segmented(c, &e.span, locales.StatsRange7(), locales.StatsRange30(), locales.StatsRangeAll()).Label(locales.StatsRange()).Changed() {
			e.bar, e.latBar, e.picked = -1, -1, false
		}
	})
	ledger := k.Usage()
	if len(ledger.Days) == 0 {
		kit.Empty(c, kit.ArtStats, locales.StatsEmpty(), nil)
		return
	}
	now := time.Now()
	buckets, byMonth := statsBuckets(ledger, statsSpans[e.span], now)
	var total usage.Counts
	var uses []usage.Use
	for _, b := range buckets {
		for _, d := range b.days {
			total.Add(d.Total())
			uses = append(uses, d.Uses...)
		}
	}
	before, compared := statsBefore(ledger, statsSpans[e.span], now)
	kind := statsKinds[e.kind]
	if e.kind == 0 { // pictures were translated with no recognizer: no recognizer's row
		uses = slices.DeleteFunc(uses, func(u usage.Use) bool { return u.ASR == "" && u.Run == 0 })
	}
	engines := byEngine(uses, kind.id, kind.value)
	colors := engineColors(p, engines, kind.id)
	index := map[string]int{}
	for j, u := range engines {
		index[kind.id(u)] = j
	}
	kit.Group(c, locales.StatsUsage(), func() {
		if kit.Segmented(c, &e.kind, locales.EnginesAsr(), locales.EnginesMt()).Label(locales.StatsChartShows()).Changed() {
			e.bar, e.picked = -1, false
		}
	}, func(c *ui.Context) ui.Element { return kit.Card(c).Gap(0) }, func() {
		var was *usage.Counts
		if compared {
			was = &before
		}
		statsTotal(c, e.kind, total, statsDays(ledger, statsSpans[e.span], now), was, statsSpans[e.span]).Margin(0, 0, kit.Space(4), 0)
		ens := make([]engineUse, len(engines))
		for j, u := range engines {
			ens[j] = kind.describe(u)
		}
		picked, ok := index[e.engine]
		if !e.picked || !ok {
			picked = -1
		}
		bars := make([]kit.Bar, len(buckets))
		for i, b := range buckets {
			sums := make([]float64, len(engines))
			for _, d := range b.days {
				for _, u := range d.Uses {
					sums[index[kind.id(u)]] += kind.value(u.Counts)
				}
			}
			bar := kit.Bar{Label: b.label}
			var each []string
			for j, v := range sums {
				if picked >= 0 && j != picked {
					continue
				}
				bar.Value += v
				bar.Parts = append(bar.Parts, kit.BarPart{Value: v, Color: colors[j]})
				if v > 0 {
					each = append(each, ens[j].name+"  "+kind.tip(v))
				}
			}
			bar.Tip = b.label + " · " + kind.tip(bar.Value)
			if len(each) > 1 { // what each engine did of it
				bar.Tip += "\n" + strings.Join(each, "\n")
			}
			bars[i] = bar
		}
		title := locales.StatsDaily()
		if byMonth {
			title = locales.StatsMonthly()
		}
		kit.BarChart(c, title, bars, kind.tick, &e.bar).Margin(0, 0, kit.Space(3), 0)
		top := 0.0
		if len(engines) > 0 {
			top = kind.value(engines[0].Counts)
		}
		for j, u := range engines {
			en := ens[j]
			col := ui.Column(c.Key(j)).Gap(kit.Space(1)).Padding(kit.Space(2), 0).BorderWidth(1, 0, 0).BorderColor(p.LineSoft)
			if picked >= 0 && picked != j {
				col = col.Opacity(.45)
			}
			col.Children(func() {
				share := float32(0)
				if top > 0 {
					share = float32(kind.value(u.Counts) / top)
				}
				if kit.ShareRow(c, kit.Share{Color: colors[j], Name: en.name, Sub: en.sub, Value: en.figure, Unit: en.unit, Share: share, Muted: !en.known},
					picked == j).Clicked() {
					e.engine, e.picked, e.bar = kind.id(u), picked != j, -1 // again, all of them
				}
				if len(en.facts) > 0 {
					kit.Facts(c, en.facts).Padding(0, 0, kit.Space(1), kit.Space(6))
				}
			})
		}
	})
	var was *usage.Latency
	if compared {
		was = &before.Latency
	}
	statsLatency(c, e, total.Latency, was, statsSpans[e.span], buckets, byMonth)
}

// statsDay opens the statistics on the lines of day: of the shortest period with it, its bar chosen.
func (s *Settings) statsDay(day, now time.Time) {
	e := &s.statsEdit
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, now.Location()) // its start, as today's
	ago := int(math.Round(today.Sub(day).Hours() / 24))
	e.span = 2
	for i, span := range statsSpans {
		if span > ago {
			e.span = i
			break
		}
	}
	e.kind, e.picked, e.bar = 1, false, -1
	if l := s.core.Usage(); len(l.Days) > 0 {
		buckets, _ := statsBuckets(l, statsSpans[e.span], now)
		for i, b := range buckets {
			if !b.at.After(day) {
				e.bar = i
			}
		}
	}
	s.Tab = "stats"
}

// statsTotal is what the kind of engine shown did in all, big, with a day's of it and more beside
// it, and how much more or less it is than in the span days before, was, if any.
func statsTotal(c *ui.Context, kind int, total usage.Counts, days int, was *usage.Counts, span int) ui.Element {
	p := kit.Colors(c)
	label, value, unit := locales.StatRun(), hours(total.Run), locales.UnitHours()
	now, then := total.Run, 0.0
	if was != nil {
		then = was.Run
	}
	note := locales.StatsPerDay(locales.Hours(hours(total.Run/float64(days)))) + " · " +
		locales.StatSpeech() + " " + locales.Minutes(kit.Count(int(total.Speech/60)))
	if kind == 1 {
		label, value, unit = locales.StatLines(), kit.Count(total.Lines), locales.UnitLines()
		now, then = float64(total.Lines), 0
		if was != nil {
			then = float64(was.Lines)
		}
		note = locales.StatsPerDay(locales.HistoryLines(int(math.Round(float64(total.Lines)/float64(days))))) + " · " +
			locales.StatChars() + " " + locales.UseChars(kit.Count(total.Chars))
		if total.Images > 0 {
			note += " · " + locales.ImageTitle() + " " + locales.UseImages(total.Images)
		}
	}
	return ui.Column(c).MinWidth(0).Gap(kit.Space(1)).Children(func() {
		kit.Caption(c, label).SingleLine()
		ui.Row(c).Gap(kit.Space(1)).AlignItems(ui.End).Children(func() {
			ui.Text(c, value).FontSize(kit.TextStat).FontWeight(kit.WeightHeavy).FontFeatures(kit.Tabular).TextColor(p.Ink).SingleLine()
			ui.Text(c, unit).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(p.InkMuted).SingleLine()
		})
		kit.Caption(c, note).FontFeatures(kit.Tabular).MaxLines(2)
		if then > 0 {
			kit.Caption(c, locales.StatsVsBefore(span, change(now, then))).FontFeatures(kit.Tabular).SingleLine()
		}
	})
}

// change is how much now is more or less than then, in percent: +18%, −12%.
func change(now, then float64) string {
	pct := int(math.Round(100 * (now - then) / then))
	if pct < 0 {
		return fmt.Sprintf("−%d%%", -pct)
	}
	return fmt.Sprintf("+%d%%", pct)
}

// statsBefore are the numbers of the span days before the last span days: for comparing, if
// anything was done then; nothing for all of them, span 0.
func statsBefore(l usage.Ledger, span int, now time.Time) (sum usage.Counts, ok bool) {
	if span == 0 {
		return sum, false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from, to := today.AddDate(0, 0, -2*span+1), today.AddDate(0, 0, -span)
	for _, d := range l.Days {
		t, err := time.ParseInLocation(time.DateOnly, d.Day, now.Location())
		if err != nil || t.Before(from) || t.After(to) {
			continue
		}
		sum.Add(d.Total())
		ok = true
	}
	return sum, ok
}

// hours is seconds as hours, to a tenth.
func hours(s float64) string { return strconv.FormatFloat(s/3600, 'f', 1, 64) }

// listening is seconds of listening as the page tells them, a value and its unit: hours to a tenth
// from one, else whole minutes, else seconds.
func listening(s float64) (value, unit string) {
	switch {
	case s >= 3600:
		return hours(s), locales.UnitHours()
	case s >= 60:
		return strconv.Itoa(int(s / 60)), locales.UnitMinutes()
	}
	return strconv.Itoa(int(s)), locales.UnitSeconds()
}

// listened is seconds of listening as listening tells them, in a phrase: 1.5 h, 12 min.
func listened(s float64) string {
	v, _ := listening(s)
	switch {
	case s >= 3600:
		return locales.Hours(v)
	case s >= 60:
		return locales.Minutes(v)
	}
	return locales.Seconds(v)
}

// statsDays are the days of the period: the last span days, or since the first day of the ledger
// for 0.
func statsDays(l usage.Ledger, span int, now time.Time) int {
	if span > 0 {
		return span
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	first, err := time.ParseInLocation(time.DateOnly, l.Days[0].Day, now.Location())
	if err != nil || !first.Before(today) {
		return 1
	}
	return int(math.Round(today.Sub(first).Hours()/24)) + 1
}

// engineColors are the colors of the engines in the chart and the list, each its own in any period.
func engineColors(p *kit.Palette, engines []usage.Use, id func(usage.Use) string) []ui.Color {
	colors := make([]ui.Color, len(engines))
	for i, u := range engines {
		colors[i] = engineColor(p, id(u))
	}
	return colors
}

// engineColor is the color of the engine id: its maker's, the lantern's family for one's own APIs
// by their ids, and the lines' for what no engine was noted of.
func engineColor(p *kit.Palette, id string) ui.Color {
	switch id {
	case "":
		return p.LineStrong
	case "apple":
		return p.Apple
	case core.QwenCloud, "qwen3-asr-1.7b":
		return p.Qwen
	case "sensevoice-small":
		return p.Alibaba
	case "cloud":
		return p.Bilibili[0]
	case "local-9b":
		return p.Bilibili[1]
	case "local-2b":
		return p.Bilibili[2]
	}
	h := fnv.New32a()
	h.Write([]byte(id))
	return p.Series[h.Sum32()%uint32(len(p.Series))]
}

// bucket is a bar of the chart: a day, or a month, and the days of it translating did something.
type bucket struct {
	label string
	at    time.Time
	days  []usage.Day
}

// statsBuckets are the days of the last span days to now, or all of them since the first for 0,
// as the chart's bars: by month when all of them are more than monthly.
func statsBuckets(l usage.Ledger, span int, now time.Time) (bs []bucket, byMonth bool) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from := today.AddDate(0, 0, 1-span)
	if span == 0 {
		from = today
		if first, err := time.ParseInLocation(time.DateOnly, l.Days[0].Day, now.Location()); err == nil && first.Before(today) {
			from = first
		}
		byMonth = today.Sub(from) > monthly*24*time.Hour
	}
	if byMonth {
		from = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, now.Location())
		for t := from; !t.After(today); t = t.AddDate(0, 1, 0) {
			bs = append(bs, bucket{label: t.Format("2006/1"), at: t})
		}
	} else {
		for t := from; !t.After(today); t = t.AddDate(0, 0, 1) {
			bs = append(bs, bucket{label: t.Format("1/2"), at: t})
		}
	}
	for _, d := range l.Days {
		t, err := time.ParseInLocation(time.DateOnly, d.Day, now.Location())
		if err != nil || t.Before(from) || t.After(today) {
			continue
		}
		i := len(bs) - 1
		for i > 0 && bs[i].at.After(t) {
			i--
		}
		bs[i].days = append(bs[i].days, d)
	}
	return bs, byMonth
}

// round is v to two digits, as a chart's ticks are: 0.25, 1.5, 120.
func round(v float64) float64 {
	if v == 0 {
		return 0
	}
	p := math.Pow(10, 1-math.Floor(math.Log10(math.Abs(v))))
	return math.Round(v*p) / p
}

// latency is a latency of what is said to its translation, chosen among them, as they come in
// recognition and in translation: how its times spread, or its typical time and 90% of them day by
// day.
type latency struct {
	name, what func() string
	hist       func(usage.Latency) usage.Hist
}

// statsLatencies are the latencies the page shows, what the user waits first; then recognition's
// two and translation's two.
var statsLatencies = [...]latency{
	{locales.LatEndToEnd, locales.LatEndToEndDescription, func(l usage.Latency) usage.Hist { return l.EndToEnd }},
	{locales.LatFirstText, locales.LatFirstTextDescription, func(l usage.Latency) usage.Hist { return l.FirstText }},
	{locales.LatFinal, locales.LatFinalDescription, func(l usage.Latency) usage.Hist { return l.Final }},
	{locales.LatFirstTranslation, locales.LatFirstTranslationDescription, func(l usage.Latency) usage.Hist { return l.FirstTranslation }},
	{locales.LatTranslation, locales.LatTranslationDescription, func(l usage.Latency) usage.Hist { return l.Translation }},
}

// statsLatency is the card of the latencies: a tile of each with its typical time, to choose one
// by; what it measures, and how much faster or slower it is than in the span days before, was, if
// any; and its chart.
func statsLatency(c *ui.Context, e *statsEdit, total usage.Latency, was *usage.Latency, span int, buckets []bucket, byMonth bool) {
	var tiles []kit.Tile
	timed := false
	for i, l := range statsLatencies {
		t := kit.Tile{Label: l.name(), Value: "—", Strong: i == 0}
		if mid, ok := l.hist(total).Quantile(.5); ok {
			t.Value, t.Unit, timed = kit.SecondsValue(mid), locales.UnitSeconds(), true
		}
		tiles = append(tiles, t)
	}
	if !timed {
		return
	}
	l := statsLatencies[e.lat]
	h := l.hist(total)
	daily := locales.StatsDaily()
	if byMonth {
		daily = locales.StatsMonthly()
	}
	kit.Group(c, locales.StatsLatency(), func() {
		if kit.Segmented(c, &e.latView, locales.StatsSpread(), daily).Label(locales.StatsChartShows()).Changed() {
			e.latBar = -1
		}
	}, kit.Card, func() {
		if kit.PickTiles(c, &e.lat, []kit.TileGroup{
			{Label: locales.LatYouWait(), Tiles: tiles[:1]},
			{Label: locales.EnginesAsr(), Tiles: tiles[1:3]},
			{Label: locales.EnginesMt(), Tiles: tiles[3:]},
		}).Label(locales.StatsLatency()).Changed() {
			e.latBar = -1
		}
		mid, ok := h.Quantile(.5)
		ui.Row(c).Gap(kit.Space(4)).AlignItems(ui.Start).Children(func() {
			kit.Caption(c, l.what()).FontSize(kit.TextBody).Grow(1).MinWidth(0).MaxLines(2)
			if was == nil || !ok {
				return
			}
			if then, ok := l.hist(*was).Quantile(.5); ok {
				faster(c, span, mid, then)
			}
		})
		if !ok {
			kit.SandCard(c).Children(func() { kit.Caption(c, locales.LatNone()) })
			return
		}
		p90, _ := h.Quantile(.9)
		if e.latView == 1 {
			kit.WhiskerChart(c, l.name(), latencyDays(buckets, l), func(v float64) string { return locales.Seconds(fmt.Sprintf("%g", round(v))) }, &e.latBar)
			return
		}
		kit.Histogram(c, l.name(), latencyBins(h, p90), []kit.HistMark{
			{At: mid.Seconds(), Label: locales.LatHalf(kit.Seconds(mid)), Strong: true},
			{At: p90.Seconds(), Label: locales.LatP90(kit.Seconds(p90))},
		}, func(v float64) string { return locales.Seconds(strconv.FormatFloat(v, 'f', -1, 64)) }, &e.latBar)
	})
}

// latencyBins are the buckets of h as a histogram's bins, in seconds, to a round time well past
// p90: those past it left out.
func latencyBins(h usage.Hist, p90 time.Duration) []kit.Bin {
	end := 10.0
	for _, s := range []float64{.5, 1, 2, 3, 4, 5, 7, 10} {
		if s >= p90.Seconds()*1.5 {
			end = s
			break
		}
	}
	n := 0
	for _, k := range h.B {
		n += k
	}
	var bins []kit.Bin
	lo := 0.0
	for i, bound := range usage.Bounds {
		hi := bound / 1000
		if lo >= end {
			break
		}
		share := float64(h.B[i]) / float64(max(n, 1))
		bins = append(bins, kit.Bin{Lo: lo, Hi: hi, Share: share,
			Tip: locales.Seconds(kit.SecondsValue(seconds(lo))+"–"+kit.SecondsValue(seconds(hi))) + fmt.Sprintf(" · %.0f%%", 100*share)})
		lo = hi
	}
	return bins
}

// latencyDays are the typical time of the latency l and the time 90% came within, of each of the
// chart's days (or months), in seconds: NaN for those it was not timed in.
func latencyDays(buckets []bucket, l latency) []kit.Whisker {
	ws := make([]kit.Whisker, len(buckets))
	for i, b := range buckets {
		var sum usage.Counts
		for _, d := range b.days {
			sum.Add(d.Total())
		}
		h := l.hist(sum.Latency)
		ws[i] = kit.Whisker{Label: b.label, Mid: math.NaN(), Hi: math.NaN(), Tip: b.label + " · —"}
		if mid, ok := h.Quantile(.5); ok {
			p90, _ := h.Quantile(.9)
			ws[i].Mid, ws[i].Hi = mid.Seconds(), p90.Seconds()
			ws[i].Tip = b.label + " · " + kit.Seconds(mid) + " · " + locales.LatP90(kit.Seconds(p90))
		}
	}
	return ws
}

// byEngine sums uses by the engine id gives, the most of rank first and those of no engine last;
// each sum is named by the latest of its uses.
func byEngine(uses []usage.Use, id func(usage.Use) string, rank func(usage.Counts) float64) []usage.Use {
	var out []usage.Use
	at := map[string]int{}
	for _, u := range uses {
		i, ok := at[id(u)]
		if !ok {
			at[id(u)] = len(out)
			out = append(out, u)
			continue
		}
		sum := out[i].Counts
		sum.Add(u.Counts)
		out[i] = usage.Use{Engines: u.Engines, Counts: sum}
	}
	unknown := func(u usage.Use) int {
		if id(u) == "" {
			return 1
		}
		return 0
	}
	slices.SortStableFunc(out, func(a, b usage.Use) int {
		return cmp.Or(cmp.Compare(unknown(a), unknown(b)), cmp.Compare(rank(b.Counts), rank(a.Counts)))
	})
	return out
}

// engineUse is what the list shows of an engine: its name and its model, the value it is ranked by,
// and its other figures, what it did, then how long it took.
type engineUse struct {
	name, sub    string
	figure, unit string
	known        bool
	facts        []kit.Fact
}

// asrUse is what a recognizer did: how long it listened, the speech it heard and the audio a cloud
// one was sent, then how soon it recognized.
func asrUse(u usage.Use) engineUse {
	e := engineUse{known: u.ASR != ""}
	e.name, e.sub = asrName(u)
	e.figure, e.unit = listening(u.Run)
	if n := int(u.Speech / 60); n > 0 {
		e.facts = append(e.facts, kit.Fact{Label: locales.StatSpeech(), Value: locales.Minutes(kit.Count(n))})
	}
	if u.Audio > 0 {
		e.facts = append(e.facts, kit.Fact{Label: locales.FactAudio(), Value: locales.Minutes(kit.Count(int(math.Ceil(u.Audio / 60))))})
	}
	e.facts = append(e.facts, latencies(
		[]string{locales.LatFirstText(), locales.LatFinal()}, u.Latency.FirstText, u.Latency.Final)...)
	return e
}

// mtUse is what a translator did: the lines, their letters, the tokens they took, the lines it
// failed and the pictures it translated, then how soon it translated.
func mtUse(u usage.Use) engineUse {
	e := engineUse{figure: kit.Count(u.Lines), unit: locales.UnitLines(), known: u.MT != ""}
	e.name, e.sub = mtName(u)
	if u.Chars > 0 {
		e.facts = append(e.facts, kit.Fact{Label: locales.StatChars(), Value: locales.UseChars(kit.Count(u.Chars))})
	}
	if u.InTokens > 0 || u.OutTokens > 0 {
		e.facts = append(e.facts, kit.Fact{Label: locales.FactInTokens(), Value: kit.Count(u.InTokens)},
			kit.Fact{Label: locales.FactOutTokens(), Value: kit.Count(u.OutTokens)})
	}
	if u.Failed > 0 {
		e.facts = append(e.facts, kit.Fact{Label: locales.FactFailed(), Value: locales.HistoryLines(u.Failed)})
	}
	if u.Images > 0 {
		e.facts = append(e.facts, kit.Fact{Label: locales.ImageTitle(), Value: locales.UseImages(u.Images)})
	}
	e.facts = append(e.facts, latencies(
		[]string{locales.LatFirstTranslation(), locales.LatTranslation(), locales.LatEndToEnd()},
		u.Latency.FirstTranslation, u.Latency.Translation, u.Latency.EndToEnd)...)
	return e
}

// asrName names a recognizer, and the cloud's model.
func asrName(u usage.Use) (name, model string) {
	if u.ASR == "" {
		return locales.EngineUnknown(), ""
	}
	return shared.EngineName(core.Engine{ID: u.ASR}), u.ASRModel
}

// mtName names a translator, and an endpoint's model: an endpoint by its name, as they were.
func mtName(u usage.Use) (name, model string) {
	switch {
	case u.MT == "":
		return locales.EngineUnknown(), ""
	case strings.HasPrefix(u.MT, prefs.CustomPrefix):
		return cmp.Or(u.MTName, strings.TrimPrefix(u.MT, prefs.CustomPrefix)), u.MTModel
	}
	return shared.EngineName(core.Engine{ID: u.MT}), ""
}

// latencies are the typical times of hs, named by names, those timed.
func latencies(names []string, hs ...usage.Hist) []kit.Fact {
	var facts []kit.Fact
	for i, h := range hs {
		if d, ok := h.Quantile(.5); ok {
			facts = append(facts, kit.Fact{Label: names[i], Value: kit.Seconds(d)})
		}
	}
	return facts
}

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// faster says how much faster or slower the typical time now is than then, of the span days before:
// in green faster, in red slower, the same within a twentieth of a second or a twentieth of it.
func faster(c *ui.Context, span int, now, then time.Duration) {
	p := kit.Colors(c)
	d := now - then
	text, ink := locales.LatSame(span), p.InkMuted
	switch {
	case d.Abs() < max(50*time.Millisecond, then/20):
	case d < 0:
		text, ink = locales.LatFaster(span, kit.Seconds(-d)), p.OkText
	default:
		text, ink = locales.LatSlower(span, kit.Seconds(d)), p.Danger
	}
	ui.Text(c, text).FontSize(kit.TextControl).FontWeight(kit.WeightBold).FontFeatures(kit.Tabular).TextColor(ink).Shrink(0).SingleLine()
}
