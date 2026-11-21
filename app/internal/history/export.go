package history

import (
	"fmt"
	"strings"
	"time"
)

const minShown = 1500 * time.Millisecond

func Text(title string, head Head, es []Entry) string {
	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n")
	for _, e := range es {
		start, _ := e.said()
		fmt.Fprintf(&b, "\n%s %s\n", start.Format("15:04:05"), e.Src)
		if t := e.translation(head); t != "" {
			b.WriteString(t)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func SRT(head Head, es []Entry) string {
	zero := head.Start
	for _, e := range es {
		if start, _ := e.said(); start.Before(zero) {
			zero = start
		}
	}
	var b strings.Builder
	for i, e := range es {
		start, end := e.said()
		if least := start.Add(minShown); end.Before(least) {
			end = least
		}
		if i+1 < len(es) {
			if next, _ := es[i+1].said(); next.After(start) && next.Before(end) {
				end = next
			}
		}
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n", i+1, stamp(start.Sub(zero)), stamp(end.Sub(zero)), e.Src)
		if t := e.translation(head); t != "" {
			b.WriteString(t)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (e Entry) said() (start, end time.Time) {
	start, end = e.Start, e.End
	if start.IsZero() {
		start = e.At
	}
	if end.Before(start) {
		end = start
	}
	return start, end
}

func (e Entry) translation(head Head) string {
	if head.Untranslated || e.Said {
		return ""
	}
	return e.Tgt
}

func stamp(d time.Duration) string {
	d = max(d, 0)
	ms := d.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
