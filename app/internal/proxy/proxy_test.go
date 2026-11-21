package proxy

import (
	"net/http"
	"testing"
)

func TestParse(t *testing.T) {
	for in, want := range map[string]string{
		"":                          "",
		" 127.0.0.1:7890 ":          "http://127.0.0.1:7890",
		"http://proxy.lan:8080/":    "http://proxy.lan:8080",
		"socks5://127.0.0.1:1080":   "socks5://127.0.0.1:1080",
		"https://user:pw@p.io:443":  "https://user:pw@p.io:443",
		"127.0.0.1":                 "error",
		"ftp://127.0.0.1:21":        "error",
		"http://127.0.0.1:7890/pac": "error",
	} {
		u, err := Parse(in)
		got := ""
		switch {
		case err != nil:
			got = "error"
		case u != nil:
			got = u.String()
		}
		if got != want {
			t.Errorf("Parse(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestFor goes directly when off and to loopback, through the user's proxy when given, and leaves
// the rest to the system's.
func TestFor(t *testing.T) {
	defer current.Store(nil)
	req := func(u string) *http.Request {
		r, _ := http.NewRequest(http.MethodGet, u, nil)
		return r
	}
	Set(false, "127.0.0.1:7897")
	if u, _ := For(req("https://api.openai.com/v1/models")); u == nil || u.Host != "127.0.0.1:7897" {
		t.Errorf("the user's: %v", u)
	}
	for _, local := range []string{"http://127.0.0.1:52011/v1/chat/completions", "http://localhost:8080", "http://[::1]:9000"} {
		if u, _ := For(req(local)); u != nil {
			t.Errorf("%s through %v", local, u)
		}
	}
	Set(true, "127.0.0.1:7897")
	if u, _ := For(req("https://api.openai.com")); u != nil {
		t.Errorf("off: %v", u)
	}
	// Not an address: an error, and the cloud is not reached.
	if err := Set(false, "127.0"); err == nil {
		t.Fatal("Set: want an error")
	}
	if u, err := For(req("https://api.openai.com")); err == nil || u != nil {
		t.Errorf("invalid: %v, %v", u, err)
	}
}

// TestSetInvalid reports an address that is not one and does not reach the cloud through the
// system's proxy instead. Loopback stays direct, an empty address is still the system's, and
// turning the proxy off reaches the cloud directly.
func TestSetInvalid(t *testing.T) {
	defer current.Store(nil)
	t.Setenv("HTTPS_PROXY", "http://10.1.2.3:8888")
	req := func(raw string) *http.Request {
		r, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	cloud := req("https://api.openai.com/v1/models")
	if err := Set(false, "127.0"); err == nil {
		t.Fatal("Set: want an error")
	}
	if u, err := For(cloud); err == nil || u != nil {
		t.Fatalf("cloud: %v, %v", u, err)
	}
	if u, err := For(req("http://127.0.0.1:8080/v1/chat/completions")); err != nil || u != nil {
		t.Fatalf("loopback: %v, %v", u, err)
	}
	if err := Set(false, ""); err != nil {
		t.Fatal(err)
	}
	if u, err := For(cloud); err != nil || u == nil || u.Host != "10.1.2.3:8888" {
		t.Fatalf("empty: %v, %v", u, err)
	}
	if err := Set(false, "127.0.0.1:7890"); err != nil {
		t.Fatal(err)
	}
	if u, err := For(cloud); err != nil || u == nil || u.Host != "127.0.0.1:7890" {
		t.Fatalf("address: %v, %v", u, err)
	}
	if err := Set(true, "127.0"); err != nil {
		t.Fatal(err)
	}
	if u, err := For(cloud); err != nil || u != nil {
		t.Fatalf("off: %v, %v", u, err)
	}
}
