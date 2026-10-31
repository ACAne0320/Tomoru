package engine

import (
	"errors"
	"strings"
	"testing"
)

func TestRead(t *testing.T) {
	const hello = `{"type":"hello","protocol":1,"engine":"mac","version":"0.1.0"}` + "\n"
	var got []Event
	err := read(strings.NewReader(hello+
		`{"type":"state","state":"listening"}`+"\n"+
		`{"type":"result","final":true,"start":1.2,"end":1.5,"tokens":[{"text":"今日","start":1.2,"end":1.4,"conf":0.9},{"text":"は"}]}`+"\n"+
		`{"type":"clock","t":1.6}`+"\n"+
		`{"type":"later","field":1}`+"\n"), func(ev Event) { got = append(got, ev) })
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 || got[0].State != "listening" || got[2].T != 1.6 || got[3].Type != "later" {
		t.Fatalf("%+v", got)
	}
	r := got[1]
	if !r.Final || r.Start != 1.2 || r.End != 1.5 || r.Text() != "今日は" || *r.Tokens[0].Conf != 0.9 || r.Tokens[1].Start != nil {
		t.Errorf("%+v", r)
	}

	// An error ends the reading with it.
	err = read(strings.NewReader(hello+`{"type":"error","code":"no-permission","message":"off"}`+"\n"+`{"type":"clock","t":1}`+"\n"), func(Event) { t.Error("an event after the error") })
	var e *Error
	if !errors.As(err, &e) || e.Code != "no-permission" {
		t.Errorf("error %v", err)
	}

	// Another protocol is refused.
	if err := read(strings.NewReader(`{"type":"hello","protocol":2}`+"\n"), func(Event) {}); err == nil {
		t.Error("protocol 2 was taken")
	}
}
