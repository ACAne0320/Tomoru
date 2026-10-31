package sensevoice

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestMain is tomoru-asr too, when the test runs itself as it: it answers each decode with a token
// at half the stretch, and exits on an empty stretch, as a crash.
func TestMain(m *testing.M) {
	if os.Getenv("FAKE_TOMORU_ASR") == "" {
		os.Exit(m.Run())
	}
	fmt.Println(`{"type":"hello","protocol":1,"engine":"asr","version":"0.1.0"}`)
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var c struct{ Type, ID, PCM string }
		json.Unmarshal(sc.Bytes(), &c)
		b, _ := base64.StdEncoding.DecodeString(c.PCM)
		if len(b) == 0 {
			os.Exit(3)
		}
		fmt.Printf(`{"type":"decoded","id":%q,"tokens":[{"text":"%d","start":%g}]}`+"\n", c.ID, len(b)/2, float64(len(b)/2)/16000/2)
	}
}

func TestDecoder(t *testing.T) {
	t.Setenv("FAKE_TOMORU_ASR", "1")
	d, err := Start(os.Args[0], t.TempDir(), "ja")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, n := range []int{1600, 3200} {
		pieces, err := d.Decode(ctx, make([]byte, 2*n))
		if err != nil || len(pieces) != 1 || pieces[0].Text != fmt.Sprint(n) || pieces[0].At != float64(n)/16000/2 {
			t.Fatalf("decoded %d samples: %v %v", n, pieces, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := d.Decode(canceled, make([]byte, 2)); err != context.Canceled {
		t.Fatalf("canceled: %v", err)
	}
	// An empty stretch has the fake crash: the decode in flight fails, and the decoder ends.
	if _, err := d.Decode(ctx, nil); err == nil {
		t.Fatal("decoded with tomoru-asr gone")
	}
	select {
	case <-d.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("tomoru-asr did not end")
	}
	if d.Err() == nil {
		t.Fatal("a crash is no error")
	}
	if len(d.waiting) != 0 {
		t.Fatalf("%d decodes still waiting", len(d.waiting))
	}
}
