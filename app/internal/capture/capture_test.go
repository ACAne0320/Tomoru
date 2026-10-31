package capture

import (
	"image"
	"testing"
)

// TestArea reads where the engine says the selection is, among its other lines.
func TestArea(t *testing.T) {
	out := []byte(`{"type":"hello","protocol":1,"engine":"windows","version":"0.1.0"}` + "\n" +
		`{"type":"area","x":-1920,"y":40,"width":300,"height":120}` + "\n")
	if got, want := area(out), image.Rect(-1920, 40, -1620, 160); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := area([]byte("[engine] not json\n")); !got.Empty() {
		t.Errorf("no area: got %v", got)
	}
}
