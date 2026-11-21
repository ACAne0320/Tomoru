package proxy

import (
	"errors"
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
	Set(false, "127.0")
	if u, err := For(req("https://api.openai.com")); u != nil || !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid: %v, %v", u, err)
	}
	if u, err := For(req("http://127.0.0.1:52011")); u != nil || err != nil {
		t.Errorf("invalid, loopback: %v, %v", u, err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: For}}
	if _, err := client.Get("https://api.openai.com"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a request with an invalid address: %v", err)
	}
	Set(true, "127.0")
	if u, err := For(req("https://api.openai.com")); u != nil || err != nil {
		t.Errorf("invalid, off: %v, %v", u, err)
	}
}
