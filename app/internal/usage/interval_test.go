package usage

import (
	"math"
	"testing"
	"time"
	_ "time/tzdata" // the zones of the tests, where the system has no database, as Windows
)

func TestCountsSinceAllFields(t *testing.T) {
	previous := Counts{Run: 10.25, Lines: 11, Speech: 12.5, Chars: 13, Audio: 14.75, Failed: 15, InTokens: 16, OutTokens: 17, Images: 18}
	increment := Counts{Run: 1.25, Lines: 2, Speech: 3.5, Chars: 4, Audio: 5.75, Failed: 6, InTokens: 7, OutTokens: 8, Images: 9}
	oldHist := []*Hist{&previous.Latency.FirstText, &previous.Latency.Final, &previous.Latency.FirstTranslation, &previous.Latency.Translation, &previous.Latency.Queue, &previous.Latency.EndToEnd}
	newHist := []*Hist{&increment.Latency.FirstText, &increment.Latency.Final, &increment.Latency.FirstTranslation, &increment.Latency.Translation, &increment.Latency.Queue, &increment.Latency.EndToEnd}
	for i, h := range oldHist {
		h.Sum = float64(1000+i) + 0.5
		newHist[i].Sum = float64(100+i) + 0.25
		for bucket := range h.B {
			h.B[bucket] = bucket + i + 1
			newHist[i].B[bucket] = bucket + i + 2
			h.N += h.B[bucket]
			newHist[i].N += newHist[i].B[bucket]
		}
	}
	current := previous
	current.Add(increment)
	before := current
	if got := current.Since(previous); got != increment {
		t.Fatalf("cumulative delta omitted fields: got %+v; want %+v", got, increment)
	}
	if current != before {
		t.Error("Since mutated its cumulative source")
	}
	if got := current.Since(current); got != (Counts{}) {
		t.Errorf("same snapshot produced delta: %+v", got)
	}
	reverse := previous.Since(current)
	if reverse.Run != -increment.Run || reverse.Speech != -increment.Speech || reverse.Audio != -increment.Audio || reverse.Failed != -increment.Failed || reverse.Latency.Translation.N != -increment.Latency.Translation.N || reverse.Latency.EndToEnd.Sum != -increment.Latency.EndToEnd.Sum {
		t.Errorf("Since clamped a raw difference: %+v", reverse)
	}
	for i, n := range reverse.Latency.FirstText.B {
		if n != -increment.Latency.FirstText.B[i] {
			t.Fatalf("histogram bucket %d was clamped", i)
		}
	}
	reconstructed := previous
	reconstructed.Add(current.Since(previous))
	if reconstructed != current {
		t.Fatal("adding the delta did not reconstruct the cumulative counts")
	}
}

type intervalCase struct {
	name, zone, start, end string
	want                   map[string]float64
}

var intervalCases = []intervalCase{
	{name: "same day fractions", zone: "Asia/Shanghai", start: "2026-10-21 10:15:00.25", end: "2026-10-21 10:15:02.75", want: map[string]float64{"2026-10-21": 2.5}},
	{name: "fractional midnight", zone: "Asia/Shanghai", start: "2026-10-21 23:59:58.25", end: "2026-10-22 00:00:03.75", want: map[string]float64{"2026-10-21": 1.75, "2026-10-22": 3.75}},
	{name: "multiple days", zone: "Asia/Shanghai", start: "2026-10-20 23:00:00", end: "2026-10-23 01:00:00", want: map[string]float64{"2026-10-20": 3600, "2026-10-21": 86400, "2026-10-22": 86400, "2026-10-23": 3600}},
	{name: "ends at midnight", zone: "UTC", start: "2026-10-21 23:00:00", end: "2026-10-22 00:00:00", want: map[string]float64{"2026-10-21": 3600}},
	{name: "full calendar day", zone: "UTC", start: "2026-10-21 00:00:00", end: "2026-10-22 00:00:00", want: map[string]float64{"2026-10-21": 86400}},
	{name: "spring DST", zone: "America/New_York", start: "2026-03-07 23:00:00", end: "2026-03-09 01:00:00", want: map[string]float64{"2026-03-07": 3600, "2026-03-08": 23 * 3600, "2026-03-09": 3600}},
	{name: "fall DST", zone: "America/New_York", start: "2026-10-31 23:00:00", end: "2026-11-02 01:00:00", want: map[string]float64{"2026-10-31": 3600, "2026-11-01": 25 * 3600, "2026-11-02": 3600}},
	{name: "DST skips midnight", zone: "America/Sao_Paulo", start: "2018-11-03 23:30:00", end: "2018-11-04 01:30:00", want: map[string]float64{"2018-11-03": 1800, "2018-11-04": 1800}},
	{name: "Santiago skips midnight", zone: "America/Santiago", start: "2026-09-05 23:30:00", end: "2026-09-06 01:30:00", want: map[string]float64{"2026-09-05": 1800, "2026-09-06": 1800}},
	{name: "Havana repeats midnight", zone: "America/Havana", start: "2026-10-31 23:00:00", end: "2026-11-02 01:00:00", want: map[string]float64{"2026-10-31": 3600, "2026-11-01": 25 * 3600, "2026-11-02": 3600}},
	{name: "skipped calendar date", zone: "Pacific/Apia", start: "2011-12-29 23:00:00", end: "2011-12-31 01:00:00", want: map[string]float64{"2011-12-29": 3600, "2011-12-31": 3600}},
	{name: "zero duration", zone: "Asia/Shanghai", start: "2026-10-21 00:00:00", end: "2026-10-21 00:00:00", want: map[string]float64{}},
	{name: "reverse time", zone: "Asia/Shanghai", start: "2026-10-22 01:00:00", end: "2026-10-21 23:00:00", want: map[string]float64{}},
}

