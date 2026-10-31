package shell

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"golang.org/x/sys/windows/registry"
)

// trayIcon is a template icon, black on clear, as Windows shows it, which tints none: white on a
// dark taskbar, black on a light one, as its own icons are.
func trayIcon(template []byte) ([]byte, bool) {
	ink := uint8(255)
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE); err == nil {
		if v, _, err := k.GetIntegerValue("SystemUsesLightTheme"); err == nil && v == 1 {
			ink = 0
		}
		k.Close()
	}
	img, err := png.Decode(bytes.NewReader(template))
	if err != nil {
		return template, true
	}
	b := img.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			out.SetNRGBA(x, y, color.NRGBA{ink, ink, ink, uint8(a >> 8)})
		}
	}
	var buf bytes.Buffer
	if png.Encode(&buf, out) != nil {
		return template, true
	}
	return buf.Bytes(), false
}
