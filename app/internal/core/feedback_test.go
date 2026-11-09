package core

import (
	"net/url"
	"strings"
	"testing"

	"tomoru/internal/prefs"
)

func feedbackCore(edit func(*prefs.Prefs)) *Core {
	p := prefs.Default("zh-Hans")
	p.Src, p.Tgt, p.ASR = "ja", "zh-Hans", "openai"
	p.ASRAPIs = []prefs.Service{{ID: "openai", URL: "https://user:s3cr3tpw@api.example.com/v1?key=sk-secret#x", Model: "gpt-4o-transcribe", Key: true}}
	p.Endpoints = []prefs.Endpoint{{ID: "e1", Name: "Mine", URL: "https://llm.example.com/api?token=abc", Model: "deepseek-flash", Format: "chat"}}
	p.MT, p.ImageMT = "custom:e1", "cloud"
	p.Source, p.SourceName = "com.example.player", "Example Player"
	p.ProxyURL = "socks5://me:hunter2@127.0.0.1:1080"
	if edit != nil {
		edit(&p)
	}
	return New(p, Deps{Post: func(fn func()) { fn() }, Version: "0.1.5", OS: "macOS 26.1", LogPath: "/Users/me/Library/Logs/Tomoru/tomoru.log",
		Memory: func() uint64 { return 16 << 30 }})
}

func TestFeedbackSaysTheSetupWithoutSecrets(t *testing.T) {
	c := feedbackCore(nil)
	got := c.Feedback("  Subtitles stopped after ten minutes.  ")
	for _, want := range []string{
		"Subtitles stopped after ten minutes.\n",
		"Tomoru 0.1.5 on macOS 26.1 (",
		"16 GB memory",
		"openai (model gpt-4o-transcribe, https://api.example.com/v1) → own API (chat format, model deepseek-flash, https://llm.example.com/api), ja → zh-Hans",
		"Pictures: cloud",
		"Listening to: one app",
		"a proxy of my own (socks5)",
		"Logs: in /Users/me/Library/Logs/Tomoru",
		Repo + "/security/advisories/new",
		"gh issue create --repo ACAne0320/Tomoru",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, secret := range []string{"s3cr3tpw", "sk-secret", "token=abc", "hunter2", "127.0.0.1", "com.example.player", "Example Player"} {
		if strings.Contains(got, secret) {
			t.Errorf("%q leaked into:\n%s", secret, got)
		}
	}
}

func TestFeedbackAsksWhenNothingIsWritten(t *testing.T) {
	c := feedbackCore(func(p *prefs.Prefs) { p.ASR, p.NoProxy = "", true })
	got := c.Feedback(" ")
	for _, want := range []string{"(Not written yet: ask me.)", "none chosen → own API", "direct, no proxy", "Listening to: one app"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestIssueURLStartsTheBugForm(t *testing.T) {
	c := feedbackCore(nil)
	long := strings.Repeat("字", 3000)
	addr := c.IssueURL(long)
	if len(addr) > maxIssueURL {
		t.Errorf("the address is %d bytes, more than %d", len(addr), maxIssueURL)
	}
	u, err := url.Parse(addr)
	if err != nil {
		t.Fatal(err)
	}
	if got := u.Scheme + "://" + u.Host + u.Path; got != Repo+"/issues/new" {
		t.Errorf("address %s", got)
	}
	q := u.Query()
	if q.Get("template") != "bug_report.yml" || q.Get("version") != "0.1.5" || !strings.HasPrefix(q.Get("os"), "macOS 26.1 (") {
		t.Errorf("query %v", q)
	}
	if !strings.HasSuffix(q.Get("engines"), ", ja → zh-Hans") || strings.Contains(q.Get("engines"), "token") {
		t.Errorf("engines %q", q.Get("engines"))
	}
	if w := q.Get("what"); len(w) < 1000 || !strings.HasPrefix(long, w) {
		t.Errorf("what is cut to %d bytes, or not from the start", len(w))
	}
	if q := must(url.Parse(c.IssueURL("Subtitles stopped"))).Query(); q.Get("what") != "Subtitles stopped" {
		t.Errorf("short words cut: %q", q.Get("what"))
	}
	if q := must(url.Parse(c.IssueURL(""))).Query(); q.Has("what") {
		t.Errorf("empty words still fill the form: %v", q)
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