func intervalTimes(t *testing.T, tc intervalCase) (time.Time, time.Time) {
	t.Helper()
	location, err := time.LoadLocation(tc.zone)
	if err != nil {
		t.Fatal(err)
	}
	parse := func(value string) time.Time {
		parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, location)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	return parse(tc.start), parse(tc.end)
}

func verifyInterval(t *testing.T, l Ledger, engines Engines, want map[string]float64, field string, total float64) {
	t.Helper()
	if len(l.Days) != len(want) {
		t.Fatalf("unexpected dates: %+v; want %+v", l.Days, want)
	}
	var sum float64
	for i, day := range l.Days {
		if i > 0 && l.Days[i-1].Day >= day.Day {
			t.Fatal("calendar days are not in order")
		}
		if len(day.Uses) != 1 || day.Uses[0].Engines != engines {
			t.Fatalf("engine identity lost: %+v", day)
		}
		count := day.Uses[0].Counts
		value := count.Run
		if field == "speech" {
			value = count.Speech
			count.Speech = 0
		} else {
			count.Run = 0
		}
		if count != (Counts{}) {
			t.Errorf("interval changed unrelated counts: %+v", count)
		}
		expected, ok := want[day.Day]
		if !ok || math.Abs(value-expected) > 1e-8 {
			t.Errorf("%s %s = %.12f; want %.12f", day.Day, field, value, expected)
		}
		sum += value
	}
	if math.Abs(sum-total) > 1e-8 {
		t.Errorf("duration not conserved: %.12f; want %.12f", sum, total)
	}
}

func TestLedgerAddRunCalendarDays(t *testing.T) {
	engines := Engines{ASR: "cloud", ASRModel: "asr-model", MT: "custom:endpoint", MTName: "endpoint", MTModel: "mt-model"}
	for _, tc := range intervalCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := intervalTimes(t, tc)
			var ledger Ledger
			// End's display location must not override the running interval's start location.
			ledger.AddRun(start, end.In(time.UTC), engines)
			total := 0.0
			if end.After(start) {
				total = end.Sub(start).Seconds()
			}
			verifyInterval(t, ledger, engines, tc.want, "run", total)
		})
	}
}

func TestLedgerAddSpeechCalendarDays(t *testing.T) {
	engines := Engines{ASR: "apple", MT: "cloud"}
	for _, tc := range intervalCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := intervalTimes(t, tc)
			seconds := end.Sub(start).Seconds()
			var ledger Ledger
			ledger.AddSpeech(end, seconds, engines)
			total := 0.0
			if seconds > 0 {
				total = seconds
			}
			verifyInterval(t, ledger, engines, tc.want, "speech", total)
		})
	}
}

func TestLedgerIntervalsAccumulateWithoutChangingOtherCounts(t *testing.T) {
	engines := Engines{ASR: "apple", MT: "cloud"}
	start := time.Date(2026, 10, 21, 23, 59, 59, 0, time.UTC)
	end := start.Add(2 * time.Second)
	var ledger Ledger
	ledger.Add("2026-10-21", engines, Counts{Lines: 2, Chars: 4})
	ledger.AddRun(start, end, engines)
	ledger.AddRun(start, end, engines)
	ledger.AddSpeech(end, 2, engines)
	if got := ledger.Day("2026-10-21"); got != (Counts{Run: 2, Speech: 1, Lines: 2, Chars: 4}) {
		t.Errorf("first day's accumulated counts: %+v", got)
	}
	if got := ledger.Day("2026-10-22"); got != (Counts{Run: 2, Speech: 1}) {
		t.Errorf("next day's accumulated counts: %+v", got)
	}
}

func TestLedgerSpeechPreservesFractionalSeconds(t *testing.T) {
	end := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	for _, seconds := range []float64{1.1234567895, 0.0000000005} {
		var ledger Ledger
		ledger.AddSpeech(end, seconds, Engines{})
		if len(ledger.Days) != 1 || ledger.Days[0].Day != "2026-10-21" || ledger.Days[0].Total().Speech != seconds {
			t.Errorf("fractional audio seconds lost or moved past exclusive midnight: %+v; want %.12f", ledger, seconds)
		}
	}
}
