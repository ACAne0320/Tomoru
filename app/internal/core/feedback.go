package core

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"tomoru/internal/prefs"
	"tomoru/internal/scene"
)

const Repo = "https://github.com/ACAne0320/Tomoru"

const maxIssueURL = 4000

func (c *Core) Versions() (app, os string) { return c.deps.Version, c.deps.OS }

func (c *Core) setup() (version, system, engines string) {
	effective := c.effective()
	p := &effective
	version = c.deps.Version
	system = c.deps.OS + " (" + runtime.GOARCH + ")"
	if mem, known := c.memory(); known {
		system += fmt.Sprintf(", %d GB memory", mem>>30)
	}
	if g, known := c.gpu(); known && g > 0 {
		system += fmt.Sprintf(", %d GB GPU memory", g>>30)
	}
	engines = fmt.Sprintf("%s → %s, %s → %s", asrName(p), mtName(p, p.MT), p.Src, p.Tgt)
	if !p.Translates() {
		engines = fmt.Sprintf("%s, %s, the original only", asrName(p), p.Src)
	}
	return
}

type FeedbackKind int

const (
	FeedbackProblem FeedbackKind = iota
	FeedbackIdea
)

func (c *Core) Feedback(kind FeedbackKind, what string) string {
	p := &c.Prefs
	version, system, engines := c.setup()
	var b strings.Builder
	line := func(f string, a ...any) { fmt.Fprintf(&b, f+"\n", a...) }
	if kind == FeedbackIdea {
		line("I use Tomoru, a desktop app that shows live translated subtitles (%s), and I have an idea for it that I want to suggest as a GitHub issue. Help me, as these steps say.", Repo)
		line("")
		line("## What I'd like")
	} else {
		line("I use Tomoru, a desktop app that shows live translated subtitles (%s), and I want to report a problem with it as a GitHub issue. Help me, as these steps say.", Repo)
		line("")
		line("## What I saw")
	}
	line("")
	if what = strings.TrimSpace(what); what == "" {
		what = "(Not written yet: ask me.)"
	}
	line("%s", what)
	line("")
	line("## My setup")
	line("")
	line("Copied from the app. API keys, passwords, the app I listen to, my subtitles and recordings are left out.")
	line("")
	line("- Tomoru %s on %s", version, system)
	line("- Engines: %s", engines)
	line("- Pictures: %s", mtName(p, p.ImageMT))
	line("- App language: %s", or(p.UILang, "the system's"))
	line("- Listening to: %s", choose(p.HearsAll(), "all the computer's sound", "one app"))
	line("- Scene: %s", choose(p.Scene == scene.GeneralID, "general", "one of my own"))
	line("- Translate while spoken: %s; history: %s", onOff(p.Draft), onOff(p.History))
	line("- Network: %s", proxyName(p))
	steps := ideaSteps
	if kind == FeedbackProblem {
		steps = problemSteps
		if c.deps.LogPath != "" {
			line("- Logs: in %s", filepath.Dir(c.deps.LogPath))
		}
	}
	line("")
	line("## Steps")
	line("")
	for _, s := range steps {
		line("%s", s)
	}
	line("%s", "Write only what I told you and what you read: don't guess a cause or say something was looked into that wasn't. Don't change Tomoru's code, its settings or any other file of mine, and don't open a pull request: this is only an issue.")
	return b.String()
}

var problemSteps = []string{
	"1. Talk with me in the language I write in, and write the issue in it too. Ask only what you still need, a question or two at a time: what I was watching and in which app or site, what I did just before, what I expected and what happened instead, whether it happens again, and about when it last happened.",
	"2. If it is a security problem (an API key leaving the system's keychain, or audio, screenshots or subtitles sent somewhere I didn't choose), stop: tell me to report it privately at " + Repo + "/security/advisories/new, not in an issue.",
	"3. Read the logs above, only reading. Lines start with the local time; look near when it happened, and keep the few lines (a few dozen at most) that bear on the problem. tomoru.log has the app and its engine, tomoru.1.log the lines before it, llama-server.log the on-device models.",
	"4. Before you show me any line, redact: API keys and tokens (sk-..., Bearer ..., key=...), passwords and user:password@ in addresses, query strings, my user name and home folder (write ~), and names of apps, sites or shows I was watching, unless I say they matter.",
	"5. Don't open other files of Tomoru: settings.json, scenes.json, stats.json and history/ hold my settings, names and subtitles. Don't record audio or the screen. If a subtitle line matters, ask me to paste the lines I choose, and show me exactly how they will appear in the issue.",
	"6. Write steps someone else can follow to see it happen, with what I expected and what happened.",
	"7. Look for the same issue, open or closed: `gh issue list --repo ACAne0320/Tomoru --state all --search \"<words>\"` if the GitHub CLI is installed and signed in, else " + Repo + "/issues. If one matches, show it to me and ask whether to add what is new there as a comment instead.",
	"8. Draft the issue, as the repository's bug report form asks: a short title starting with \"[Bug] \", and a body with these parts: Tomoru version, Platform, OS version and computer, Engines, What happened (with the steps), Logs (the redacted lines, in a code block; leave it out if none bear on it). Show me the whole title and body, change what I ask, and send it only once I say yes.",
	"9. Send it with `gh issue create --repo ACAne0320/Tomoru --title \"...\" --body-file -`, the body on its standard input, then give me its link. Without the GitHub CLI, give me the title and body to paste at " + Repo + "/issues/new?template=bug_report.yml.",
}

