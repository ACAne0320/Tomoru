package samples

import "testing"

func TestAudio(t *testing.T) {
	for _, code := range []string{"ja", "zh", "zh-Hant", "en", "ko"} {
		if !Has(code) {
			t.Errorf("Has(%q) = false", code)
		}
		b := Audio(code)
		if len(b) < 44 || string(b[:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
			t.Errorf("Audio(%q): invalid WAV header (%d bytes)", code, len(b))
		}
	}
	if Has("fr") || Audio("fr") != nil {
		t.Error("unexpected audio for fr")
	}
}
