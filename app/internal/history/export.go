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
		start := e.Start
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
		if start := e.Start; start.Before(zero) {
			zero = start
		}
	}
	var b strings.Builder
	for i, e := range es {
		start, end := e.Start, e.End
		if least := start.Add(minShown); end.Before(least) {
			end = least
		}
		if i+1 < len(es) {
			if next := es[i+1].Start; next.After(start) && next.Before(end) {
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

func (e Entry) translation(head Head) string {
	if head.Untranslated || e.InTargetLang {
		return ""
	}
	return e.Tgt
}

func stamp(d time.Duration) string {
	d = max(d, 0)
	ms := d.Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d,%03d", ms/3600000, ms/60000%60, ms/1000%60, ms%1000)
}