var ideaSteps = []string{
	"1. Talk with me in the language I write in, and write the issue in it too. Ask only what you still need, a question or two at a time: what I was watching or doing, what got in the way or what I miss, what I'd like Tomoru to do, and why it matters to me (how often it comes up, what I do instead now). Don't ask me for steps to reproduce it.",
	"2. Don't read the logs or any file of Tomoru's: an idea needs none. My setup above is there if it bears on the idea; leave it out if not.",
	"3. If it turns out something doesn't work as it should, rather than something missing, say so and tell me to report it as a problem instead (Settings > About > Feedback & ideas > Problem). Don't write an idea up as a bug, nor a bug as an idea.",
	"4. Look for the same idea, open or closed: `gh issue list --repo ACAne0320/Tomoru --state all --search \"<words>\"` if the GitHub CLI is installed and signed in, else " + Repo + "/issues. If one matches, show it to me and ask whether to add what is new there as a comment instead.",
	"5. Draft the issue, as the repository's idea form asks: a short title starting with \"[Feature] \", and a body with two parts: \"What were you watching, and what got in the way?\" and \"What would help\", in my words, the need before any solution. Show me the whole title and body, change what I ask, and send it only once I say yes.",
	"6. Send it with `gh issue create --repo ACAne0320/Tomoru --title \"...\" --body-file -`, the body on its standard input, then give me its link. Without the GitHub CLI, give me the title and body to paste at " + Repo + "/issues/new?template=feature_request.yml.",
}

func (c *Core) IssueURL(kind FeedbackKind, what string) string {
	q, field := url.Values{"template": {"feature_request.yml"}}, "problem"
	if kind == FeedbackProblem {
		version, system, engines := c.setup()
		q, field = url.Values{"template": {"bug_report.yml"}, "version": {version}, "os": {system}, "engines": {engines}}, "what"
	}
	u := Repo + "/issues/new?" + q.Encode()
	what = strings.TrimSpace(what)
	room := max(maxIssueURL-len(u)-len("&"+field+"="), 0)
	for len(url.QueryEscape(what)) > room {
		r := []rune(what)
		what = string(r[:len(r)*room/len(url.QueryEscape(what))])
	}
	if what == "" {
		return u
	}
	q.Set(field, what)
	return Repo + "/issues/new?" + q.Encode()
}

func asrName(p *prefs.Prefs) string {
	if p.ASR == "" {
		return "none chosen"
	}
	for _, s := range p.ASRAPIs {
		if s.ID == p.ASR {
			return serviceName(s)
		}
	}
	return p.ASR
}

func mtName(p *prefs.Prefs, id string) string {
	if eid, ok := prefs.EndpointOf(id); ok {
		if e, ok := p.Endpoint(eid); ok {
			return fmt.Sprintf("own API (%s format, model %s, %s)", or(e.Format, "chat"), or(e.Model, "none"), bareURL(e.URL))
		}
	}
	for _, s := range p.MTAPIs {
		if s.ID == id {
			return serviceName(s)
		}
	}
	return or(id, "none chosen")
}

func serviceName(s prefs.Service) string {
	var more []string
	if s.Model != "" {
		more = append(more, "model "+s.Model)
	}
	if s.URL != "" {
		more = append(more, bareURL(s.URL))
	}
	if len(more) == 0 {
		return s.ID
	}
	return s.ID + " (" + strings.Join(more, ", ") + ")"
}

func bareURL(s string) string {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || u.Host == "" {
		return "an address of my own"
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
}

func proxyName(p *prefs.Prefs) string {
	switch {
	case p.NoProxy:
		return "direct, no proxy"
	case p.ProxyURL == "":
		return "the system's proxy settings"
	}
	kind := "http"
	if u, err := url.Parse(p.ProxyURL); err == nil && strings.Contains(p.ProxyURL, "://") {
		kind = u.Scheme
	}
	return "a proxy of my own (" + kind + ")"
}

func or(s, otherwise string) string { return choose(s != "", s, otherwise) }

func onOff(on bool) string { return choose(on, "on", "off") }

func choose(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}
