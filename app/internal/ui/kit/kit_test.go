package kit

import (
	"testing"
	"time"
)

func TestNumbers(t *testing.T) {
	for n, want := range map[int]string{0: "0", 999: "999", 1000: "1,000", 1234567: "1,234,567", -12345: "-12,345"} {
		if got := Count(n); got != want {
			t.Errorf("Count(%d) = %q, want %q", n, got, want)
		}
	}
	for d, want := range map[time.Duration]string{620 * time.Millisecond: "0.62", 1340 * time.Millisecond: "1.3", 12 * time.Second: "12.0"} {
		if got := SecondsValue(d); got != want {
			t.Errorf("SecondsValue(%v) = %q, want %q", d, got, want)
		}
	}
}
